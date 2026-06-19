package listings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"
	"github.com/madura7/ai-elamachan/backend/internal/auth"
	"github.com/madura7/ai-elamachan/backend/internal/blob"

	// pgx registers the "pgx" driver name with database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Handler serves listing and category endpoints.
type Handler struct {
	db              *sql.DB
	policy          PostingPolicy
	bearer          func(http.Handler) http.Handler
	verifyToken     func(ctx context.Context, token string) (string, error)
	blob            blob.BlobStore
	onImageChange   func(listingID string, hasImage bool) // best-effort search update
}

// NewHandlerFromEnv constructs a Handler from DATABASE_URL.
// Returns an error when the env var is absent or the connection ping fails.
func NewHandlerFromEnv() (*Handler, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("listings: DATABASE_URL not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("listings: open db: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)
	return &Handler{
		db:     db,
		policy: policyFromEnv(db),
	}, nil
}

// SetDeps wires optional image-upload (b) and a search-update callback (onImageChange).
// Both may be nil — handlers degrade gracefully when absent.
func (h *Handler) SetDeps(b blob.BlobStore, onImageChange func(listingID string, hasImage bool)) {
	h.blob = b
	h.onImageChange = onImageChange
}

// SetAuth wires in bearer middleware (for POST /listings) and a token verifier
// (for optional auth on GET /listings?mine=true).
func (h *Handler) SetAuth(
	bearer func(http.Handler) http.Handler,
	verify func(ctx context.Context, token string) (string, error),
) {
	h.bearer = bearer
	h.verifyToken = verify
}

// RegisterRoutes wires the listings and categories routes onto mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/listings", h.listListings)
	mux.HandleFunc("GET /api/v1/listings/{id}", h.getListing)
	mux.HandleFunc("GET /api/v1/categories", h.listCategories)

	post := http.Handler(http.HandlerFunc(h.createListing))
	put := http.Handler(http.HandlerFunc(h.updateListing))
	del := http.Handler(http.HandlerFunc(h.deleteListing))
	presign := http.Handler(http.HandlerFunc(h.imagePresign))
	confirm := http.Handler(http.HandlerFunc(h.imageConfirm))
	imgDel := http.Handler(http.HandlerFunc(h.imageDelete))
	if h.bearer != nil {
		post = h.bearer(post)
		put = h.bearer(put)
		del = h.bearer(del)
		presign = h.bearer(presign)
		confirm = h.bearer(confirm)
		imgDel = h.bearer(imgDel)
	}
	mux.Handle("POST /api/v1/listings", post)
	mux.Handle("PUT /api/v1/listings/{id}", put)
	mux.Handle("DELETE /api/v1/listings/{id}", del)
	mux.Handle("POST /api/v1/listings/{id}/images:presign", presign)
	mux.Handle("POST /api/v1/listings/{id}/images:confirm", confirm)
	mux.Handle("DELETE /api/v1/listings/{id}/images/{imageId}", imgDel)
}

// listListings serves GET /api/v1/listings with optional lang, category,
// page/pageSize, and mine=true (auth required when set) filters.
func (h *Handler) listListings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	lang := q.Get("lang")
	if lang == "" {
		lang = "en"
	}
	if !ValidLangs[lang] {
		apierr.Write(w, http.StatusBadRequest, "invalid_lang", "lang must be en, si, or ta")
		return
	}

	category := q.Get("category")
	if category != "" && !ValidCategories[category] {
		apierr.Write(w, http.StatusBadRequest, "invalid_category", "unknown category slug")
		return
	}

	mine := q.Get("mine") == "true"
	var callerID string
	if mine {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || h.verifyToken == nil {
			apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required for mine=true")
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		uid, err := h.verifyToken(r.Context(), token)
		if err != nil {
			apierr.Write(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session")
			return
		}
		callerID = uid
	}

	page, pageSize := 1, defaultPageSize
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			apierr.Write(w, http.StatusBadRequest, "invalid_page", "page must be a positive integer")
			return
		}
		page = n
	}
	if v := q.Get("pageSize"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPageSize {
			apierr.Write(w, http.StatusBadRequest, "invalid_page_size", "pageSize must be between 1 and 100")
			return
		}
		pageSize = n
	}

	offset := (page - 1) * pageSize

	countQuery := `SELECT COUNT(*) FROM listings l JOIN categories c ON c.id = l.category_id WHERE l.status = 'active'`
	countArgs := []any{}
	argIdx := 1
	if mine {
		countQuery += fmt.Sprintf(" AND l.user_id = $%d", argIdx)
		countArgs = append(countArgs, callerID)
		argIdx++
	}
	if category != "" {
		countQuery += fmt.Sprintf(" AND c.slug = $%d", argIdx)
		countArgs = append(countArgs, category)
		argIdx++
	}

	var total int
	if err := h.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		log.Printf("listings: count query: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not count listings")
		return
	}

	// Data query: resolve title using lang → 'en' → any fallback (ADR 0001).
	// Cover thumbnail is fetched via a correlated subquery using idx_listing_images_listing_status.
	dataQuery := fmt.Sprintf(`
		SELECT l.id,
		       c.slug,
		       COALESCE(
		         (SELECT title FROM listing_translations WHERE listing_id = l.id AND lang = $1),
		         (SELECT title FROM listing_translations WHERE listing_id = l.id AND lang = 'en'),
		         (SELECT title FROM listing_translations WHERE listing_id = l.id LIMIT 1),
		         ''
		       ) AS title,
		       l.price_cents,
		       l.created_at,
		       (SELECT li.url FROM listing_images li
		        WHERE li.listing_id = l.id AND li.status = 'active'
		        ORDER BY li.sort_order ASC LIMIT 1) AS thumbnail_url
		FROM listings l
		JOIN categories c ON c.id = l.category_id
		WHERE l.status = 'active'`)

	dataArgs := []any{lang}
	argIdx = 2
	if mine {
		dataQuery += fmt.Sprintf(" AND l.user_id = $%d", argIdx)
		dataArgs = append(dataArgs, callerID)
		argIdx++
	}
	if category != "" {
		dataQuery += fmt.Sprintf(" AND c.slug = $%d", argIdx)
		dataArgs = append(dataArgs, category)
		argIdx++
	}
	dataQuery += fmt.Sprintf(" ORDER BY l.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	dataArgs = append(dataArgs, pageSize, offset)

	rows, err := h.db.QueryContext(r.Context(), dataQuery, dataArgs...)
	if err != nil {
		log.Printf("listings: data query: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch listings")
		return
	}
	defer rows.Close()

	items := make([]Summary, 0)
	for rows.Next() {
		var s Summary
		var priceCents sql.NullInt64
		var thumbnailURL sql.NullString
		if err := rows.Scan(&s.ID, &s.Category, &s.Title, &priceCents, &s.CreatedAt, &thumbnailURL); err != nil {
			log.Printf("listings: scan row: %v", err)
			apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read listing")
			return
		}
		if priceCents.Valid {
			lkr := priceCents.Int64 / 100
			s.PriceLKR = &lkr
		}
		if thumbnailURL.Valid {
			s.ThumbnailURL = &thumbnailURL.String
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		log.Printf("listings: rows error: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read listings")
		return
	}

	writeJSON(w, http.StatusOK, Page{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

// getListing serves GET /api/v1/listings/{id}. Returns the full listing detail
// with title and description resolved using the ADR 0001 lang fallback chain.
func (h *Handler) getListing(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}
	if !ValidLangs[lang] {
		apierr.Write(w, http.StatusBadRequest, "invalid_lang", "lang must be en, si, or ta")
		return
	}

	var d Detail
	var priceCents sql.NullInt64
	var description sql.NullString
	var translationSource sql.NullString

	err := h.db.QueryRowContext(r.Context(), `
		SELECT l.id,
		       c.slug,
		       l.content_language,
		       COALESCE(
		         (SELECT title FROM listing_translations WHERE listing_id = l.id AND lang = $2),
		         (SELECT title FROM listing_translations WHERE listing_id = l.id AND lang = 'en'),
		         (SELECT title FROM listing_translations WHERE listing_id = l.id LIMIT 1),
		         ''
		       ) AS title,
		       COALESCE(
		         (SELECT description FROM listing_translations WHERE listing_id = l.id AND lang = $2),
		         (SELECT description FROM listing_translations WHERE listing_id = l.id AND lang = 'en'),
		         (SELECT description FROM listing_translations WHERE listing_id = l.id LIMIT 1)
		       ) AS description,
		       COALESCE(
		         (SELECT source::text FROM listing_translations WHERE listing_id = l.id AND lang = $2),
		         (SELECT source::text FROM listing_translations WHERE listing_id = l.id AND lang = 'en'),
		         (SELECT source::text FROM listing_translations WHERE listing_id = l.id LIMIT 1)
		       ) AS translation_source,
		       l.price_cents,
		       l.created_at
		FROM listings l
		JOIN categories c ON c.id = l.category_id
		WHERE l.id = $1 AND l.status = 'active'
	`, id, lang).Scan(&d.ID, &d.Category, &d.ContentLanguage, &d.Title, &description, &translationSource, &priceCents, &d.CreatedAt)

	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		log.Printf("listings: get listing: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch listing")
		return
	}

	if description.Valid {
		d.Description = description.String
	}
	if priceCents.Valid {
		lkr := priceCents.Int64 / 100
		d.PriceLKR = &lkr
	}
	if translationSource.Valid {
		d.TranslationSource = &translationSource.String
	}

	imgRows, err := h.db.QueryContext(r.Context(), `
		SELECT id, url, sort_order
		FROM listing_images
		WHERE listing_id = $1 AND status = 'active'
		ORDER BY sort_order ASC
	`, id)
	if err != nil {
		log.Printf("listings: get images: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch images")
		return
	}
	defer imgRows.Close()
	d.Images = make([]ImageRecord, 0)
	for imgRows.Next() {
		var img ImageRecord
		if err := imgRows.Scan(&img.ID, &img.URL, &img.SortOrder); err != nil {
			log.Printf("listings: scan image: %v", err)
			apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read images")
			return
		}
		d.Images = append(d.Images, img)
	}
	if err := imgRows.Err(); err != nil {
		log.Printf("listings: images rows error: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read images")
		return
	}
	if len(d.Images) > 0 {
		d.ThumbnailURL = &d.Images[0].URL
	}

	writeJSON(w, http.StatusOK, d)
}

// createListing serves POST /api/v1/listings. Requires bearer auth.
// Calls the PostingPolicy before touching the DB.
func (h *Handler) createListing(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}

	// Policy check FIRST — before parsing or touching the DB.
	d := h.policy.CheckCanPost(r.Context(), userID)
	if !d.Allowed {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":        d.Code,
			"message":     d.Message,
			"retry_after": d.RetryAfter,
		})
		return
	}

	var req ListingCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}

	if !ValidCategories[req.Category] {
		apierr.Write(w, http.StatusBadRequest, "invalid_category", "unknown category slug")
		return
	}
	if !ValidLangs[req.ContentLanguage] {
		apierr.Write(w, http.StatusBadRequest, "invalid_content_language", "content_language must be en, si, or ta")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		apierr.Write(w, http.StatusBadRequest, "missing_title", "title is required")
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		apierr.Write(w, http.StatusBadRequest, "missing_description", "description is required")
		return
	}

	var categoryID string
	err := h.db.QueryRowContext(r.Context(), `SELECT id FROM categories WHERE slug = $1`, req.Category).Scan(&categoryID)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusBadRequest, "invalid_category", "category not found")
		return
	}
	if err != nil {
		log.Printf("listings: lookup category: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not look up category")
		return
	}

	var priceCents sql.NullInt64
	if req.PriceLKR != nil {
		priceCents = sql.NullInt64{Int64: *req.PriceLKR * 100, Valid: true}
	}

	var listingID string
	var createdAt time.Time
	err = h.db.QueryRowContext(r.Context(), `
		INSERT INTO listings (user_id, category_id, content_language, price_cents, currency, status)
		VALUES ($1, $2, $3, $4, 'LKR', 'active')
		RETURNING id, created_at
	`, userID, categoryID, req.ContentLanguage, priceCents).Scan(&listingID, &createdAt)
	if err != nil {
		log.Printf("listings: insert listing: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not create listing")
		return
	}

	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO listing_translations (listing_id, lang, title, description, source)
		VALUES ($1, $2, $3, $4, 'human')
	`, listingID, req.ContentLanguage, req.Title, req.Description)
	if err != nil {
		log.Printf("listings: insert translation: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not save listing content")
		return
	}

	var priceLKR *int64
	if priceCents.Valid {
		lkr := priceCents.Int64 / 100
		priceLKR = &lkr
	}
	src := "human"
	writeJSON(w, http.StatusCreated, Detail{
		ID:                listingID,
		Category:          req.Category,
		ContentLanguage:   req.ContentLanguage,
		Title:             req.Title,
		Description:       req.Description,
		PriceLKR:          priceLKR,
		TranslationSource: &src,
		CreatedAt:         createdAt,
	})
}

// updateListing serves PUT /api/v1/listings/{id}. Requires bearer auth and
// ownership: the caller must own the listing (else 403); an unknown or already
// removed listing returns 404. content_language is immutable, so title and
// description edits update the listing's existing authored-language translation.
func (h *Handler) updateListing(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}
	id := r.PathValue("id")

	var req ListingUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}
	if !ValidCategories[req.Category] {
		apierr.Write(w, http.StatusBadRequest, "invalid_category", "unknown category slug")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		apierr.Write(w, http.StatusBadRequest, "missing_title", "title is required")
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		apierr.Write(w, http.StatusBadRequest, "missing_description", "description is required")
		return
	}

	ownerID, contentLang, err := h.listingOwner(r.Context(), id)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		log.Printf("listings: load listing for update: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not load listing")
		return
	}
	if ownerID != userID {
		apierr.Write(w, http.StatusForbidden, "forbidden", "you do not own this listing")
		return
	}

	var categoryID string
	err = h.db.QueryRowContext(r.Context(), `SELECT id FROM categories WHERE slug = $1`, req.Category).Scan(&categoryID)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusBadRequest, "invalid_category", "category not found")
		return
	}
	if err != nil {
		log.Printf("listings: lookup category: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not look up category")
		return
	}

	var priceCents sql.NullInt64
	if req.PriceLKR != nil {
		priceCents = sql.NullInt64{Int64: *req.PriceLKR * 100, Valid: true}
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("listings: begin update tx: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not update listing")
		return
	}
	defer tx.Rollback()

	var createdAt time.Time
	err = tx.QueryRowContext(r.Context(), `
		UPDATE listings
		SET category_id = $1, price_cents = $2, updated_at = now()
		WHERE id = $3
		RETURNING created_at
	`, categoryID, priceCents, id).Scan(&createdAt)
	if err != nil {
		log.Printf("listings: update listing: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not update listing")
		return
	}

	// Upsert the authored-language translation (it always exists post-create,
	// but ON CONFLICT keeps this safe if the row was ever absent).
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO listing_translations (listing_id, lang, title, description, source)
		VALUES ($1, $2, $3, $4, 'human')
		ON CONFLICT (listing_id, lang)
		DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description, source = 'human'
	`, id, contentLang, req.Title, req.Description)
	if err != nil {
		log.Printf("listings: update translation: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not update listing content")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("listings: commit update: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not update listing")
		return
	}

	var priceLKR *int64
	if priceCents.Valid {
		lkr := priceCents.Int64 / 100
		priceLKR = &lkr
	}
	src := "human"
	writeJSON(w, http.StatusOK, Detail{
		ID:                id,
		Category:          req.Category,
		ContentLanguage:   contentLang,
		Title:             req.Title,
		Description:       req.Description,
		PriceLKR:          priceLKR,
		TranslationSource: &src,
		CreatedAt:         createdAt,
	})
}

// deleteListing serves DELETE /api/v1/listings/{id}. Requires bearer auth and
// ownership (403 for non-owners, 404 for unknown/already-removed). Soft-deletes
// by setting status='removed' so the row drops out of catalog and dashboard
// (both filter status='active') while remaining recoverable.
func (h *Handler) deleteListing(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}
	id := r.PathValue("id")

	ownerID, _, err := h.listingOwner(r.Context(), id)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		log.Printf("listings: load listing for delete: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not load listing")
		return
	}
	if ownerID != userID {
		apierr.Write(w, http.StatusForbidden, "forbidden", "you do not own this listing")
		return
	}

	_, err = h.db.ExecContext(r.Context(), `
		UPDATE listings SET status = 'removed', updated_at = now() WHERE id = $1
	`, id)
	if err != nil {
		log.Printf("listings: delete listing: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not delete listing")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listingOwner returns the owner user_id and content_language of an active
// listing. Returns sql.ErrNoRows when the listing does not exist or is not
// active (already removed/sold), which callers map to 404.
func (h *Handler) listingOwner(ctx context.Context, id string) (ownerID, contentLang string, err error) {
	err = h.db.QueryRowContext(ctx, `
		SELECT user_id, content_language FROM listings WHERE id = $1 AND status = 'active'
	`, id).Scan(&ownerID, &contentLang)
	return ownerID, contentLang, err
}

// listCategories serves GET /api/v1/categories with the requested lang
// resolved through the ADR 0001 fallback chain (requested → en → any).
func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}
	if !ValidLangs[lang] {
		apierr.Write(w, http.StatusBadRequest, "invalid_lang", "lang must be en, si, or ta")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT c.slug,
		       COALESCE(
		         (SELECT name FROM category_translations WHERE category_id = c.id AND lang = $1),
		         (SELECT name FROM category_translations WHERE category_id = c.id AND lang = 'en'),
		         (SELECT name FROM category_translations WHERE category_id = c.id LIMIT 1),
		         c.slug
		       ) AS name,
		       (SELECT slug FROM categories WHERE id = c.parent_id) AS parent_slug,
		       c.sort_order
		FROM categories c
		ORDER BY c.sort_order, name
	`, lang)
	if err != nil {
		log.Printf("listings: categories query: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch categories")
		return
	}
	defer rows.Close()

	cats := make([]Category, 0)
	for rows.Next() {
		var cat Category
		var parentSlug sql.NullString
		if err := rows.Scan(&cat.Slug, &cat.Name, &parentSlug, &cat.SortOrder); err != nil {
			log.Printf("listings: scan category: %v", err)
			apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read category")
			return
		}
		if parentSlug.Valid {
			cat.ParentSlug = &parentSlug.String
		}
		cats = append(cats, cat)
	}
	if err := rows.Err(); err != nil {
		log.Printf("listings: category rows error: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read categories")
		return
	}

	writeJSON(w, http.StatusOK, cats)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
