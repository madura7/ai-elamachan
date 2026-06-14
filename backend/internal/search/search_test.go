package search

import (
	"testing"
	"time"
)

func TestDocumentTitleFor(t *testing.T) {
	doc := Document{
		ContentLanguage: "si",
		TitleEN:         "Toyota car",
		TitleSI:         "ටොයොටා කාර්",
		// TitleTA intentionally empty (no Tamil translation cached yet).
	}

	tests := []struct {
		name string
		lang string
		want string
	}{
		{"requested lang present", "en", "Toyota car"},
		{"requested lang is content lang", "si", "ටොයොටා කාර්"},
		{"requested lang missing → content lang fallback", "ta", "ටොයොටා කාර්"},
		{"no lang requested → content lang", "", "ටොයොටා කාර්"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doc.titleFor(tt.lang); got != tt.want {
				t.Errorf("titleFor(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestDocumentTitleForLastResort(t *testing.T) {
	// content_language title is somehow absent; fall back to any non-empty title.
	doc := Document{ContentLanguage: "si", TitleEN: "only english"}
	if got := doc.titleFor("ta"); got != "only english" {
		t.Errorf("titleFor last-resort = %q, want %q", got, "only english")
	}
	if got := (Document{}).titleFor("en"); got != "" {
		t.Errorf("empty doc titleFor = %q, want empty", got)
	}
}

func TestDocumentCreatedAtTime(t *testing.T) {
	want := time.Date(2026, 6, 14, 10, 30, 0, 0, time.UTC)

	// Primary path: RFC3339 parses.
	d := Document{CreatedAt: want.Format(time.RFC3339), CreatedAtTS: want.Unix()}
	if got := d.createdAtTime(); !got.Equal(want) {
		t.Errorf("createdAtTime() = %v, want %v", got, want)
	}

	// Fallback path: unparseable RFC field → use unix ts.
	d2 := Document{CreatedAt: "not-a-date", CreatedAtTS: want.Unix()}
	if got := d2.createdAtTime(); !got.Equal(want) {
		t.Errorf("createdAtTime() fallback = %v, want %v", got, want)
	}
}

func TestCategoryFacets(t *testing.T) {
	// Shape mirrors Meilisearch's facetDistribution JSON (numbers decode as float64).
	dist := map[string]interface{}{
		"category": map[string]interface{}{
			"electronics": float64(3),
			"vehicles":    float64(1),
		},
	}
	got := categoryFacets(dist)
	if got["electronics"] != 3 || got["vehicles"] != 1 {
		t.Errorf("categoryFacets = %v, want electronics:3 vehicles:1", got)
	}

	// Robust to nil / wrong-shape input.
	if len(categoryFacets(nil)) != 0 {
		t.Errorf("categoryFacets(nil) should be empty")
	}
	if len(categoryFacets("garbage")) != 0 {
		t.Errorf("categoryFacets(non-map) should be empty")
	}
	if len(categoryFacets(map[string]interface{}{"other": 1})) != 0 {
		t.Errorf("categoryFacets without category key should be empty")
	}
}

func TestCentsToLKR(t *testing.T) {
	if got := centsToLKR(nil); got != nil {
		t.Errorf("centsToLKR(nil) = %v, want nil", got)
	}
	cents := int64(150000)
	if got := centsToLKR(&cents); got == nil || *got != 1500 {
		t.Errorf("centsToLKR(150000) = %v, want 1500", got)
	}
}
