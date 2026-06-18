package search

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/madura7/ai-elamachan/backend/internal/listings"
)

// ─── Unit tests: toDocument mapping ──────────────────────────────────────────

func TestToDocument_EnglishTitle(t *testing.T) {
	lkr := int64(4500)
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	url := "https://cdn.example.com/img.jpg"

	doc := toDocument(listings.IndexableDoc{
		ID:              "abc-123",
		Category:        "electronics",
		ContentLanguage: "en",
		Title:           "iPhone 14 Pro",
		HasImage:        true,
		ThumbnailURL:    &url,
		PriceLKR:        &lkr,
		CreatedAt:       createdAt,
	})

	if doc.ID != "abc-123" {
		t.Errorf("ID = %q, want abc-123", doc.ID)
	}
	if doc.TitleEN != "iPhone 14 Pro" {
		t.Errorf("TitleEN = %q, want iPhone 14 Pro", doc.TitleEN)
	}
	if doc.TitleSI != "" || doc.TitleTA != "" {
		t.Errorf("non-EN title fields should be empty, got si=%q ta=%q", doc.TitleSI, doc.TitleTA)
	}
	if !doc.HasImage {
		t.Error("HasImage should be true")
	}
	if doc.ThumbnailURL == nil || *doc.ThumbnailURL != url {
		t.Errorf("ThumbnailURL = %v, want %s", doc.ThumbnailURL, url)
	}
	if doc.PriceLKR == nil || *doc.PriceLKR != 4500 {
		t.Errorf("PriceLKR = %v, want 4500", doc.PriceLKR)
	}
	if doc.CreatedAt != createdAt.UTC().Format(time.RFC3339) {
		t.Errorf("CreatedAt = %q, want RFC3339", doc.CreatedAt)
	}
	if doc.CreatedAtTS != createdAt.Unix() {
		t.Errorf("CreatedAtTS = %d, want %d", doc.CreatedAtTS, createdAt.Unix())
	}
	if doc.Category != "electronics" {
		t.Errorf("Category = %q, want electronics", doc.Category)
	}
}

func TestToDocument_SinhalaTitle(t *testing.T) {
	doc := toDocument(listings.IndexableDoc{
		ID:              "abc-456",
		ContentLanguage: "si",
		Title:           "ටොයොටා කාර්",
	})
	if doc.TitleSI != "ටොයොටා කාර්" {
		t.Errorf("TitleSI = %q, want Sinhala title", doc.TitleSI)
	}
	if doc.TitleEN != "" || doc.TitleTA != "" {
		t.Errorf("non-SI fields should be empty")
	}
}

func TestToDocument_TamilTitle(t *testing.T) {
	doc := toDocument(listings.IndexableDoc{
		ID:              "abc-789",
		ContentLanguage: "ta",
		Title:           "மொபைல் போன்",
	})
	if doc.TitleTA != "மொபைல் போன்" {
		t.Errorf("TitleTA = %q, want Tamil title", doc.TitleTA)
	}
}

func TestToDocument_PhotolessListing(t *testing.T) {
	doc := toDocument(listings.IndexableDoc{
		ID:              "no-photo",
		ContentLanguage: "en",
		Title:           "Old couch",
		HasImage:        false,
	})
	if doc.HasImage {
		t.Error("HasImage should be false")
	}
	if doc.ThumbnailURL != nil {
		t.Errorf("ThumbnailURL should be nil, got %v", doc.ThumbnailURL)
	}
}

func TestToDocument_NilPrice(t *testing.T) {
	doc := toDocument(listings.IndexableDoc{
		ID:              "no-price",
		ContentLanguage: "en",
		PriceLKR:        nil,
	})
	if doc.PriceLKR != nil {
		t.Errorf("PriceLKR should be nil, got %v", doc.PriceLKR)
	}
}

// ─── Integration tests: require live Meilisearch ─────────────────────────────
//
// Skipped automatically when MEILI_URL is not set.

func meiliSvcForTest(t *testing.T) *Service {
	t.Helper()
	if os.Getenv("MEILI_URL") == "" {
		t.Skip("MEILI_URL not set — skipping Meilisearch integration test")
	}
	svc, err := NewFromEnv()
	if err != nil || svc == nil {
		t.Skipf("search.NewFromEnv: err=%v svc=%v — skipping", err, svc)
	}
	return svc
}

// uniqueTestID returns an ID prefix guaranteed unique within a test run.
func uniqueTestID(base string) string {
	return fmt.Sprintf("test-%s-%d", base, time.Now().UnixNano())
}

// TestRankingPhotographedAbovePhotoless verifies that a photographed listing
// outranks a photoless listing at equal text relevance (AC5 ranking test).
func TestRankingPhotographedAbovePhotoless(t *testing.T) {
	svc := meiliSvcForTest(t)
	ctx := context.Background()

	if err := svc.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	base := uniqueTestID("rank")
	// Photoless is newer to ensure has_image is the differentiating factor, not created_at.
	withPhoto := listings.IndexableDoc{
		ID:              base + "-photo",
		ContentLanguage: "en",
		Title:           "ranking test widget with photo",
		HasImage:        true,
		CreatedAt:       time.Now().Add(-2 * time.Hour),
	}
	withoutPhoto := listings.IndexableDoc{
		ID:              base + "-nophoto",
		ContentLanguage: "en",
		Title:           "ranking test widget without photo",
		HasImage:        false,
		CreatedAt:       time.Now(),
	}

	if err := svc.BatchIndexListings(ctx, []listings.IndexableDoc{withPhoto, withoutPhoto}); err != nil {
		t.Fatalf("BatchIndexListings: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	t.Cleanup(func() {
		svc.RemoveListing(ctx, withPhoto.ID)    //nolint:errcheck
		svc.RemoveListing(ctx, withoutPhoto.ID) //nolint:errcheck
	})

	result, err := svc.Search(ctx, Params{
		Query:    "ranking test widget",
		Lang:     "en",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	photoPos, nophotoPos := -1, -1
	for i, item := range result.Items {
		switch item.ID {
		case withPhoto.ID:
			photoPos = i
		case withoutPhoto.ID:
			nophotoPos = i
		}
	}
	if photoPos == -1 || nophotoPos == -1 {
		t.Fatalf("test docs missing in results (photoPos=%d nophotoPos=%d)", photoPos, nophotoPos)
	}
	if photoPos >= nophotoPos {
		t.Errorf("photographed listing (pos %d) should rank above photoless (pos %d)",
			photoPos, nophotoPos)
	}
}

// TestBrowseParityWithListings verifies that empty-query search returns items
// in has_image DESC, created_at DESC order — matching GET /listings (AC4 + AC5).
func TestBrowseParityWithListings(t *testing.T) {
	svc := meiliSvcForTest(t)
	ctx := context.Background()

	if err := svc.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	base := uniqueTestID("parity")
	now := time.Now().UTC()
	// Expected browse order: photo-new → photo-old → photoless-new → (photoless older items)
	photoNew := listings.IndexableDoc{
		ID: base + "-photo-new", ContentLanguage: "en",
		Title: "parity test photo new", HasImage: true, CreatedAt: now,
	}
	photoOld := listings.IndexableDoc{
		ID: base + "-photo-old", ContentLanguage: "en",
		Title: "parity test photo old", HasImage: true, CreatedAt: now.Add(-24 * time.Hour),
	}
	photolessNew := listings.IndexableDoc{
		ID: base + "-noPhoto-new", ContentLanguage: "en",
		Title: "parity test no photo new", HasImage: false, CreatedAt: now.Add(-1 * time.Hour),
	}

	if err := svc.BatchIndexListings(ctx, []listings.IndexableDoc{photoNew, photoOld, photolessNew}); err != nil {
		t.Fatalf("BatchIndexListings: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	t.Cleanup(func() {
		svc.RemoveListing(ctx, photoNew.ID)      //nolint:errcheck
		svc.RemoveListing(ctx, photoOld.ID)      //nolint:errcheck
		svc.RemoveListing(ctx, photolessNew.ID)  //nolint:errcheck
	})

	result, err := svc.Search(ctx, Params{Query: "", Lang: "en", Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("browse Search: %v", err)
	}

	pos := map[string]int{}
	for i, item := range result.Items {
		pos[item.ID] = i
	}
	for _, id := range []string{photoNew.ID, photoOld.ID, photolessNew.ID} {
		if _, ok := pos[id]; !ok {
			t.Fatalf("doc %s missing from browse results", id)
		}
	}
	if pos[photoNew.ID] >= pos[photolessNew.ID] {
		t.Errorf("photo-new (pos %d) should precede photoless (pos %d)",
			pos[photoNew.ID], pos[photolessNew.ID])
	}
	if pos[photoOld.ID] >= pos[photolessNew.ID] {
		t.Errorf("photo-old (pos %d) should precede photoless (pos %d)",
			pos[photoOld.ID], pos[photolessNew.ID])
	}
	if pos[photoNew.ID] >= pos[photoOld.ID] {
		t.Errorf("photo-new (pos %d) should precede photo-old (pos %d)",
			pos[photoNew.ID], pos[photoOld.ID])
	}
}

// TestKeywordRelevanceUnchanged verifies that text relevance takes priority
// over has_image / created_at ranking (regression guard — AC5).
func TestKeywordRelevanceUnchanged(t *testing.T) {
	svc := meiliSvcForTest(t)
	ctx := context.Background()

	if err := svc.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	base := uniqueTestID("relev")
	// Exact match: older, no photo.  Fuzzy match: newer, has photo.
	// Text relevance must win over has_image + recency.
	exact := listings.IndexableDoc{
		ID:              base + "-exact",
		ContentLanguage: "en",
		Title:           "blue widget exact",
		HasImage:        false,
		CreatedAt:       time.Now().Add(-48 * time.Hour),
	}
	fuzzy := listings.IndexableDoc{
		ID:              base + "-fuzzy",
		ContentLanguage: "en",
		Title:           "some other product mentioning widget tangentially",
		HasImage:        true,
		CreatedAt:       time.Now(),
	}

	if err := svc.BatchIndexListings(ctx, []listings.IndexableDoc{exact, fuzzy}); err != nil {
		t.Fatalf("BatchIndexListings: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	t.Cleanup(func() {
		svc.RemoveListing(ctx, exact.ID) //nolint:errcheck
		svc.RemoveListing(ctx, fuzzy.ID) //nolint:errcheck
	})

	result, err := svc.Search(ctx, Params{
		Query: "blue widget exact", Lang: "en", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Items) == 0 {
		t.Fatal("no results")
	}
	if result.Items[0].ID != exact.ID {
		t.Errorf("first result = %q, want exact match %q", result.Items[0].ID, exact.ID)
	}
}

// TestIndexLifecycle verifies create→upsert, confirm/update→upsert, delete→remove.
func TestIndexLifecycle(t *testing.T) {
	svc := meiliSvcForTest(t)
	ctx := context.Background()

	if err := svc.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	id := uniqueTestID("lifecycle")

	// 1. Create — no image
	if err := svc.IndexListing(ctx, listings.IndexableDoc{
		ID:              id,
		ContentLanguage: "en",
		Title:           "lifecycle test sofa",
		HasImage:        false,
		CreatedAt:       time.Now(),
	}); err != nil {
		t.Fatalf("IndexListing (create): %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	found := func(q string) (Summary, bool) {
		res, err := svc.Search(ctx, Params{Query: q, Lang: "en", Page: 1, PageSize: 5})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		for _, item := range res.Items {
			if item.ID == id {
				return item, true
			}
		}
		return Summary{}, false
	}

	item, ok := found("lifecycle test sofa")
	if !ok {
		t.Fatal("listing not found after create")
	}
	if item.HasImage {
		t.Error("after create: HasImage should be false")
	}

	// 2. Confirm image — upsert with has_image=true
	thumb := "https://cdn.example.com/sofa.jpg"
	if err := svc.IndexListing(ctx, listings.IndexableDoc{
		ID: id, ContentLanguage: "en", Title: "lifecycle test sofa",
		HasImage: true, ThumbnailURL: &thumb, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("IndexListing (confirm): %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	item, ok = found("lifecycle test sofa")
	if !ok {
		t.Fatal("listing not found after confirm")
	}
	if !item.HasImage {
		t.Error("after confirm: HasImage should be true")
	}
	if item.ThumbnailURL == nil || *item.ThumbnailURL != thumb {
		t.Errorf("after confirm: ThumbnailURL = %v, want %s", item.ThumbnailURL, thumb)
	}

	// 3. Delete — remove from index
	if err := svc.RemoveListing(ctx, id); err != nil {
		t.Fatalf("RemoveListing: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if _, ok := found("lifecycle test sofa"); ok {
		t.Error("listing still in index after delete")
	}
}
