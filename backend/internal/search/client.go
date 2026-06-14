// Package search wraps Meilisearch for ElaMachan listing search.
//
// Index layout: one Meilisearch document per listing-translation pair.
// Document ID: "{listing_id}_{lang}". DistinctAttribute "listing_id"
// ensures each listing appears at most once in any result page.
package search

import (
	"context"
	"fmt"

	meilisearch "github.com/meilisearch/meilisearch-go"
)

const indexUID = "listings"

// Document is a single index record (one listing × one language).
type Document struct {
	ID            string  `json:"id"`
	ListingID     string  `json:"listing_id"`
	Lang          string  `json:"lang"`
	Category      string  `json:"category"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	PriceLKR      *int64  `json:"price_lkr,omitempty"`
	ThumbnailURL  *string `json:"thumbnail_url,omitempty"`
	CreatedAtUnix int64   `json:"created_at_unix"`
}

// Client owns the Meilisearch connection and index configuration.
type Client struct {
	svc *meilisearch.Client
}

// New connects to Meilisearch and idempotently configures the listings index.
func New(host, apiKey string) (*Client, error) {
	svc := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   host,
		APIKey: apiKey,
	})
	c := &Client{svc: svc}
	if err := c.ensureIndex(); err != nil {
		return nil, fmt.Errorf("search: ensure index: %w", err)
	}
	return c, nil
}

func (c *Client) ensureIndex() error {
	task, err := c.svc.CreateIndex(&meilisearch.IndexConfig{
		Uid:        indexUID,
		PrimaryKey: "id",
	})
	if err == nil {
		if _, werr := c.svc.WaitForTask(task.TaskUID); werr != nil {
			return fmt.Errorf("wait create index: %w", werr)
		}
	}
	// If err != nil the index may already exist; proceed to settings.

	distinctAttr := "listing_id"
	settingsTask, err := c.idx().UpdateSettings(&meilisearch.Settings{
		SearchableAttributes: []string{"title", "description"},
		FilterableAttributes: []string{"category", "lang", "listing_id"},
		SortableAttributes:   []string{"created_at_unix", "price_lkr"},
		DistinctAttribute:    &distinctAttr,
	})
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	if _, werr := c.svc.WaitForTask(settingsTask.TaskUID); werr != nil {
		return fmt.Errorf("wait settings: %w", werr)
	}
	return nil
}

func (c *Client) idx() *meilisearch.Index {
	return c.svc.Index(indexUID)
}

// Upsert adds or replaces a document in the listings index.
func (c *Client) Upsert(_ context.Context, doc Document) error {
	_, err := c.idx().AddDocuments([]Document{doc})
	if err != nil {
		return fmt.Errorf("search: upsert %s: %w", doc.ID, err)
	}
	return nil
}

// Delete removes all index documents for a listing (all languages).
func (c *Client) Delete(_ context.Context, listingID string) error {
	_, err := c.idx().DeleteDocumentsByFilter(fmt.Sprintf("listing_id = %q", listingID))
	if err != nil {
		return fmt.Errorf("search: delete %s: %w", listingID, err)
	}
	return nil
}
