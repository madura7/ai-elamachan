package search

import "time"

// Document is the denormalized listing record stored in Meilisearch.
//
// Title/description are split into one field per MVP language so charabia
// (Meilisearch's tokenizer) indexes each script natively — the VER-41 spike
// confirmed stock tokenization handles Sinhala/Tamil prefix + typo-tolerance
// without a custom analyzer. Empty per-lang fields (no translation cached yet)
// are omitted so they never pollute relevance.
type Document struct {
	ID              string `json:"id"`
	Category        string `json:"category"`         // filterable + facet
	ContentLanguage string `json:"content_language"` // filterable
	PriceLKR        *int64 `json:"price_lkr"`
	ThumbnailURL    *string `json:"thumbnail_url"`
	CreatedAt       string `json:"created_at"`    // RFC3339 (UTC) for display
	CreatedAtTS     int64  `json:"created_at_ts"` // unix seconds, sortable

	TitleEN string `json:"title_en,omitempty"`
	TitleSI string `json:"title_si,omitempty"`
	TitleTA string `json:"title_ta,omitempty"`
	DescEN  string `json:"description_en,omitempty"`
	DescSI  string `json:"description_si,omitempty"`
	DescTA  string `json:"description_ta,omitempty"`
}

// titleFor returns the title in lang, falling back to the content_language
// title (and then any non-empty title) so a hit always has a usable label.
func (d Document) titleFor(lang string) string {
	byLang := map[string]string{"en": d.TitleEN, "si": d.TitleSI, "ta": d.TitleTA}
	if t := byLang[lang]; t != "" {
		return t
	}
	if t := byLang[d.ContentLanguage]; t != "" {
		return t
	}
	for _, t := range []string{d.TitleEN, d.TitleSI, d.TitleTA} {
		if t != "" {
			return t
		}
	}
	return ""
}

// createdAtTime parses the stored RFC3339 timestamp; on a parse failure it
// falls back to the unix field so the response always carries a valid time.
func (d Document) createdAtTime() time.Time {
	if t, err := time.Parse(time.RFC3339, d.CreatedAt); err == nil {
		return t
	}
	return time.Unix(d.CreatedAtTS, 0).UTC()
}

// Summary is the slim search hit (mirrors listings.Summary so the frontend can
// reuse one card component for browse + search).
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
	Lang     string // "" | en | si | ta — controls which title is displayed
	Category string // "" or a CategorySlug — filters results
	Page     int
	PageSize int
}
