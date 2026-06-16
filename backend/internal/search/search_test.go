package search

import (
	"testing"
	"time"
)

func TestDocumentTitleFor_PerLanguage(t *testing.T) {
	doc := Document{
		ContentLanguage: "si",
		TitleEN:         "Toyota car",
		TitleSI:         "ටොයොටා කාර්",
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

func TestDocumentTitleFor_SeedFallback(t *testing.T) {
	// Seed documents have a flat Title field, no per-language fields.
	doc := Document{Title: "iPhone 14 Pro"}
	if got := doc.titleFor("en"); got != "iPhone 14 Pro" {
		t.Errorf("seed titleFor = %q, want iPhone 14 Pro", got)
	}
	if got := doc.titleFor(""); got != "iPhone 14 Pro" {
		t.Errorf("seed titleFor empty lang = %q, want iPhone 14 Pro", got)
	}
}

func TestDocumentTitleForLastResort(t *testing.T) {
	doc := Document{ContentLanguage: "si", TitleEN: "only english"}
	if got := doc.titleFor("ta"); got != "only english" {
		t.Errorf("titleFor last-resort = %q, want %q", got, "only english")
	}
	if got := (Document{}).titleFor("en"); got != "" {
		t.Errorf("empty doc titleFor = %q, want empty", got)
	}
}

func TestDocumentCategorySlug(t *testing.T) {
	// Service-indexed document uses Category field.
	d1 := Document{Category: "electronics", CategorySlug: ""}
	if got := d1.categorySlug(); got != "electronics" {
		t.Errorf("service doc categorySlug = %q, want electronics", got)
	}
	// Seed document uses CategorySlug field.
	d2 := Document{Category: "", CategorySlug: "mobile-phones"}
	if got := d2.categorySlug(); got != "mobile-phones" {
		t.Errorf("seed doc categorySlug = %q, want mobile-phones", got)
	}
}

func TestDocumentPriceLKR(t *testing.T) {
	// Service-indexed: already in LKR.
	lkr := int64(4500)
	d1 := Document{PriceLKR: &lkr}
	if got := d1.priceLKR(); got == nil || *got != 4500 {
		t.Errorf("priceLKR (service) = %v, want 4500", got)
	}
	// Seed: stored in cents.
	cents := int64(450000)
	d2 := Document{PriceCents: &cents}
	if got := d2.priceLKR(); got == nil || *got != 4500 {
		t.Errorf("priceLKR (seed/cents) = %v, want 4500", got)
	}
	// Nil price.
	if got := (Document{}).priceLKR(); got != nil {
		t.Errorf("priceLKR nil = %v, want nil", got)
	}
}

func TestDocumentCreatedAtTime(t *testing.T) {
	want := time.Date(2026, 6, 14, 10, 30, 0, 0, time.UTC)

	d := Document{CreatedAt: want.Format(time.RFC3339), CreatedAtTS: want.Unix()}
	if got := d.createdAtTime(); !got.Equal(want) {
		t.Errorf("createdAtTime() = %v, want %v", got, want)
	}

	d2 := Document{CreatedAt: "not-a-date", CreatedAtTS: want.Unix()}
	if got := d2.createdAtTime(); !got.Equal(want) {
		t.Errorf("createdAtTime() fallback = %v, want %v", got, want)
	}
}

func TestCategoryFacets(t *testing.T) {
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
