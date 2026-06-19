// Command api is the ElaMachan backend HTTP service entrypoint.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/madura7/ai-elamachan/backend/internal/aiassist"
	"github.com/madura7/ai-elamachan/backend/internal/auth"
	"github.com/madura7/ai-elamachan/backend/internal/blob"
	"github.com/madura7/ai-elamachan/backend/internal/health"
	"github.com/madura7/ai-elamachan/backend/internal/inquiries"
	"github.com/madura7/ai-elamachan/backend/internal/listings"
	"github.com/madura7/ai-elamachan/backend/internal/middleware"
	"github.com/madura7/ai-elamachan/backend/internal/search"
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
		mux.HandleFunc("POST /api/v1/listings/ai-draft", aiAssistUnavailable)
	} else {
		mux.Handle("POST /api/v1/listings/ai-draft", h)
	}

	// Phone/OTP auth + JWT/Redis sessions (VER-135, ADR 0002).
	// Requires JWT_SECRET and DATABASE_URL. Redis defaults to localhost:6379.
	// SMS_MODE=dev logs OTPs to stdout; real SMS delivery needs VER-44.
	var bearer func(http.Handler) http.Handler
	var verifyToken func(ctx context.Context, token string) (string, error)
	if h, err := auth.NewHandlerFromEnv(); err != nil {
		log.Printf("auth: endpoints disabled: %v", err)
		mux.HandleFunc("POST /api/v1/auth/otp/request", authUnavailable)
		mux.HandleFunc("POST /api/v1/auth/otp/verify", authUnavailable)
	} else {
		bearer = h.BearerMiddleware
		verifyToken = h.VerifyToken
		h.RegisterRoutes(mux)
	}

	// Object storage for listing images (VER-365/VER-368).
	// Optional: when BLOB_ENDPOINT is absent, image upload returns 503.
	blobStore, err := blob.NewFromEnv()
	if err != nil {
		log.Printf("blob: image upload disabled: %v", err)
	} else if blobStore == nil {
		log.Printf("blob: image upload disabled (BLOB_ENDPOINT not set)")
	}

	// Full-text search via Meilisearch (VER-225).
	// Optional: when MEILI_URL is absent the endpoint returns 503 rather than
	// crashing the whole service. This mirrors the graceful-degradation pattern
	// used by auth and ai-assist.
	var searchSvc *search.Service
	if svc, err := search.NewFromEnv(); err != nil {
		log.Printf("search: endpoint disabled: %v", err)
		mux.HandleFunc("GET /api/v1/search", searchUnavailable)
	} else {
		searchSvc = svc
		search.NewHandler(svc).Register(mux)
	}

	// Listings browse, create, and category taxonomy (VER-225, VER-289).
	// Requires DATABASE_URL. Falls back to 503 stubs when DB is unavailable.
	// POST /listings and GET /listings?mine=true require bearer auth.
	if h, err := listings.NewHandlerFromEnv(); err != nil {
		log.Printf("listings: endpoints disabled: %v", err)
		mux.HandleFunc("GET /api/v1/listings", listingsUnavailable)
		mux.HandleFunc("POST /api/v1/listings", listingsUnavailable)
		mux.HandleFunc("GET /api/v1/listings/{id}", listingsUnavailable)
		mux.HandleFunc("GET /api/v1/categories", listingsUnavailable)
	} else {
		if bearer != nil {
			h.SetAuth(bearer, verifyToken)
		}
		var onImageChange func(string, bool)
		if searchSvc != nil {
			onImageChange = searchSvc.UpdateHasImage
		}
		h.SetDeps(blobStore, onImageChange)
		h.RegisterRoutes(mux)
	}

	// Inquiry persistence + seller inbox (VER-297, VER-295 M1).
	// Requires DATABASE_URL and bearer auth.
	if h, err := inquiries.NewHandlerFromEnv(); err != nil {
		log.Printf("inquiries: endpoints disabled: %v", err)
		mux.HandleFunc("POST /api/v1/listings/{listingId}/inquiries", inquiriesUnavailable)
		mux.HandleFunc("GET /api/v1/inquiries", inquiriesUnavailable)
	} else {
		if bearer != nil {
			h.SetBearer(bearer)
		}
		h.RegisterRoutes(mux)
	}

	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("elamachan-backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, middleware.CORS(mux)); err != nil {
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

func listingsUnavailable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "listings_unavailable",
			"message": "Listings endpoints are not configured on this server (check DATABASE_URL)",
		},
	})
}

func inquiriesUnavailable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "inquiries_unavailable",
			"message": "Inquiries endpoints are not configured on this server (check DATABASE_URL)",
		},
	})
}

func searchUnavailable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "search_unavailable",
			"message": "Search is not configured on this server (check MEILI_URL)",
		},
	})
}
