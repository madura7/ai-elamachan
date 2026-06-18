package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/madura7/ai-elamachan/backend/internal/listings"
	"github.com/madura7/ai-elamachan/backend/internal/search"
)

// Stable synthetic identifiers for seed/test users.
// Fixed phones are the idempotency key (ON CONFLICT phone_e164).
// Both phones accept OTP "000000" on staging when DEV_OTP_BYPASS=true.
const (
	seedUserPhone  = "+94700000001"
	seedUserID     = "00000000-0000-4000-a000-000000000001"
	seedUser2Phone = "+94700000002"
	seedUser2ID    = "00000000-0000-4000-a000-000000000002"
)

// catSeed describes a category row and its translations.
type catSeed struct {
	id         string
	slug       string
	parentSlug string // empty for root categories
	sortOrder  int
	names      map[string]string // lang → display name
}

// listingSeed describes one listing + its single human-authored translation.
type listingSeed struct {
	id          string
	catSlug     string
	lang        string
	priceCents  int64
	title       string
	description string
}

// allCategories must be ordered: parents before children.
var allCategories = []catSeed{
	{
		id: "00000000-0000-4000-a000-000000000010", slug: "electronics",
		sortOrder: 0,
		names:     map[string]string{"en": "Electronics", "si": "විද්‍යුත් භාණ්ඩ", "ta": "மின்னணுவியல்"},
	},
	{
		id: "00000000-0000-4000-a000-000000000012", slug: "vehicles",
		sortOrder: 1,
		names:     map[string]string{"en": "Vehicles", "si": "වාහන", "ta": "வாகனங்கள்"},
	},
	{
		id: "00000000-0000-4000-a000-000000000014", slug: "furniture",
		sortOrder: 2,
		names:     map[string]string{"en": "Furniture", "si": "ගෘහ භාණ්ඩ", "ta": "தளபாடங்கள்"},
	},
	{
		id: "00000000-0000-4000-a000-000000000016", slug: "property",
		sortOrder: 3,
		names:     map[string]string{"en": "Property", "si": "දේපල", "ta": "சொத்து"},
	},
	{
		id: "00000000-0000-4000-a000-000000000018", slug: "services",
		sortOrder: 4,
		names:     map[string]string{"en": "Services", "si": "සේවා", "ta": "சேவைகள்"},
	},
	{
		id: "00000000-0000-4000-a000-000000000011", slug: "mobile-phones",
		parentSlug: "electronics",
		sortOrder:  0,
		names:      map[string]string{"en": "Mobile Phones", "si": "ජංගම දුරකථන", "ta": "மொபைல் போன்கள்"},
	},
	{
		id: "00000000-0000-4000-a000-000000000015", slug: "laptops",
		parentSlug: "electronics",
		sortOrder:  1,
		names:      map[string]string{"en": "Laptops", "si": "ලැප්ටොප්", "ta": "லேப்டாப்"},
	},
	{
		id: "00000000-0000-4000-a000-000000000013", slug: "cars",
		parentSlug: "vehicles",
		sortOrder:  0,
		names:      map[string]string{"en": "Cars", "si": "මෝටර් රථ", "ta": "கார்கள்"},
	},
	{
		id: "00000000-0000-4000-a000-000000000017", slug: "motorcycles",
		parentSlug: "vehicles",
		sortOrder:  1,
		names:      map[string]string{"en": "Motorcycles", "si": "යතුරුපැදි", "ta": "மோட்டார் சைக்கிள்"},
	},
}

var allListings = []listingSeed{
	// Mobile phones
	{
		id: "00000000-0000-4000-a000-000000000020", catSlug: "mobile-phones",
		lang: "en", priceCents: 450000,
		title:       "iPhone 14 Pro",
		description: "Apple iPhone 14 Pro 256GB Space Black, excellent condition, original box and accessories included.",
	},
	{
		id: "00000000-0000-4000-a000-000000000021", catSlug: "mobile-phones",
		lang: "en", priceCents: 380000,
		title:       "Samsung Galaxy S24",
		description: "Samsung Galaxy S24 128GB Phantom Black, lightly used (10 months), all original accessories.",
	},
	{
		id: "00000000-0000-4000-a000-000000000029", catSlug: "mobile-phones",
		lang: "en", priceCents: 185000,
		title:       "Xiaomi Redmi Note 13",
		description: "Xiaomi Redmi Note 13 128GB Arctic White, brand new in sealed box, dual SIM.",
	},
	// Laptops
	{
		id: "00000000-0000-4000-a000-000000000030", catSlug: "laptops",
		lang: "en", priceCents: 520000,
		title:       "Dell XPS 15 (2023)",
		description: "Dell XPS 15 Core i7, 16GB RAM, 512GB SSD, OLED 4K display. Excellent condition, 14 months old.",
	},
	{
		id: "00000000-0000-4000-a000-000000000031", catSlug: "laptops",
		lang: "en", priceCents: 310000,
		title:       "MacBook Air M1",
		description: "Apple MacBook Air M1, 8GB RAM, 256GB SSD, Space Grey. Battery health 91%, minor scuffs on lid.",
	},
	// Cars
	{
		id: "00000000-0000-4000-a000-000000000022", catSlug: "cars",
		lang: "en", priceCents: 8500000,
		title:       "Toyota Corolla 2020",
		description: "Toyota Corolla 2020 1.6L manual, 45,000 km, service records available, first owner.",
	},
	{
		id: "00000000-0000-4000-a000-000000000023", catSlug: "cars",
		lang: "en", priceCents: 7200000,
		title:       "Honda Civic 2019",
		description: "Honda Civic 2019 1.8L automatic, 52,000 km, reverse camera, power steering, excellent condition.",
	},
	{
		id: "00000000-0000-4000-a000-000000000032", catSlug: "cars",
		lang: "en", priceCents: 5900000,
		title:       "Suzuki Alto 2021",
		description: "Suzuki Alto 2021 0.66L automatic, 22,000 km, single owner, full service history, accident-free.",
	},
	// Motorcycles
	{
		id: "00000000-0000-4000-a000-000000000033", catSlug: "motorcycles",
		lang: "en", priceCents: 650000,
		title:       "Honda CB150R",
		description: "Honda CB150R 2022, 8,500 km, excellent condition, genuine parts only, never raced.",
	},
	// Furniture
	{
		id: "00000000-0000-4000-a000-000000000024", catSlug: "furniture",
		lang: "en", priceCents: 65000,
		title:       "Teak Dining Table (6-Seater)",
		description: "Solid teak wood 6-seater dining table, 1.8m × 0.9m, minor surface scratches. Chairs not included.",
	},
	{
		id: "00000000-0000-4000-a000-000000000034", catSlug: "furniture",
		lang: "en", priceCents: 42000,
		title:       "3-Seater Fabric Sofa",
		description: "Light grey fabric sofa, 3-seater, good condition, non-smoking home, minor pilling on armrests.",
	},
	{
		id: "00000000-0000-4000-a000-000000000035", catSlug: "furniture",
		lang: "en", priceCents: 28000,
		title:       "Queen Bed Frame with Storage",
		description: "Wooden queen bed frame with under-bed drawers, 2 years old, no mattress, self-pickup Colombo 5.",
	},
}

// seedPostgres inserts the demo taxonomy and listings into Postgres.
// All inserts use ON CONFLICT DO NOTHING so re-runs are no-ops.
func seedPostgres(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// 1. Seed users (idempotent via phone_e164 UNIQUE).
	// Both phones accept OTP "000000" on staging when DEV_OTP_BYPASS=true.
	type seedUser struct{ id, phone, name string }
	seedUsers := []seedUser{
		{seedUserID, seedUserPhone, "Test User One"},
		{seedUser2ID, seedUser2Phone, "Test User Two"},
	}
	for _, u := range seedUsers {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users (id, phone_e164, phone_verified_at, display_name, preferred_language)
			VALUES ($1, $2, now(), $3, 'en')
			ON CONFLICT (phone_e164) DO NOTHING`,
			u.id, u.phone, u.name); err != nil {
			return fmt.Errorf("upsert seed user %s: %w", u.phone, err)
		}
	}

	var userID string
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM users WHERE phone_e164 = $1`, seedUserPhone,
	).Scan(&userID); err != nil {
		return fmt.Errorf("resolve seed user: %w", err)
	}

	// 2. Categories (parents before children, guaranteed by allCategories ordering)
	for _, c := range allCategories {
		if err := upsertCategory(ctx, tx, c); err != nil {
			return err
		}
	}

	// 3. Listings + translations
	for _, l := range allListings {
		var catID string
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM categories WHERE slug = $1`, l.catSlug,
		).Scan(&catID); err != nil {
			return fmt.Errorf("resolve category %q: %w", l.catSlug, err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO listings
				(id, user_id, category_id, content_language, price_cents, currency, status)
			VALUES ($1, $2, $3, $4, $5, 'LKR', 'active')
			ON CONFLICT (id) DO NOTHING`,
			l.id, userID, catID, l.lang, l.priceCents); err != nil {
			return fmt.Errorf("upsert listing %s: %w", l.id, err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO listing_translations (listing_id, lang, title, description, source)
			VALUES ($1, $2, $3, $4, 'human'::listing_translation_source)
			ON CONFLICT (listing_id, lang) DO NOTHING`,
			l.id, l.lang, l.title, l.description); err != nil {
			return fmt.Errorf("upsert listing_translation %s/%s: %w", l.id, l.lang, err)
		}
	}

	return tx.Commit()
}

func upsertCategory(ctx context.Context, tx *sql.Tx, c catSeed) error {
	if c.parentSlug == "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO categories (id, slug, sort_order)
			VALUES ($1, $2, $3)
			ON CONFLICT (slug) DO NOTHING`,
			c.id, c.slug, c.sortOrder); err != nil {
			return fmt.Errorf("upsert category %q: %w", c.slug, err)
		}
	} else {
		// Look up parent ID by slug to stay correct even if the parent was
		// inserted on a prior run with a different UUID.
		var parentID string
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM categories WHERE slug = $1`, c.parentSlug,
		).Scan(&parentID); err != nil {
			return fmt.Errorf("resolve parent category %q: %w", c.parentSlug, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO categories (id, slug, parent_id, sort_order)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (slug) DO NOTHING`,
			c.id, c.slug, parentID, c.sortOrder); err != nil {
			return fmt.Errorf("upsert category %q: %w", c.slug, err)
		}
	}

	// Resolve the actual UUID (may differ from c.id if the slug pre-existed).
	var actualID string
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM categories WHERE slug = $1`, c.slug,
	).Scan(&actualID); err != nil {
		return fmt.Errorf("resolve category %q after upsert: %w", c.slug, err)
	}

	for lang, name := range c.names {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO category_translations (category_id, lang, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (category_id, lang) DO NOTHING`,
			actualID, lang, name); err != nil {
			return fmt.Errorf("upsert category_translation %q/%s: %w", c.slug, lang, err)
		}
	}

	return nil
}

// ── Meilisearch ──────────────────────────────────────────────────────────────

// buildSeedIndexDocs converts the in-memory seed data into IndexableDocs.
// Seed listings have no images (has_image = false).
func buildSeedIndexDocs() []listings.IndexableDoc {
	docs := make([]listings.IndexableDoc, len(allListings))
	for i, l := range allListings {
		priceLKR := l.priceCents / 100
		docs[i] = listings.IndexableDoc{
			ID:              l.id,
			Category:        l.catSlug,
			ContentLanguage: l.lang,
			Title:           l.title,
			HasImage:        false,
			PriceLKR:        &priceLKR,
		}
	}
	return docs
}

// seedMeilisearch ensures the listings index exists with correct settings,
// then upserts the demo documents. All operations are idempotent.
// Reads MEILI_URL and MEILI_MASTER_KEY from environment.
func seedMeilisearch(ctx context.Context) error {
	svc, err := search.NewFromEnv()
	if err != nil {
		return fmt.Errorf("search client: %w", err)
	}
	if svc == nil {
		return fmt.Errorf("MEILI_URL not set")
	}

	if err := svc.EnsureIndex(ctx); err != nil {
		return fmt.Errorf("ensure index: %w", err)
	}

	if err := svc.BatchIndexListings(ctx, buildSeedIndexDocs()); err != nil {
		return fmt.Errorf("push documents: %w", err)
	}

	return nil
}
