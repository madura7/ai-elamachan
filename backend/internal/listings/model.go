package listings

import "time"

// Listing is the full response for GET/POST/PUT /listings/{id}.
type Listing struct {
	ID                string    `json:"id"`
	OwnerID           string    `json:"-"`
	Category          string    `json:"category"`
	ContentLanguage   string    `json:"content_language"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	PriceLKR          *int64    `json:"price_lkr"`
	TranslationSource *string   `json:"translation_source"` // null = original language returned
	Images            []Image   `json:"images"`
	CreatedAt         time.Time `json:"created_at"`
}

// Image is a single listing image.
type Image struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
}

// Summary is the slim card used in paginated list responses.
type Summary struct {
	ID           string    `json:"id"`
	Category     string    `json:"category"`
	Title        string    `json:"title"`
	PriceLKR     *int64    `json:"price_lkr"`
	ThumbnailURL *string   `json:"thumbnail_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// Page is the paginated list response.
type Page struct {
	Items    []Summary `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int       `json:"total"`
}

// CreateRequest is the decoded body for POST /listings.
type CreateRequest struct {
	Category        string `json:"category"`
	ContentLanguage string `json:"content_language"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	PriceLKR        *int64 `json:"price_lkr"`
}

// UpdateRequest is the decoded body for PUT /listings/{id}.
// All mutable fields are replaced; content_language is immutable post-create.
type UpdateRequest struct {
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	PriceLKR    *int64 `json:"price_lkr"`
}
