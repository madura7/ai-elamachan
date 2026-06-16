// Package search wraps Meilisearch for ElaMachan listing search.
//
// The service operates in search-only mode: it queries the existing index
// without modifying its settings. This keeps it compatible with seed documents
// (cmd/seed) which use flat title/description/category_slug fields, as well as
// future per-language documents from a full indexing pipeline.
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

// Service owns the Meilisearch client and exposes Search.
// A nil *Service is the "search disabled" sentinel.
type Service struct {
	client *meilisearch.Client
	index  *meilisearch.Index
}

// NewFromEnv builds a Service from MEILI_URL / MEILI_MASTER_KEY.
// Returns (nil, nil) when MEILI_URL is unset — callers treat nil as "search
// disabled" and return 503 on search requests.
// Constructing the client does not open a connection; per-call errors surface
// lazily at search time.
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

// Search runs a full-text query against the listings index and returns a page
// of results. It handles both seed-format documents (flat title/category_slug)
// and per-language documents (title_en/category fields) by delegating title and
// category resolution to the Document helper methods.
func (s *Service) Search(_ context.Context, p Params) (*Result, error) {
	req := &meilisearch.SearchRequest{
		Page:        int64(p.Page),
		HitsPerPage: int64(p.PageSize),
		Facets:      []string{"category", "category_slug"},
	}
	if p.Category != "" {
		// Support both the service-indexed "category" field and the seed's
		// "category_slug" field in a single OR filter so the endpoint works
		// regardless of which document format is in the index.
		req.Filter = fmt.Sprintf(`category = %q OR category_slug = %q`, p.Category, p.Category)
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
			CreatedAt:    doc.createdAtTime(),
		})
	}

	return &Result{
		Items:    items,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    int(resp.TotalHits),
		Facets:   categoryFacets(resp.FacetDistribution),
	}, nil
}
