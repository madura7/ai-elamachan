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

// ValidCategories is the closed category slug set. Single source of truth for
// server-side validation; must stay in sync with openapi.yaml CategorySlug enum.
var ValidCategories = map[string]bool{
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
