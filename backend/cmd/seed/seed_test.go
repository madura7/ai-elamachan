package main

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestSeedPostgresIdempotent runs seedPostgres twice against a real database
// and asserts that row counts are identical after both runs.
// Set TEST_DATABASE_URL to enable; skipped otherwise.
func TestSeedPostgresIdempotent(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// First seed
	if err := seedPostgres(ctx, db); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	before := countSeedRows(t, ctx, db)

	// Second seed — must be a no-op
	if err := seedPostgres(ctx, db); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	after := countSeedRows(t, ctx, db)

	if before != after {
		t.Errorf("seed is not idempotent: counts changed %+v → %+v", before, after)
	}
}

type seedCounts struct {
	users       int
	categories  int
	catTrans    int
	listings    int
	listingTrans int
}

func countSeedRows(t *testing.T, ctx context.Context, db *sql.DB) seedCounts {
	t.Helper()
	var c seedCounts

	row := db.QueryRowContext(ctx,
		`SELECT count(*) FROM users WHERE phone_e164 = $1`, seedUserPhone)
	if err := row.Scan(&c.users); err != nil {
		t.Fatalf("count users: %v", err)
	}

	slugs := make([]string, len(allCategories))
	for i, cat := range allCategories {
		slugs[i] = cat.slug
	}
	row = db.QueryRowContext(ctx,
		`SELECT count(*) FROM categories WHERE slug = ANY($1::text[])`,
		"{"+joinStrings(slugs, ",")+"}",
	)
	if err := row.Scan(&c.categories); err != nil {
		t.Fatalf("count categories: %v", err)
	}

	row = db.QueryRowContext(ctx,
		`SELECT count(*) FROM category_translations ct
		 JOIN categories c ON c.id = ct.category_id
		 WHERE c.slug = ANY($1::text[])`,
		"{"+joinStrings(slugs, ",")+"}",
	)
	if err := row.Scan(&c.catTrans); err != nil {
		t.Fatalf("count category_translations: %v", err)
	}

	ids := make([]string, len(allListings))
	for i, l := range allListings {
		ids[i] = l.id
	}
	row = db.QueryRowContext(ctx,
		`SELECT count(*) FROM listings WHERE id = ANY($1::uuid[])`,
		"{"+joinStrings(ids, ",")+"}",
	)
	if err := row.Scan(&c.listings); err != nil {
		t.Fatalf("count listings: %v", err)
	}

	row = db.QueryRowContext(ctx,
		`SELECT count(*) FROM listing_translations WHERE listing_id = ANY($1::uuid[])`,
		"{"+joinStrings(ids, ",")+"}",
	)
	if err := row.Scan(&c.listingTrans); err != nil {
		t.Fatalf("count listing_translations: %v", err)
	}

	return c
}

// joinStrings joins a slice with sep (no external dependency).
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += sep + s
	}
	return out
}

// TestBuildMeiliDocs verifies document shape without any HTTP calls.
func TestBuildMeiliDocs(t *testing.T) {
	docs := buildMeiliDocs()
	if len(docs) != len(allListings) {
		t.Fatalf("expected %d docs, got %d", len(allListings), len(docs))
	}
	requiredKeys := []string{"id", "title", "description", "category_slug", "price_cents", "currency", "status", "lang"}
	for i, d := range docs {
		for _, k := range requiredKeys {
			if _, ok := d[k]; !ok {
				t.Errorf("doc[%d] missing key %q", i, k)
			}
		}
		if d["currency"] != "LKR" {
			t.Errorf("doc[%d] currency = %q, want LKR", i, d["currency"])
		}
		if d["status"] != "active" {
			t.Errorf("doc[%d] status = %q, want active", i, d["status"])
		}
	}
}
