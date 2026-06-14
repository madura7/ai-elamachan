// Package search wires the VER-41 Meilisearch spike into production: it indexes
// localized listing fields on every listing write and serves GET /api/v1/search
// with prefix/as-you-type, typo-tolerance, and category facets.
//
// Indexing is best-effort and decoupled from the listings write path: a search
// outage never fails a listing create/update/delete (callers log and continue).
// The index document is rebuilt from the DB on each write so it always reflects
// the full set of cached translations, including machine ones (VER-139).
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meilisearch/meilisearch-go"
)

// indexUID is the single Meilisearch index backing listing search.
const indexUID = "listings"

// Service owns the Meilisearch client and the DB pool it reads documents from.
// A nil *Service is the "search disabled" sentinel: callers guard on it the same
// way the listings handler guards on a nil Translator.
type Service struct {
	client       *meilisearch.Client
	index        *meilisearch.Index
	db           *pgxpool.Pool
	imageBaseURL string
}

// NewFromEnv builds a Service from MEILI_URL / MEILI_MASTER_KEY. It returns
// (nil, nil) when MEILI_URL is unset so the caller can boot with search
// disabled (the /api/v1/search route then returns 503 and indexing is skipped).
// Constructing the client does not open a connection, so a Service is returned
// even when Meilisearch is unreachable; per-call errors surface lazily.
func NewFromEnv(db *pgxpool.Pool, imageBaseURL string) (*Service, error) {
	host := os.Getenv("MEILI_URL")
	if host == "" {
		return nil, nil
	}
	if db == nil {
		return nil, fmt.Errorf("search: db pool is required")
	}
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:    host,
		APIKey:  os.Getenv("MEILI_MASTER_KEY"),
		Timeout: 5 * time.Second,
	})
	return &Service{
		client:       client,
		index:        client.Index(indexUID),
		db:           db,
		imageBaseURL: imageBaseURL,
	}, nil
}

// EnsureIndex creates the index (if absent) and applies the search settings the
// VER-41 spike prescribes: per-language searchable title/description fields, a
// filterable + facetable category, and created_at sorting. Typo-tolerance and
// prefix search are Meilisearch defaults, so no override is needed.
//
// Settings updates are async on the Meilisearch side; we enqueue them and do not
// block on task completion. Safe to call on every boot — it is idempotent.
func (s *Service) EnsureIndex(ctx context.Context) error {
	// CreateIndex is a no-op-ish if the index already exists (returns a task that
	// fails harmlessly); GetIndex-then-create races, so just attempt create and
	// ignore an "index_already_exists" outcome by not waiting on the task.
	if _, err := s.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        indexUID,
		PrimaryKey: "id",
	}); err != nil {
		return fmt.Errorf("search: create index: %w", err)
	}

	settings := &meilisearch.Settings{
		SearchableAttributes: []string{
			"title_en", "title_si", "title_ta",
			"description_en", "description_si", "description_ta",
		},
		FilterableAttributes: []string{"category", "content_language"},
		SortableAttributes:   []string{"created_at_ts"},
		TypoTolerance:        &meilisearch.TypoTolerance{Enabled: true},
	}
	if _, err := s.index.UpdateSettings(settings); err != nil {
		return fmt.Errorf("search: update settings: %w", err)
	}
	return nil
}

// IndexListing (re)builds the search document for listingID from the DB and
// upserts it (AddDocuments upserts on the "id" primary key). If the listing is
// missing or no longer active it is removed from the index instead.
func (s *Service) IndexListing(ctx context.Context, listingID string) error {
	doc, err := s.buildDocument(ctx, listingID)
	if err != nil {
		return err
	}
	if doc == nil {
		// Not active (draft/removed/sold) → ensure it is not searchable.
		return s.DeleteListing(ctx, listingID)
	}
	if _, err := s.index.AddDocuments([]Document{*doc}, "id"); err != nil {
		return fmt.Errorf("search: add document %s: %w", listingID, err)
	}
	return nil
}

// DeleteListing removes a listing from the index. Deleting an absent document is
// not an error in Meilisearch.
func (s *Service) DeleteListing(ctx context.Context, listingID string) error {
	if _, err := s.index.DeleteDocument(listingID); err != nil {
		return fmt.Errorf("search: delete document %s: %w", listingID, err)
	}
	return nil
}

// buildDocument assembles the denormalized Meilisearch document for an active
// listing, pivoting all of its listing_translations rows into per-language
// fields. Returns (nil, nil) when the listing is not active.
func (s *Service) buildDocument(ctx context.Context, listingID string) (*Document, error) {
	var (
		category        string
		contentLanguage string
		priceCents      *int64
		createdAt       time.Time
		thumbKey        *string
	)
	err := s.db.QueryRow(ctx, `
		SELECT c.slug, l.content_language, l.price_cents, l.created_at,
		       (SELECT object_key FROM listing_images
		        WHERE listing_id = l.id ORDER BY sort_order LIMIT 1)
		FROM listings l
		JOIN categories c ON c.id = l.category_id
		WHERE l.id = $1 AND l.status = 'active'
	`, listingID).Scan(&category, &contentLanguage, &priceCents, &createdAt, &thumbKey)
	if err != nil {
		// pgx.ErrNoRows means not active/not found → signal "remove from index".
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("search: load listing %s: %w", listingID, err)
	}

	doc := &Document{
		ID:              listingID,
		Category:        category,
		ContentLanguage: contentLanguage,
		PriceLKR:        centsToLKR(priceCents),
		CreatedAt:       createdAt.UTC().Format(time.RFC3339),
		CreatedAtTS:     createdAt.Unix(),
	}
	if thumbKey != nil {
		u := s.imageBaseURL + "/" + *thumbKey
		doc.ThumbnailURL = &u
	}

	rows, err := s.db.Query(ctx, `
		SELECT lang, title, COALESCE(description, '')
		FROM listing_translations WHERE listing_id = $1
	`, listingID)
	if err != nil {
		return nil, fmt.Errorf("search: load translations %s: %w", listingID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var lang, title, desc string
		if err := rows.Scan(&lang, &title, &desc); err != nil {
			return nil, fmt.Errorf("search: scan translation: %w", err)
		}
		switch lang {
		case "en":
			doc.TitleEN, doc.DescEN = title, desc
		case "si":
			doc.TitleSI, doc.DescSI = title, desc
		case "ta":
			doc.TitleTA, doc.DescTA = title, desc
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search: translation rows: %w", err)
	}
	return doc, nil
}

// Search runs a query against the index and maps hits to Summaries plus category
// facet counts. Prefix/as-you-type and typo-tolerance are inherent to the
// Meilisearch query; the category param becomes a filter and the facet
// distribution is always requested over "category".
func (s *Service) Search(ctx context.Context, p Params) (*Result, error) {
	req := &meilisearch.SearchRequest{
		Page:        int64(p.Page),
		HitsPerPage: int64(p.PageSize),
		Facets:      []string{"category"},
	}
	if p.Category != "" {
		req.Filter = fmt.Sprintf("category = %q", p.Category)
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
			Category:     doc.Category,
			Title:        doc.titleFor(p.Lang),
			PriceLKR:     doc.PriceLKR,
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
