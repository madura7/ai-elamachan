// Package listings provides shared types and validation maps for listing and
// category domain objects. Other packages (search, handler) import from here
// to avoid circular dependencies.
package listings

import "time"

// ValidLangs is the closed set of supported content/UI languages (ADR 0001).
var ValidLangs = map[string]bool{
	"en": true,
	"si": true,
	"ta": true,
}

// ValidCategories is the category slug allowlist used for query-param validation.
// Includes both the OpenAPI enum slugs and the seed taxonomy slugs so that
// ?category= filters work with both the current live seed data and any future
// migration-seeded taxonomy.
var ValidCategories = map[string]bool{
	// OpenAPI CategorySlug enum
	"electronics":  true,
	"vehicles":     true,
	"property":     true,
	"home_garden":  true,
	"fashion":      true,
	"mobile_phones": true,
	"services":     true,
	"jobs":         true,
	"pets":         true,
	"other":        true,
	// Seed taxonomy slugs (cmd/seed)
	"mobile-phones": true,
	"cars":          true,
	"furniture":     true,
}

// Summary is the slim listing representation returned by GET /api/v1/listings.
// Matches the OpenAPI ListingSummary schema.
type Summary struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	PriceLKR  *int64    `json:"price_lkr"`
	CreatedAt time.Time `json:"created_at"`
}

// Detail is the full listing representation returned by GET /api/v1/listings/{id}.
// Matches the OpenAPI Listing schema.
type Detail struct {
	ID                string    `json:"id"`
	Category          string    `json:"category"`
	ContentLanguage   string    `json:"content_language"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	PriceLKR          *int64    `json:"price_lkr"`
	TranslationSource *string   `json:"translation_source,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// Page is the paginated response for GET /api/v1/listings.
// Matches the OpenAPI ListingPage schema.
type Page struct {
	Items    []Summary `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int       `json:"total"`
}

// Category is a taxonomy entry with its name resolved to the requested language.
// Matches the OpenAPI Category schema.
type Category struct {
	Slug       string  `json:"slug"`
	Name       string  `json:"name"`
	ParentSlug *string `json:"parent_slug"`
	SortOrder  int     `json:"sort_order"`
}
