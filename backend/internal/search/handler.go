package search

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"
	meilisearch "github.com/meilisearch/meilisearch-go"
)

// Handler registers the search route.
type Handler struct {
	client *Client
}

// NewHandler returns a Handler backed by client.
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// Register adds the search route to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/search", h.search)
}

// searchHit mirrors the subset of Document fields returned in hits.
type searchHit struct {
	ListingID     string  `json:"listing_id"`
	Category      string  `json:"category"`
	Title         string  `json:"title"`
	PriceLKR      *int64  `json:"price_lkr"`
	ThumbnailURL  *string `json:"thumbnail_url"`
	CreatedAtUnix int64   `json:"created_at_unix"`
}

// SearchPage is the paginated search response.
type SearchPage struct {
	Items    []SearchItem `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Total    int          `json:"total"`
}

// SearchItem is a listing summary enriched with a search score hint.
type SearchItem struct {
	ID           string    `json:"id"`
	Category     string    `json:"category"`
	Title        string    `json:"title"`
	PriceLKR     *int64    `json:"price_lkr"`
	ThumbnailURL *string   `json:"thumbnail_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// validLangs mirrors the Lang enum.
var validLangs = map[string]bool{"en": true, "si": true, "ta": true}

// validCategories mirrors the CategorySlug enum.
var validCategories = map[string]bool{
	"electronics": true, "vehicles": true, "property": true,
	"home_garden": true, "fashion": true, "mobile_phones": true,
	"services": true, "jobs": true, "pets": true, "other": true,
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		apierr.Write(w, http.StatusBadRequest, "missing_query", "q is required")
		return
	}

	lang := r.URL.Query().Get("lang")
	if lang != "" && !validLangs[lang] {
		apierr.Write(w, http.StatusBadRequest, "invalid_lang", "unknown lang code")
		return
	}

	category := r.URL.Query().Get("category")
	if category != "" && !validCategories[category] {
		apierr.Write(w, http.StatusBadRequest, "invalid_category", "unknown category slug")
		return
	}

	page, pageSize := 1, 20
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
		if err != nil || n < 1 || n > 100 {
			apierr.Write(w, http.StatusBadRequest, "invalid_page_size", "pageSize must be between 1 and 100")
			return
		}
		pageSize = n
	}

	var filters []string
	if lang != "" {
		filters = append(filters, fmt.Sprintf("lang = %q", lang))
	}
	if category != "" {
		filters = append(filters, fmt.Sprintf("category = %q", category))
	}

	req := &meilisearch.SearchRequest{
		Limit:  int64(pageSize),
		Offset: int64((page - 1) * pageSize),
	}
	if len(filters) == 1 {
		req.Filter = filters[0]
	} else if len(filters) > 1 {
		req.Filter = filters[0] + " AND " + filters[1]
	}

	res, err := h.client.idx().Search(q, req)
	if err != nil {
		log.Printf("search: meilisearch: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "search_error", "search unavailable")
		return
	}

	// Decode hits via JSON round-trip (meilisearch-go returns []interface{}).
	hitsJSON, _ := json.Marshal(res.Hits)
	var hits []searchHit
	if err := json.Unmarshal(hitsJSON, &hits); err != nil {
		log.Printf("search: decode hits: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "search_error", "failed to decode results")
		return
	}

	items := make([]SearchItem, 0, len(hits))
	for _, h := range hits {
		items = append(items, SearchItem{
			ID:           h.ListingID,
			Category:     h.Category,
			Title:        h.Title,
			PriceLKR:     h.PriceLKR,
			ThumbnailURL: h.ThumbnailURL,
			CreatedAt:    time.Unix(h.CreatedAtUnix, 0).UTC(),
		})
	}

	total := int(res.EstimatedTotalHits)
	result := SearchPage{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}
