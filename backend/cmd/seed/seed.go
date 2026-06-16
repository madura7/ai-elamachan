package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Stable synthetic identifiers for the seed user.
// Using a fixed phone means ON CONFLICT (phone_e164) is the idempotency key.
const (
	seedUserPhone = "+94700000001"
	seedUserID    = "00000000-0000-4000-a000-000000000001"
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
		id: "00000000-0000-4000-a000-000000000011", slug: "mobile-phones",
		parentSlug: "electronics",
		sortOrder:  0,
		names:      map[string]string{"en": "Mobile Phones", "si": "ජංගම දුරකථන", "ta": "மொபைல் போன்கள்"},
	},
	{
		id: "00000000-0000-4000-a000-000000000013", slug: "cars",
		parentSlug: "vehicles",
		sortOrder:  0,
		names:      map[string]string{"en": "Cars", "si": "මෝටර් රථ", "ta": "கார்கள்"},
	},
}

var allListings = []listingSeed{
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
		id: "00000000-0000-4000-a000-000000000024", catSlug: "furniture",
		lang: "en", priceCents: 65000,
		title:       "Teak Dining Table (6-Seater)",
		description: "Solid teak wood 6-seater dining table, 1.8m × 0.9m, minor surface scratches. Chairs not included.",
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

	// 1. Seed user (idempotent via phone_e164 UNIQUE)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, phone_e164, phone_verified_at, display_name, preferred_language)
		VALUES ($1, $2, now(), 'Seed User', 'en')
		ON CONFLICT (phone_e164) DO NOTHING`,
		seedUserID, seedUserPhone); err != nil {
		return fmt.Errorf("upsert seed user: %w", err)
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

var meiliSettings = map[string]any{
	"searchableAttributes": []string{"title", "description", "category_slug"},
	"filterableAttributes": []string{"category_slug", "status", "currency"},
	"sortableAttributes":   []string{"price_cents"},
}

func buildMeiliDocs() []map[string]any {
	docs := make([]map[string]any, len(allListings))
	for i, l := range allListings {
		docs[i] = map[string]any{
			"id":            l.id,
			"title":         l.title,
			"description":   l.description,
			"category_slug": l.catSlug,
			"price_cents":   l.priceCents,
			"currency":      "LKR",
			"status":        "active",
			"lang":          l.lang,
		}
	}
	return docs
}

// seedMeilisearch ensures the listings index exists with correct settings,
// then upserts the demo documents. All operations are idempotent.
func seedMeilisearch(ctx context.Context, baseURL, apiKey string) error {
	if err := ensureMeiliIndex(ctx, baseURL, apiKey); err != nil {
		return err
	}

	if err := meiliReq(ctx, "PATCH", baseURL, "/indexes/listings/settings", apiKey, meiliSettings); err != nil {
		return fmt.Errorf("update settings: %w", err)
	}

	if err := meiliReq(ctx, "PUT", baseURL, "/indexes/listings/documents", apiKey, buildMeiliDocs()); err != nil {
		return fmt.Errorf("push documents: %w", err)
	}

	return nil
}

// ensureMeiliIndex checks whether the listings index exists and creates it if
// not. It polls until the index is ready (up to 10 s) before returning.
func ensureMeiliIndex(ctx context.Context, baseURL, apiKey string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/indexes/listings", nil)
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil // index already exists
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check index: unexpected status %d", resp.StatusCode)
	}

	// Create index and wait for it to become available.
	if err := meiliReq(ctx, "POST", baseURL, "/indexes", apiKey, map[string]string{
		"uid":        "listings",
		"primaryKey": "id",
	}); err != nil {
		return fmt.Errorf("create index: %w", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/indexes/listings", nil)
		if err != nil {
			return fmt.Errorf("wait for index: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("wait for index: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("meilisearch listings index not ready after 10s")
}

func meiliReq(ctx context.Context, method, baseURL, path, apiKey string, body any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respData, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respData))
	}
	return nil
}
