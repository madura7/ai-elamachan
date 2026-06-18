package listings

import (
	"context"
	"time"
)

// ListingIndexer is the interface implemented by the search package and injected
// into Handler via SetIndexer. All methods are best-effort; callers must not
// fail the listing operation on index errors.
type ListingIndexer interface {
	IndexListing(ctx context.Context, doc IndexableDoc) error
	RemoveListing(ctx context.Context, id string) error
}

// IndexableDoc holds the fields the search indexer needs from a listing.
// Only the authored-language title is provided (the indexer stores it in the
// appropriate per-language field based on ContentLanguage).
type IndexableDoc struct {
	ID              string
	Category        string
	ContentLanguage string
	Title           string
	HasImage        bool
	ThumbnailURL    *string
	PriceLKR        *int64
	CreatedAt       time.Time
}
