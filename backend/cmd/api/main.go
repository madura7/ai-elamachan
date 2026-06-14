// Command api is the ElaMachan backend HTTP service entrypoint.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/madura7/ai-elamachan/backend/internal/aiassist"
	"github.com/madura7/ai-elamachan/backend/internal/auth"
	"github.com/madura7/ai-elamachan/backend/internal/health"
	"github.com/madura7/ai-elamachan/backend/internal/listings"
	"github.com/madura7/ai-elamachan/backend/internal/storage"
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

	// Phone/OTP auth + JWT/Redis sessions (VER-135, ADR 0002).
	// Requires JWT_SECRET and DATABASE_URL. Redis defaults to localhost:6379.
	// SMS_MODE=dev logs OTPs to stdout; real SMS delivery needs VER-44.
	if h, err := auth.NewHandlerFromEnv(); err != nil {
		log.Printf("auth: endpoints disabled: %v", err)
		mux.HandleFunc("POST /api/v1/auth/otp/request", authUnavailable)
		mux.HandleFunc("POST /api/v1/auth/otp/verify", authUnavailable)
	} else {
		h.RegisterRoutes(mux)
	}

	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = "8080"
	}

	// Listings CRUD + image upload (VER-129).
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://elamachan:elamachan@localhost:5432/elamachan?sslmode=disable"
	}
	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("listings: db: %v", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Ping(pingCtx); err != nil {
		log.Fatalf("listings: db ping: %v", err)
	}
	log.Println("listings: db connected")

	imgDir := os.Getenv("IMAGE_DIR")
	if imgDir == "" {
		imgDir = "/tmp/elamachan-images"
	}
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + port
	}
	imgBaseURL := baseURL + "/api/v1/images"

	stor, err := storage.NewLocal(imgDir)
	if err != nil {
		log.Fatalf("listings: storage: %v", err)
	}
	// Serve locally stored images at /api/v1/images/{key}
	mux.Handle("/api/v1/images/",
		http.StripPrefix("/api/v1/images/", http.FileServer(http.Dir(imgDir))))

	listingStore := listings.NewStore(db, imgBaseURL)
	listings.NewHandler(listingStore, stor).Register(mux)

	log.Printf("elamachan-backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}


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

func authUnavailable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "auth_unavailable",
			"message": "Auth is not configured on this server (check JWT_SECRET and DATABASE_URL)",
		},
	})
}
