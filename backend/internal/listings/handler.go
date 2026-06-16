package listings

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"

	// pgx registers the "pgx" driver name with database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Handler serves GET /api/v1/listings and GET /api/v1/categories.
type Handler struct {
	db *sql.DB
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
	return &Handler{db: db}, nil
}

// RegisterRoutes wires the listings and categories routes onto mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/listings", h.listListings)
	mux.HandleFunc("GET /api/v1/listings/{id}", h.getListing)
	mux.HandleFunc("GET /api/v1/categories", h.listCategories)
}

// listListings serves GET /api/v1/listings with optional lang and category
// filters plus page/pageSize pagination.
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

	// Count query.
	var total int
	countQuery := `SELECT COUNT(*) FROM listings l JOIN categories c ON c.id = l.category_id WHERE l.status = 'active'`
	countArgs := []any{}
	if category != "" {
		countQuery += " AND c.slug = $1"
		countArgs = append(countArgs, category)
	}
	if err := h.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		log.Printf("listings: count query: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not count listings")
		return
	}

	// Data query: resolve title using lang → 'en' → any fallback (ADR 0001).
	dataQuery := `
		SELECT l.id,
		       c.slug,
		       COALESCE(
		         (SELECT title FROM listing_translations WHERE listing_id = l.id AND lang = $1),
		         (SELECT title FROM listing_translations WHERE listing_id = l.id AND lang = 'en'),
		         (SELECT title FROM listing_translations WHERE listing_id = l.id LIMIT 1),
		         ''
		       ) AS title,
		       l.price_cents,
		       l.created_at
		FROM listings l
		JOIN categories c ON c.id = l.category_id
		WHERE l.status = 'active'`

	dataArgs := []any{lang}
	argIdx := 2
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
		if err := rows.Scan(&s.ID, &s.Category, &s.Title, &priceCents, &s.CreatedAt); err != nil {
			log.Printf("listings: scan row: %v", err)
			apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read listing")
			return
		}
		if priceCents.Valid {
			lkr := priceCents.Int64 / 100
			s.PriceLKR = &lkr
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

	writeJSON(w, http.StatusOK, d)
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
