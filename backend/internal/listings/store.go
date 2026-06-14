package listings

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DevOwnerID is the stub owner used until auth lands (E10 / VER-10).
// Seeded by migration 0003_seed_categories.
const DevOwnerID = "00000000-0000-0000-0000-000000000001"

// ValidCategories mirrors the CategorySlug enum in api/openapi.yaml.
var ValidCategories = map[string]bool{
	"electronics": true, "vehicles": true, "property": true,
	"home_garden": true, "fashion": true, "mobile_phones": true,
	"services": true, "jobs": true, "pets": true, "other": true,
}

// ValidLangs mirrors the Lang enum in api/openapi.yaml.
var ValidLangs = map[string]bool{
	"en": true, "si": true, "ta": true,
}

// Store wraps a pgx pool and provides all listing DB operations.
// It uses the schema defined in migration 0002_feature_schema:
//   - listings (user_id FK, category_id FK, price_cents, status)
//   - listing_translations (listing_id, lang, title, description, source)
//   - listing_images (listing_id, object_key, sort_order)
//
// Image public URLs are derived at read time: imageBaseURL + "/" + object_key.
type Store struct {
	db           *pgxpool.Pool
	imageBaseURL string // e.g. "http://localhost:8080/api/v1/images"
}

// NewStore returns a Store backed by db.
func NewStore(db *pgxpool.Pool, imageBaseURL string) *Store {
	return &Store{db: db, imageBaseURL: imageBaseURL}
}

func (s *Store) imageURL(key string) string {
	return s.imageBaseURL + "/" + key
}

// Create inserts a new active listing and its authored translation.
func (s *Store) Create(ctx context.Context, ownerID string, req CreateRequest) (*Listing, error) {
	// Resolve category slug → UUID.
	var categoryID string
	if err := s.db.QueryRow(ctx,
		`SELECT id FROM categories WHERE slug = $1`, req.Category).Scan(&categoryID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("store: unknown category %q", req.Category)
		}
		return nil, fmt.Errorf("store: resolve category: %w", err)
	}

	priceCents := lkrToCents(req.PriceLKR)

	var id, contentLanguage string
	var createdAt time.Time
	if err := s.db.QueryRow(ctx, `
		INSERT INTO listings (user_id, category_id, content_language, price_cents, status)
		VALUES ($1, $2, $3, $4, 'active')
		RETURNING id, content_language, created_at
	`, ownerID, categoryID, req.ContentLanguage, priceCents).
		Scan(&id, &contentLanguage, &createdAt); err != nil {
		return nil, fmt.Errorf("store: create listing: %w", err)
	}

	if _, err := s.db.Exec(ctx, `
		INSERT INTO listing_translations (listing_id, lang, title, description, source)
		VALUES ($1, $2, $3, $4, 'human')
	`, id, contentLanguage, req.Title, req.Description); err != nil {
		return nil, fmt.Errorf("store: create translation: %w", err)
	}

	return &Listing{
		ID:              id,
		OwnerID:         ownerID,
		Category:        req.Category,
		ContentLanguage: contentLanguage,
		Title:           req.Title,
		Description:     req.Description,
		PriceLKR:        req.PriceLKR,
		Images:          []Image{},
		CreatedAt:       createdAt,
	}, nil
}

// Get fetches a full listing by ID, or returns nil if not found/removed.
func (s *Store) Get(ctx context.Context, id string) (*Listing, error) {
	type scanRow struct {
		ID              string
		UserID          string
		CategorySlug    string
		ContentLanguage string
		Title           string
		Description     string
		PriceCents      *int64
		CreatedAt       time.Time
	}
	var r scanRow
	err := s.db.QueryRow(ctx, `
		SELECT l.id, l.user_id, c.slug, l.content_language,
		       COALESCE(lt.title, ''), COALESCE(lt.description, ''),
		       l.price_cents, l.created_at
		FROM listings l
		JOIN categories c ON c.id = l.category_id
		LEFT JOIN listing_translations lt
		       ON lt.listing_id = l.id AND lt.lang = l.content_language
		WHERE l.id = $1 AND l.status = 'active'
	`, id).Scan(&r.ID, &r.UserID, &r.CategorySlug, &r.ContentLanguage,
		&r.Title, &r.Description, &r.PriceCents, &r.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get listing: %w", err)
	}

	images, err := s.loadImages(ctx, id)
	if err != nil {
		return nil, err
	}

	return &Listing{
		ID:              r.ID,
		OwnerID:         r.UserID,
		Category:        r.CategorySlug,
		ContentLanguage: r.ContentLanguage,
		Title:           r.Title,
		Description:     r.Description,
		PriceLKR:        centsToLKR(r.PriceCents),
		Images:          images,
		CreatedAt:       r.CreatedAt,
	}, nil
}

// List returns a paginated, optionally category-filtered page of active listings.
func (s *Store) List(ctx context.Context, category string, page, pageSize int) (*Page, error) {
	offset := (page - 1) * pageSize

	var total int
	var rows pgx.Rows
	var err error

	if category != "" {
		if err = s.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM listings l
			JOIN categories c ON c.id = l.category_id
			WHERE l.status = 'active' AND c.slug = $1
		`, category).Scan(&total); err != nil {
			return nil, fmt.Errorf("store: count listings: %w", err)
		}
		rows, err = s.db.Query(ctx, `
			SELECT l.id, c.slug, COALESCE(lt.title, ''), l.price_cents, l.created_at,
			       (SELECT object_key FROM listing_images
			        WHERE listing_id = l.id ORDER BY sort_order LIMIT 1)
			FROM listings l
			JOIN categories c ON c.id = l.category_id
			LEFT JOIN listing_translations lt
			       ON lt.listing_id = l.id AND lt.lang = l.content_language
			WHERE l.status = 'active' AND c.slug = $1
			ORDER BY l.created_at DESC
			LIMIT $2 OFFSET $3
		`, category, pageSize, offset)
	} else {
		if err = s.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM listings WHERE status = 'active'`).Scan(&total); err != nil {
			return nil, fmt.Errorf("store: count listings: %w", err)
		}
		rows, err = s.db.Query(ctx, `
			SELECT l.id, c.slug, COALESCE(lt.title, ''), l.price_cents, l.created_at,
			       (SELECT object_key FROM listing_images
			        WHERE listing_id = l.id ORDER BY sort_order LIMIT 1)
			FROM listings l
			JOIN categories c ON c.id = l.category_id
			LEFT JOIN listing_translations lt
			       ON lt.listing_id = l.id AND lt.lang = l.content_language
			WHERE l.status = 'active'
			ORDER BY l.created_at DESC
			LIMIT $1 OFFSET $2
		`, pageSize, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("store: list listings: %w", err)
	}
	defer rows.Close()

	items := []Summary{}
	for rows.Next() {
		var item Summary
		var priceCents *int64
		var thumbnailKey *string
		if err := rows.Scan(&item.ID, &item.Category, &item.Title,
			&priceCents, &item.CreatedAt, &thumbnailKey); err != nil {
			return nil, fmt.Errorf("store: scan summary: %w", err)
		}
		item.PriceLKR = centsToLKR(priceCents)
		if thumbnailKey != nil {
			u := s.imageURL(*thumbnailKey)
			item.ThumbnailURL = &u
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list rows: %w", err)
	}

	return &Page{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// Update replaces the mutable fields (category, title, description, price) of a listing.
func (s *Store) Update(ctx context.Context, id, ownerID string, req UpdateRequest) (*Listing, error) {
	var categoryID string
	if err := s.db.QueryRow(ctx,
		`SELECT id FROM categories WHERE slug = $1`, req.Category).Scan(&categoryID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("store: unknown category %q", req.Category)
		}
		return nil, fmt.Errorf("store: resolve category: %w", err)
	}

	priceCents := lkrToCents(req.PriceLKR)

	var listingID, contentLanguage string
	var createdAt time.Time
	err := s.db.QueryRow(ctx, `
		UPDATE listings
		SET category_id = $3, price_cents = $4, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'active'
		RETURNING id, content_language, created_at
	`, id, ownerID, categoryID, priceCents).
		Scan(&listingID, &contentLanguage, &createdAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: update listing: %w", err)
	}

	// Upsert translation for the original content_language.
	if _, err := s.db.Exec(ctx, `
		INSERT INTO listing_translations (listing_id, lang, title, description, source)
		VALUES ($1, $2, $3, $4, 'human')
		ON CONFLICT (listing_id, lang) DO UPDATE
		SET title = EXCLUDED.title, description = EXCLUDED.description
	`, listingID, contentLanguage, req.Title, req.Description); err != nil {
		return nil, fmt.Errorf("store: upsert translation: %w", err)
	}

	images, err := s.loadImages(ctx, listingID)
	if err != nil {
		return nil, err
	}

	return &Listing{
		ID:              listingID,
		OwnerID:         ownerID,
		Category:        req.Category,
		ContentLanguage: contentLanguage,
		Title:           req.Title,
		Description:     req.Description,
		PriceLKR:        req.PriceLKR,
		Images:          images,
		CreatedAt:       createdAt,
	}, nil
}

// Delete soft-deletes a listing by setting status = 'removed'.
func (s *Store) Delete(ctx context.Context, id, ownerID string) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE listings SET status = 'removed', updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'active'
	`, id, ownerID)
	if err != nil {
		return false, fmt.Errorf("store: delete listing: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// AddImage inserts a listing_images row and returns the new Image.
// sortOrder should be the current image count (0-based next index).
func (s *Store) AddImage(ctx context.Context, listingID, objectKey string, sortOrder int) (*Image, error) {
	var id string
	var order int
	if err := s.db.QueryRow(ctx, `
		INSERT INTO listing_images (listing_id, object_key, sort_order)
		VALUES ($1, $2, $3)
		RETURNING id, sort_order
	`, listingID, objectKey, sortOrder).Scan(&id, &order); err != nil {
		return nil, fmt.Errorf("store: add image: %w", err)
	}
	return &Image{ID: id, URL: s.imageURL(objectKey), SortOrder: order}, nil
}

// ImageCount returns the number of images attached to a listing.
func (s *Store) ImageCount(ctx context.Context, listingID string) (int, error) {
	var count int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM listing_images WHERE listing_id = $1`, listingID).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: image count: %w", err)
	}
	return count, nil
}

// CachedTranslation holds a machine-generated translation row.
type CachedTranslation struct {
	Title       string
	Description string
}

// GetCachedTranslation returns the machine translation for listingID in lang,
// or nil if no machine translation exists yet.
func (s *Store) GetCachedTranslation(ctx context.Context, listingID, lang string) (*CachedTranslation, error) {
	var t CachedTranslation
	err := s.db.QueryRow(ctx, `
		SELECT title, COALESCE(description, '')
		FROM listing_translations
		WHERE listing_id = $1 AND lang = $2 AND source = 'machine'
	`, listingID, lang).Scan(&t.Title, &t.Description)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get cached translation: %w", err)
	}
	return &t, nil
}

// UpsertMachineTranslation writes a Claude-generated translation.
// It will never overwrite a human-authored translation (source='human') — the
// WHERE clause on DO UPDATE enforces ADR 0001.
func (s *Store) UpsertMachineTranslation(ctx context.Context, listingID, lang, title, description string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO listing_translations (listing_id, lang, title, description, source, generated_at)
		VALUES ($1, $2, $3, $4, 'machine', now())
		ON CONFLICT (listing_id, lang) DO UPDATE
			SET title        = EXCLUDED.title,
			    description  = EXCLUDED.description,
			    generated_at = now()
			WHERE listing_translations.source = 'machine'
	`, listingID, lang, title, description)
	if err != nil {
		return fmt.Errorf("store: upsert machine translation: %w", err)
	}
	return nil
}

// ListingExists returns true if id refers to an active listing.
func (s *Store) ListingExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM listings WHERE id = $1 AND status = 'active')`, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("store: listing exists: %w", err)
	}
	return exists, nil
}

// --- helpers -----------------------------------------------------------------

func (s *Store) loadImages(ctx context.Context, listingID string) ([]Image, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, object_key, sort_order FROM listing_images
		WHERE listing_id = $1
		ORDER BY sort_order
	`, listingID)
	if err != nil {
		return nil, fmt.Errorf("store: load images: %w", err)
	}
	defer rows.Close()
	images := []Image{}
	for rows.Next() {
		var img Image
		var key string
		if err := rows.Scan(&img.ID, &key, &img.SortOrder); err != nil {
			return nil, fmt.Errorf("store: scan image: %w", err)
		}
		img.URL = s.imageURL(key)
		images = append(images, img)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: image rows: %w", err)
	}
	return images, nil
}

// lkrToCents converts a nullable LKR amount to the price_cents storage unit (×100).
func lkrToCents(lkr *int64) *int64 {
	if lkr == nil {
		return nil
	}
	v := *lkr * 100
	return &v
}

// centsToLKR is the inverse of lkrToCents.
func centsToLKR(cents *int64) *int64 {
	if cents == nil {
		return nil
	}
	v := *cents / 100
	return &v
}
