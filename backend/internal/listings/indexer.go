package listings

import "context"

// SearchDoc holds the fields needed to index a listing translation.
type SearchDoc struct {
	ListingID     string
	Lang          string
	Category      string
	Title         string
	Description   string
	PriceLKR      *int64
	ThumbnailURL  *string
	CreatedAtUnix int64
}

// SearchIndexer is the interface the listings handler uses to keep the
// search index in sync on create, update, and delete.
// A nil SearchIndexer means search is disabled.
type SearchIndexer interface {
	IndexListing(ctx context.Context, doc SearchDoc) error
	RemoveListing(ctx context.Context, listingID string) error
}
