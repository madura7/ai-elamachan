package search

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestSearchIntegration drives the real Service against a live Meilisearch
// (MEILI_URL) and Postgres (DATABASE_URL). It reproduces the VER-41 spike
// scenarios end-to-end through the production code path: seed listings + their
// translations in the DB, index them via the Service, then assert prefix,
// typo-tolerance, Sinhala/Tamil, category-facet, and category-filter behaviour.
//
// Skipped unless both MEILI_URL and DATABASE_URL are set, so the default unit
// test run (and CI without these services) stays fast and hermetic.
func TestSearchIntegration(t *testing.T) {
	meiliURL := os.Getenv("MEILI_URL")
	dbURL := os.Getenv("DATABASE_URL")
	if meiliURL == "" || dbURL == "" {
		t.Skip("set MEILI_URL and DATABASE_URL to run the search integration test")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	svc, err := NewFromEnv(db, "http://localhost:8080/api/v1/images")
	if err != nil {
		t.Fatalf("NewFromEnv: %v", err)
	}
	if svc == nil {
		t.Fatal("expected a Service (MEILI_URL is set)")
	}
	if err := svc.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	// Start from a clean index for deterministic counts.
	if task, err := svc.index.DeleteAllDocuments(); err != nil {
		t.Fatalf("clear index: %v", err)
	} else if _, err := svc.client.WaitForTask(task.TaskUID); err != nil {
		t.Fatalf("wait clear: %v", err)
	}

	// Seed three trilingual listings modelled on the spike's sample ads.
	type seed struct {
		category string
		content  string
		tr       map[string]string // lang → title
	}
	seeds := []seed{
		{"vehicles", "si", map[string]string{
			"en": "Toyota car for sale",
			"si": "ටොයොටා කාර් විකිණීමට",
			"ta": "டொயோட்டா கார் விற்பனைக்கு",
		}},
		{"property", "si", map[string]string{
			"si": "කොළඹ නිවසක් කුලියට",
		}},
		{"mobile_phones", "ta", map[string]string{
			"ta": "சாம்சங் தொலைபேசி விற்பனை",
		}},
	}

	ids := make([]string, 0, len(seeds))
	for _, s := range seeds {
		id := seedListing(t, ctx, db, s.category, s.content, s.tr)
		ids = append(ids, id)
		if err := svc.IndexListing(ctx, id); err != nil {
			t.Fatalf("IndexListing %s: %v", id, err)
		}
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = db.Exec(ctx, `DELETE FROM listings WHERE id = $1`, id)
			_ = svc.DeleteListing(ctx, id)
		}
	})

	carID := ids[0]

	cases := []struct {
		name      string
		params    Params
		wantID    string // a hit that must be present ("" = expect zero hits)
		wantTotal int    // -1 = don't assert exact total
	}{
		{"english exact", Params{Query: "Toyota", Page: 1, PageSize: 20}, carID, 1},
		{"english prefix/as-you-type", Params{Query: "Toyo", Page: 1, PageSize: 20}, carID, 1},
		{"english typo-tolerance", Params{Query: "Toyata", Page: 1, PageSize: 20}, carID, 1},
		{"sinhala prefix", Params{Query: "ටොයො", Lang: "si", Page: 1, PageSize: 20}, carID, 1},
		{"tamil prefix", Params{Query: "டொயோ", Lang: "ta", Page: 1, PageSize: 20}, carID, 1},
		{"sinhala body term other listing", Params{Query: "කොළඹ", Lang: "si", Page: 1, PageSize: 20}, ids[1], 1},
		{"tamil other listing", Params{Query: "சாம்சங்", Lang: "ta", Page: 1, PageSize: 20}, ids[2], 1},
		{"category filter excludes", Params{Query: "Toyota", Category: "property", Page: 1, PageSize: 20}, "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := waitForSearch(t, ctx, svc, tc.params, tc.wantTotal)
			if tc.wantTotal >= 0 && res.Total != tc.wantTotal {
				t.Fatalf("Total = %d, want %d (items=%+v)", res.Total, tc.wantTotal, res.Items)
			}
			if tc.wantID == "" {
				if len(res.Items) != 0 {
					t.Fatalf("expected zero hits, got %+v", res.Items)
				}
				return
			}
			if !containsID(res.Items, tc.wantID) {
				t.Fatalf("expected hit %s, got %+v", tc.wantID, res.Items)
			}
			// Every hit must carry a non-empty localized title.
			for _, it := range res.Items {
				if it.Title == "" {
					t.Errorf("hit %s has empty title", it.ID)
				}
			}
		})
	}

	// Category facet: a Toyota search must report the vehicles facet count.
	t.Run("category facets", func(t *testing.T) {
		res := waitForSearch(t, ctx, svc, Params{Query: "Toyota", Page: 1, PageSize: 20}, 1)
		if res.Facets["vehicles"] != 1 {
			t.Fatalf("facets[vehicles] = %d, want 1 (facets=%v)", res.Facets["vehicles"], res.Facets)
		}
	})

	// Localized display: querying lang=ta returns the Tamil title for the car.
	t.Run("localized title display", func(t *testing.T) {
		res := waitForSearch(t, ctx, svc, Params{Query: "Toyota", Lang: "ta", Page: 1, PageSize: 20}, 1)
		var got string
		for _, it := range res.Items {
			if it.ID == carID {
				got = it.Title
			}
		}
		if got != "டொயோட்டா கார் விற்பனைக்கு" {
			t.Fatalf("ta title = %q, want Tamil title", got)
		}
	})
}

// seedListing inserts an active listing and its translations, returning the id.
func seedListing(t *testing.T, ctx context.Context, db *pgxpool.Pool, category, content string, tr map[string]string) string {
	t.Helper()
	const devUser = "00000000-0000-0000-0000-000000000001"
	var catID string
	if err := db.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, category).Scan(&catID); err != nil {
		t.Fatalf("resolve category %s: %v", category, err)
	}
	var id string
	if err := db.QueryRow(ctx, `
		INSERT INTO listings (user_id, category_id, content_language, status)
		VALUES ($1, $2, $3, 'active') RETURNING id
	`, devUser, catID, content).Scan(&id); err != nil {
		t.Fatalf("insert listing: %v", err)
	}
	for lang, title := range tr {
		if _, err := db.Exec(ctx, `
			INSERT INTO listing_translations (listing_id, lang, title, source)
			VALUES ($1, $2, $3, 'human')
		`, id, lang, title); err != nil {
			t.Fatalf("insert translation %s: %v", lang, err)
		}
	}
	return id
}

// waitForSearch polls Search until the result count matches wantTotal (when
// >= 0) or a short timeout elapses — Meilisearch indexing is async.
func waitForSearch(t *testing.T, ctx context.Context, svc *Service, p Params, wantTotal int) *Result {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var res *Result
	for {
		var err error
		res, err = svc.Search(ctx, p)
		if err != nil {
			t.Fatalf("Search(%q): %v", p.Query, err)
		}
		if wantTotal < 0 || res.Total == wantTotal {
			return res
		}
		if time.Now().After(deadline) {
			return res
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func containsID(items []Summary, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}
