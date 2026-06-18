// Command seed populates Postgres + Meilisearch with demo data on boot.
// It is idempotent: running it twice leaves row/document counts unchanged.
//
// Required env vars: DATABASE_URL
// Optional env vars: MEILI_URL, MEILI_MASTER_KEY (skips Meili if absent)
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("seed: DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("seed: open db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(5)

	if err := seedPostgres(ctx, db); err != nil {
		log.Fatalf("seed: postgres: %v", err)
	}
	log.Println("seed: postgres OK")

	if os.Getenv("MEILI_URL") == "" || os.Getenv("MEILI_MASTER_KEY") == "" {
		log.Println("seed: MEILI_URL/MEILI_MASTER_KEY not set — skipping Meilisearch")
		return
	}

	if err := seedMeilisearch(ctx); err != nil {
		log.Fatalf("seed: meilisearch: %v", err)
	}
	log.Println("seed: meilisearch OK")
}
