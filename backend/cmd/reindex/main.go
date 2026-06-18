// Command reindex backfills all active listings into Meilisearch.
// Run once after deploying VER-338 to ensure existing listings are indexed with
// the new has_image and created_at ranking fields.
//
// Required env vars: DATABASE_URL, MEILI_URL, MEILI_MASTER_KEY
//
// Usage: ./reindex [--batch-size N]  (default batch size: 100)
//
// Idempotent: safe to run multiple times.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/madura7/ai-elamachan/backend/internal/listings"
	"github.com/madura7/ai-elamachan/backend/internal/search"
)

func main() {
	batchSize := flag.Int("batch-size", 100, "number of listings to upsert per Meilisearch call")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("reindex: DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("reindex: open db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(5)

	svc, err := search.NewFromEnv()
	if err != nil {
		log.Fatalf("reindex: search client: %v", err)
	}
	if svc == nil {
		log.Fatal("reindex: MEILI_URL is required")
	}

	log.Println("reindex: applying index settings…")
	if err := svc.EnsureIndex(ctx); err != nil {
		log.Fatalf("reindex: EnsureIndex: %v", err)
	}

	log.Println("reindex: loading listings from database…")
	total, indexed, err := reindexAll(ctx, db, svc, *batchSize)
	if err != nil {
		log.Fatalf("reindex: %v", err)
	}
	log.Printf("reindex: done — %d/%d listings indexed", indexed, total)
}

// reindexAll streams all active listings from Postgres and batch-upserts them
// into Meilisearch. Returns (total rows, rows indexed, error).
func reindexAll(ctx context.Context, db *sql.DB, svc *search.Service, batchSize int) (total, indexed int, err error) {
	rows, err := db.QueryContext(ctx, `
		SELECT l.id,
		       c.slug,
		       l.content_language,
		       l.price_cents,
		       l.created_at,
		       COALESCE(
		         (SELECT title FROM listing_translations WHERE listing_id = l.id AND lang = l.content_language LIMIT 1),
		         ''
		       ),
		       (SELECT url FROM listing_images
		          WHERE listing_id = l.id AND status = 'active'
		          ORDER BY sort_order LIMIT 1),
		       EXISTS(SELECT 1 FROM listing_images WHERE listing_id = l.id AND status = 'active') AS has_image
		FROM listings l
		JOIN categories c ON c.id = l.category_id
		WHERE l.status = 'active'
		ORDER BY l.created_at
	`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var batch []listings.IndexableDoc
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := svc.BatchIndexListings(ctx, batch); err != nil {
			return err
		}
		indexed += len(batch)
		batch = batch[:0]
		return nil
	}

	for rows.Next() {
		total++
		var doc listings.IndexableDoc
		var priceCents sql.NullInt64
		var thumbnailURL sql.NullString

		if err := rows.Scan(
			&doc.ID, &doc.Category, &doc.ContentLanguage,
			&priceCents, &doc.CreatedAt, &doc.Title,
			&thumbnailURL, &doc.HasImage,
		); err != nil {
			return total, indexed, err
		}
		if priceCents.Valid {
			lkr := priceCents.Int64 / 100
			doc.PriceLKR = &lkr
		}
		if thumbnailURL.Valid && thumbnailURL.String != "" {
			u := thumbnailURL.String
			doc.ThumbnailURL = &u
		}

		batch = append(batch, doc)
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return total, indexed, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return total, indexed, err
	}
	return total, indexed, flush()
}
