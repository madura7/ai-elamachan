package catalog

import (
	"context"
	"encoding/json"
	"net/http"
)

// validLangs is the closed set of supported language codes (ADR 0001).
var validLangs = map[string]bool{"en": true, "si": true, "ta": true}

// Storer is the data-access interface the Handler requires.
// *Store satisfies it; tests supply a fake.
type Storer interface {
	ListCategories(ctx context.Context, lang string) ([]Category, error)
}

// Handler serves GET /api/v1/categories.
type Handler struct {
	store Storer
}

// NewHandler returns a Handler backed by store.
func NewHandler(store Storer) *Handler { return &Handler{store: store} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}
	if !validLangs[lang] {
		writeError(w, http.StatusBadRequest, "invalid_lang", "lang must be en, si, or ta")
		return
	}

	cats, err := h.store.ListCategories(r.Context(), lang)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not fetch categories")
		return
	}
	if cats == nil {
		cats = []Category{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cats)
}

type errEnvelope struct {
	Error errBody `json:"error"`
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errEnvelope{Error: errBody{Code: code, Message: message}})
}
