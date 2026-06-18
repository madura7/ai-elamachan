// Package search wraps Meilisearch for ElaMachan listing search.
//
// The service supports both full-text queries and empty-query browse requests.
// Index settings (sortableAttributes + rankingRules) are applied idempotently
// at startup via EnsureIndex; the same helper is used by seed and backfill.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// indexUID is the single Meilisearch index backing listing search.
const indexUID = "listings"

// Service owns the Meilisearch client and exposes Search, EnsureIndex,
// IndexListing, RemoveListing, and BatchIndexListings.
// A nil *Service is the "search disabled" sentinel; all methods are nil-safe.
type Service struct {
	client *meilisearch.Client
	index  *meilisearch.Index
}

// NewFromEnv builds a Service from MEILI_URL / MEILI_MASTER_KEY.
// Returns (nil, nil) when MEILI_URL is unset — callers treat nil as "search
// disabled" and return 503 on search requests.
// Constructing the client does not open a connection; per-call errors surface
// lazily.
func NewFromEnv() (*Service, error) {
	host := os.Getenv("MEILI_URL")
	if host == "" {
		return nil, nil
	}
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:    host,
		APIKey:  os.Getenv("MEILI_MASTER_KEY"),
		Timeout: 5 * time.Second,
	})
	return &Service{
		client: client,
		index:  client.Index(indexUID),
	}, nil
}

// Search runs a query against the listings index and returns a page of results.
// When p.Query is empty (browse/catalog mode) an explicit sort is applied to
// produce has_image DESC, created_at DESC ordering — matching GET /listings.
//
// Handles both seed-format documents (flat title/category_slug) and
// service-indexed per-language documents via the Document helper methods.
// Uses Limit/Offset pagination for Meilisearch version compatibility.
func (s *Service) Search(_ context.Context, p Params) (*Result, error) {
	req := &meilisearch.SearchRequest{
		Limit:  int64(p.PageSize),
		Offset: int64((p.Page - 1) * p.PageSize),
		Facets: []string{"category_slug"},
	}
	if p.Category != "" {
		req.Filter = fmt.Sprintf(`category_slug = %q`, p.Category)
	}
	// For empty-query / browse requests, apply explicit sort so that ordering
	// is deterministic and matches GET /listings (has_image DESC, created_at DESC).
	if p.Query == "" {
		req.Sort = []string{"has_image:desc", "created_at:desc"}
	}

	resp, err := s.index.Search(p.Query, req)
	if err != nil {
		return nil, fmt.Errorf("search: query: %w", err)
	}

	items := make([]Summary, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		raw, err := json.Marshal(hit)
		if err != nil {
			return nil, fmt.Errorf("search: marshal hit: %w", err)
		}
		var doc Document
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("search: unmarshal hit: %w", err)
		}
		items = append(items, Summary{
			ID:           doc.ID,
			Category:     doc.categorySlug(),
			Title:        doc.titleFor(p.Lang),
			PriceLKR:     doc.priceLKR(),
			ThumbnailURL: doc.ThumbnailURL,
			HasImage:     doc.HasImage,
			CreatedAt:    doc.createdAtTime(),
		})
	}

	total := int(resp.EstimatedTotalHits)
	if resp.TotalHits > 0 {
		total = int(resp.TotalHits)
	}

	return &Result{
		Items:    items,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
		Facets:   categoryFacets(resp.FacetDistribution),
	}, nil
}
