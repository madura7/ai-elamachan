package search

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/madura7/ai-elamachan/backend/internal/listings"
)

// indexSettings is the authoritative Meilisearch settings for the listings index.
// Applied idempotently at startup and by the backfill command.
var indexSettings = &meilisearch.Settings{
	SortableAttributes: []string{"has_image", "created_at"},
	RankingRules: []string{
		"words",
		"typo",
		"proximity",
		"attribute",
		"sort",
		"exactness",
		"has_image:desc",
		"created_at:desc",
	},
}

// EnsureIndex applies index settings idempotently. Calling from multiple service
// instances simultaneously is safe; Meilisearch applies settings atomically.
// A nil receiver is a no-op (search disabled).
func (s *Service) EnsureIndex(_ context.Context) error {
	if s == nil {
		return nil
	}
	// Create index if it does not exist yet; ignore error (may already exist).
	if _, err := s.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        indexUID,
		PrimaryKey: "id",
	}); err != nil {
		log.Printf("search: EnsureIndex create (may already exist): %v", err)
	}
	if _, err := s.index.UpdateSettings(indexSettings); err != nil {
		return fmt.Errorf("search: EnsureIndex settings: %w", err)
	}
	return nil
}

// IndexListing upserts a listing document in Meilisearch. The call is
// fire-and-forget at the listing-service layer: failures are returned here but
// callers should log and continue rather than surfacing them to the user.
// A nil receiver is a no-op.
func (s *Service) IndexListing(_ context.Context, doc listings.IndexableDoc) error {
	if s == nil {
		return nil
	}
	meiliDoc := toDocument(doc)
	if _, err := s.index.AddDocuments([]Document{meiliDoc}, "id"); err != nil {
		return fmt.Errorf("search: IndexListing %s: %w", doc.ID, err)
	}
	return nil
}

// RemoveListing deletes a listing document from Meilisearch. Same best-effort
// contract as IndexListing. A nil receiver is a no-op.
func (s *Service) RemoveListing(_ context.Context, id string) error {
	if s == nil {
		return nil
	}
	if _, err := s.index.DeleteDocument(id); err != nil {
		return fmt.Errorf("search: RemoveListing %s: %w", id, err)
	}
	return nil
}

// BatchIndexListings upserts a slice of documents in a single Meilisearch
// request. Used by the backfill command. A nil receiver is a no-op.
func (s *Service) BatchIndexListings(_ context.Context, docs []listings.IndexableDoc) error {
	if s == nil || len(docs) == 0 {
		return nil
	}
	meiliDocs := make([]Document, len(docs))
	for i, d := range docs {
		meiliDocs[i] = toDocument(d)
	}
	if _, err := s.index.AddDocuments(meiliDocs, "id"); err != nil {
		return fmt.Errorf("search: BatchIndexListings: %w", err)
	}
	return nil
}

// toDocument maps a listings.IndexableDoc to the Meilisearch Document shape.
// The title is stored in the per-language field matching ContentLanguage.
func toDocument(doc listings.IndexableDoc) Document {
	d := Document{
		ID:              doc.ID,
		Category:        doc.Category,
		ContentLanguage: doc.ContentLanguage,
		HasImage:        doc.HasImage,
		ThumbnailURL:    doc.ThumbnailURL,
		PriceLKR:        doc.PriceLKR,
		CreatedAt:       doc.CreatedAt.UTC().Format(time.RFC3339),
		CreatedAtTS:     doc.CreatedAt.Unix(),
	}
	switch doc.ContentLanguage {
	case "en":
		d.TitleEN = doc.Title
	case "si":
		d.TitleSI = doc.Title
	case "ta":
		d.TitleTA = doc.Title
	default:
		d.Title = doc.Title // seed-compat fallback
	}
	return d
}
