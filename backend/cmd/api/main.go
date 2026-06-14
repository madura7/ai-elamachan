// Command api is the ElaMachan backend HTTP service entrypoint.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/madura7/ai-elamachan/backend/internal/aiassist"
	"github.com/madura7/ai-elamachan/backend/internal/catalog"
	"github.com/madura7/ai-elamachan/backend/internal/health"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(health.Check())
	})

	// AI-assisted listing draft (VER-58). Returns a draft only — never creates
	// or publishes a listing. Requires ANTHROPIC_API_KEY (secret, from Secret
	// Manager). When the key is absent the route still exists but returns 503,
	// so the service boots in environments where the key is not yet provisioned.
	if h, err := aiassist.NewHandlerFromEnv(); err != nil {
		log.Printf("aiassist: AI-draft endpoint disabled: %v", err)
		mux.HandleFunc("POST /api/listings/ai-draft", aiAssistUnavailable)
	} else {
		mux.Handle("POST /api/listings/ai-draft", h)
	}

	// Localized category taxonomy (VER-128). DATABASE_URL must be set; if absent
	// the process still starts but the categories route returns 503 so dependent
	// services can degrade gracefully in environments without a DB.
	if dbURL := os.Getenv("DATABASE_URL"); dbURL == "" {
		log.Printf("catalog: DATABASE_URL not set — categories endpoint disabled")
		mux.HandleFunc("GET /api/v1/categories", categoriesUnavailable)
	} else {
		db, err := sql.Open("pgx", dbURL)
		if err != nil {
			log.Fatalf("db: open: %v", err)
		}
		defer db.Close()
		if err := db.PingContext(context.Background()); err != nil {
			log.Printf("catalog: db ping failed (%v) — categories endpoint degraded", err)
		}
		mux.Handle("GET /api/v1/categories", catalog.NewHandler(catalog.NewStore(db)))
	}

	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("elamachan-backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// aiAssistUnavailable responds when the AI-draft endpoint is registered but its
// API key is not provisioned, using the canonical error envelope (ADR 0003).
func aiAssistUnavailable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "ai_assist_unavailable",
			"message": "AI-assist is not configured on this server",
		},
	})
}

// categoriesUnavailable responds when DATABASE_URL is not configured.
func categoriesUnavailable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "database_unavailable",
			"message": "categories are not available on this server",
		},
	})
}
