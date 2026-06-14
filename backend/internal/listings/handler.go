package listings

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"
	"github.com/madura7/ai-elamachan/backend/internal/storage"
)

const (
	maxImages    = 10
	maxImageSize = 5 << 20 // 5 MiB
)

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// Handler registers all /api/v1/listings routes.
type Handler struct {
	store   *Store
	storage storage.Store
}

// NewHandler creates a Handler.
func NewHandler(store *Store, stor storage.Store) *Handler {
	return &Handler{store: store, storage: stor}
}

// Register adds listing routes to mux using Go 1.22 method+path patterns.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/listings", h.list)
	mux.HandleFunc("POST /api/v1/listings", h.create)
	mux.HandleFunc("GET /api/v1/listings/{id}", h.get)
	mux.HandleFunc("PUT /api/v1/listings/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/listings/{id}", h.deleteOne)
	mux.HandleFunc("POST /api/v1/listings/{id}/images", h.uploadImage)
}

// list handles GET /api/v1/listings
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if category != "" && !ValidCategories[category] {
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

	result, err := h.store.List(r.Context(), category, page, pageSize)
	if err != nil {
		log.Printf("listings.list: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "failed to list listings")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// create handles POST /api/v1/listings
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if err := validateCreate(req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	listing, err := h.store.Create(r.Context(), DevOwnerID, req)
	if err != nil {
		log.Printf("listings.create: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "failed to create listing")
		return
	}

	writeJSON(w, http.StatusCreated, listing)
}

// get handles GET /api/v1/listings/{id}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidUUID(id) {
		apierr.Write(w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return
	}

	listing, err := h.store.Get(r.Context(), id)
	if err != nil {
		log.Printf("listings.get %s: %v", id, err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "failed to get listing")
		return
	}
	if listing == nil {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}

	writeJSON(w, http.StatusOK, listing)
}

// update handles PUT /api/v1/listings/{id}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidUUID(id) {
		apierr.Write(w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if err := validateUpdate(req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	listing, err := h.store.Update(r.Context(), id, DevOwnerID, req)
	if err != nil {
		log.Printf("listings.update %s: %v", id, err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "failed to update listing")
		return
	}
	if listing == nil {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}

	writeJSON(w, http.StatusOK, listing)
}

// deleteOne handles DELETE /api/v1/listings/{id}
func (h *Handler) deleteOne(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidUUID(id) {
		apierr.Write(w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return
	}

	deleted, err := h.store.Delete(r.Context(), id, DevOwnerID)
	if err != nil {
		log.Printf("listings.delete %s: %v", id, err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "failed to delete listing")
		return
	}
	if !deleted {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// uploadImage handles POST /api/v1/listings/{id}/images
func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidUUID(id) {
		apierr.Write(w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return
	}

	exists, err := h.store.ListingExists(r.Context(), id)
	if err != nil {
		log.Printf("listings.uploadImage: exists: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "failed to check listing")
		return
	}
	if !exists {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}

	count, err := h.store.ImageCount(r.Context(), id)
	if err != nil {
		log.Printf("listings.uploadImage: count: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "failed to check image count")
		return
	}
	if count >= maxImages {
		apierr.Write(w, http.StatusBadRequest, "too_many_images",
			fmt.Sprintf("a listing may have at most %d images", maxImages))
		return
	}

	if err := r.ParseMultipartForm(maxImageSize); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_form", "request must be multipart/form-data with an image field")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, "missing_image", "image field is required")
		return
	}
	defer file.Close()

	if header.Size > maxImageSize {
		apierr.Write(w, http.StatusRequestEntityTooLarge, "image_too_large", "image must be ≤ 5 MiB")
		return
	}

	// Sniff content type from first 512 bytes (avoids trusting the client-supplied header).
	sniff := make([]byte, 512)
	n, _ := file.Read(sniff)
	contentType := http.DetectContentType(sniff[:n])
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		apierr.Write(w, http.StatusUnsupportedMediaType, "unsupported_image_type",
			"image must be jpeg, png, webp, or gif")
		return
	}

	objectKey := fmt.Sprintf("%s/%s%s", id, mustUUID(), ext)
	body := io.MultiReader(bytes.NewReader(sniff[:n]), file)

	if err := h.storage.Put(r.Context(), objectKey, body, contentType); err != nil {
		log.Printf("listings.uploadImage: storage.Put: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "storage_error", "failed to store image")
		return
	}

	img, err := h.store.AddImage(r.Context(), id, objectKey, count)
	if err != nil {
		log.Printf("listings.uploadImage: AddImage: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "failed to record image")
		return
	}

	writeJSON(w, http.StatusCreated, img)
}

// --- helpers -----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func validateCreate(req CreateRequest) error {
	if !ValidCategories[req.Category] {
		return fmt.Errorf("unknown category: %q", req.Category)
	}
	if !ValidLangs[req.ContentLanguage] {
		return fmt.Errorf("unsupported content_language: %q", req.ContentLanguage)
	}
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

func validateUpdate(req UpdateRequest) error {
	if !ValidCategories[req.Category] {
		return fmt.Errorf("unknown category: %q", req.Category)
	}
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// isValidUUID does a lightweight format check: 8-4-4-4-12 hex with dashes.
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// mustUUID returns a random v4 UUID string; panics on entropy failure.
func mustUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("uuid: rand: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
