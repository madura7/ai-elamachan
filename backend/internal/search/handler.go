package search

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"
	"github.com/madura7/ai-elamachan/backend/internal/listings"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Handler serves GET /api/v1/search. A nil *Service makes the route return 503,
// mirroring how auth/ai-draft degrade when their dependency is unconfigured.
type Handler struct {
	svc *Service
}

// NewHandler wraps svc (which may be nil → search unavailable).
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register adds the search route to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/search", h.search)
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		apierr.Write(w, http.StatusServiceUnavailable, "search_unavailable",
			"search is not configured on this server (check MEILI_URL)")
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		apierr.Write(w, http.StatusBadRequest, "missing_query", "q parameter is required")
		return
	}

	lang := r.URL.Query().Get("lang")
	if lang != "" && !listings.ValidLangs[lang] {
		apierr.Write(w, http.StatusBadRequest, "invalid_lang", "lang must be en, si, or ta")
		return
	}

	category := r.URL.Query().Get("category")
	if category != "" && !listings.ValidCategories[category] {
		apierr.Write(w, http.StatusBadRequest, "invalid_category", "unknown category slug")
		return
	}

	page, pageSize := 1, defaultPageSize
	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			apierr.Write(w, http.StatusBadRequest, "invalid_page", "page must be a positive integer")
			return
		}
		page = n
	}
	if v := r.URL.Query().Get("pageSize"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPageSize {
			apierr.Write(w, http.StatusBadRequest, "invalid_page_size", "pageSize must be between 1 and 100")
			return
		}
		pageSize = n
	}

	result, err := h.svc.Search(r.Context(), Params{
		Query:    q,
		Lang:     lang,
		Category: category,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		log.Printf("search.search: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "search failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}
