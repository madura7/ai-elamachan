package search

import "time"

// Document is the denormalized listing record stored in Meilisearch.
//
// The struct covers two document shapes:
//  1. Seed documents (from cmd/seed): use title/description/category_slug/price_cents.
//  2. Future service-indexed documents: per-language title_*/description_* fields.
//
// The helper methods (titleFor, category, priceLKR) handle both shapes
// transparently so the search handler never needs to distinguish them.
type Document struct {
	ID              string  `json:"id"`
	Category        string  `json:"category,omitempty"`         // service-indexed
	ContentLanguage string  `json:"content_language,omitempty"` // service-indexed
	PriceLKR        *int64  `json:"price_lkr,omitempty"`        // service-indexed
	ThumbnailURL    *string `json:"thumbnail_url,omitempty"`
	CreatedAt       string  `json:"created_at,omitempty"`    // RFC3339 (UTC)
	CreatedAtTS     int64   `json:"created_at_ts,omitempty"` // unix seconds, sortable

	// Per-language fields (service-indexed)
	TitleEN string `json:"title_en,omitempty"`
	TitleSI string `json:"title_si,omitempty"`
	TitleTA string `json:"title_ta,omitempty"`
	DescEN  string `json:"description_en,omitempty"`
	DescSI  string `json:"description_si,omitempty"`
	DescTA  string `json:"description_ta,omitempty"`

	// Seed-compatible flat fields
	Title        string `json:"title,omitempty"`
	CategorySlug string `json:"category_slug,omitempty"`
	PriceCents   *int64 `json:"price_cents,omitempty"`
	Lang         string `json:"lang,omitempty"`
}

// titleFor returns the best title for lang, falling back through available
// languages and finally to the flat Title field (seed documents).
func (d Document) titleFor(lang string) string {
	byLang := map[string]string{"en": d.TitleEN, "si": d.TitleSI, "ta": d.TitleTA}
	if lang != "" {
		if t := byLang[lang]; t != "" {
			return t
		}
	}
	if t := byLang[d.ContentLanguage]; t != "" {
		return t
	}
	for _, t := range []string{d.TitleEN, d.TitleSI, d.TitleTA} {
		if t != "" {
			return t
		}
	}
	return d.Title // seed fallback
}

// categorySlug returns the category slug from whichever field is populated.
func (d Document) categorySlug() string {
	if d.Category != "" {
		return d.Category
	}
	return d.CategorySlug
}

// priceLKR returns the price in LKR, converting from cents if needed.
func (d Document) priceLKR() *int64 {
	if d.PriceLKR != nil {
		return d.PriceLKR
	}
	return centsToLKR(d.PriceCents)
}

// createdAtTime parses the stored RFC3339 timestamp; on parse failure it falls
// back to the unix field.
func (d Document) createdAtTime() time.Time {
	if t, err := time.Parse(time.RFC3339, d.CreatedAt); err == nil {
		return t
	}
	return time.Unix(d.CreatedAtTS, 0).UTC()
}

// Summary is the slim search hit returned to callers.
type Summary struct {
	ID           string    `json:"id"`
	Category     string    `json:"category"`
	Title        string    `json:"title"`
	PriceLKR     *int64    `json:"price_lkr"`
	ThumbnailURL *string   `json:"thumbnail_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// Result is the search response: a page of hits plus category facet counts.
type Result struct {
	Items    []Summary      `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int            `json:"total"`
	Facets   map[string]int `json:"facets"` // category slug → match count
}

// Params is the validated input to a search.
type Params struct {
	Query    string
	Lang     string // "" | en | si | ta
	Category string // "" or a slug — filters results
	Page     int
	PageSize int
}

// categoryFacets pulls the {slug: count} map for the "category" facet out of
// Meilisearch's loosely-typed facetDistribution payload.
func categoryFacets(dist interface{}) map[string]int {
	out := map[string]int{}
	top, ok := dist.(map[string]interface{})
	if !ok {
		return out
	}
	cat, ok := top["category"].(map[string]interface{})
	if !ok {
		return out
	}
	for slug, count := range cat {
		if n, ok := count.(float64); ok {
			out[slug] = int(n)
		}
	}
	return out
}

// centsToLKR converts the price_cents storage unit back to whole LKR.
func centsToLKR(cents *int64) *int64 {
	if cents == nil {
		return nil
	}
	v := *cents / 100
	return &v
}
