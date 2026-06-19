package listings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"
	"github.com/madura7/ai-elamachan/backend/internal/auth"
)

const (
	maxImages     = 8
	maxImageBytes = 8 * 1024 * 1024 // 8 MB
	// blobHostSuffix is the public host all Vercel Blob URLs share. Uploaded
	// images are persisted client-side via @vercel/blob and only their public
	// URL reaches us — we accept a URL only when it is served from this host,
	// so callers cannot attach arbitrary external URLs to a listing.
	blobHostSuffix = ".public.blob.vercel-storage.com"
)

var allowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// attachRequest is the body of POST /api/v1/listings/{id}/images. The browser
// uploads directly to Vercel Blob (see frontend /api/blob/upload) and posts the
// resulting public URL here for persistence.
type attachRequest struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SortOrder   *int   `json:"sort_order,omitempty"`
}

// fetchListingOwner returns the user_id of the listing. Returns sql.ErrNoRows
// when the listing does not exist.
func (h *Handler) fetchListingOwner(ctx context.Context, listingID string) (string, error) {
	var ownerID string
	err := h.db.QueryRowContext(ctx,
		`SELECT user_id FROM listings WHERE id = $1 AND status = 'active'`,
		listingID,
	).Scan(&ownerID)
	return ownerID, err
}

// blobObjectKey validates that rawURL is an https Vercel Blob URL and returns
// the object key (path without leading slash). Returns ok=false when the URL is
// not a well-formed Blob URL.
func blobObjectKey(rawURL string) (key string, ok bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", false
	}
	if !strings.HasSuffix(u.Host, blobHostSuffix) {
		return "", false
	}
	key = strings.TrimPrefix(u.Path, "/")
	if key == "" {
		return "", false
	}
	return key, true
}

// imageAttach handles POST /api/v1/listings/{id}/images. It persists an
// already-uploaded Vercel Blob URL as an active image on the listing.
func (h *Handler) imageAttach(w http.ResponseWriter, r *http.Request) {
	callerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}

	listingID := r.PathValue("id")

	ownerID, err := h.fetchListingOwner(r.Context(), listingID)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		log.Printf("images attach: owner lookup: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not verify listing")
		return
	}
	if ownerID != callerID {
		apierr.Write(w, http.StatusForbidden, "forbidden", "not the listing owner")
		return
	}

	var count int
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM listing_images WHERE listing_id = $1 AND status = 'active'`,
		listingID,
	).Scan(&count); err != nil {
		log.Printf("images attach: count: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not check image count")
		return
	}
	if count >= maxImages {
		apierr.Write(w, http.StatusUnprocessableEntity, "image_limit_exceeded",
			fmt.Sprintf("maximum %d images per listing", maxImages))
		return
	}

	var req attachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}
	if !allowedContentTypes[req.ContentType] {
		apierr.Write(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"content_type must be image/jpeg, image/png, or image/webp")
		return
	}
	if req.SizeBytes <= 0 || req.SizeBytes > maxImageBytes {
		apierr.Write(w, http.StatusRequestEntityTooLarge, "file_too_large",
			fmt.Sprintf("size_bytes must be between 1 and %d", maxImageBytes))
		return
	}
	objectKey, ok := blobObjectKey(req.URL)
	if !ok {
		apierr.Write(w, http.StatusBadRequest, "invalid_url",
			"url must be a public Vercel Blob URL")
		return
	}

	sortOrder := count
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	var imageID string
	err = h.db.QueryRowContext(r.Context(), `
		INSERT INTO listing_images (id, listing_id, object_key, sort_order, status, content_type, size_bytes, url)
		VALUES (gen_random_uuid(), $1, $2, $3, 'active', $4, $5, $6)
		RETURNING id
	`, listingID, objectKey, sortOrder, req.ContentType, req.SizeBytes, req.URL).Scan(&imageID)
	if err != nil {
		log.Printf("images attach: insert: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not save image")
		return
	}

	if h.onImageChange != nil {
		go h.onImageChange(listingID, true)
	}

	writeJSON(w, http.StatusOK, ImageRecord{
		ID:        imageID,
		URL:       req.URL,
		SortOrder: sortOrder,
	})
}

// imageDelete handles DELETE /api/v1/listings/{id}/images/{imageId}. The Blob
// object itself is left in storage (deletion needs the write token, which lives
// only in the frontend env); the listing simply stops referencing it.
func (h *Handler) imageDelete(w http.ResponseWriter, r *http.Request) {
	callerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}

	listingID := r.PathValue("id")
	imageID := r.PathValue("imageId")

	ownerID, err := h.fetchListingOwner(r.Context(), listingID)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		log.Printf("images delete: owner lookup: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not verify listing")
		return
	}
	if ownerID != callerID {
		apierr.Write(w, http.StatusForbidden, "forbidden", "not the listing owner")
		return
	}

	res, err := h.db.ExecContext(r.Context(),
		`DELETE FROM listing_images WHERE id = $1 AND listing_id = $2`, imageID, listingID,
	)
	if err != nil {
		log.Printf("images delete: row: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not delete image")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		apierr.Write(w, http.StatusNotFound, "not_found", "image not found")
		return
	}

	// Update search has_image when no active images remain (best-effort).
	if h.onImageChange != nil {
		db := h.db
		fn := h.onImageChange
		go func() {
			var remaining int
			_ = db.QueryRowContext(context.Background(),
				`SELECT COUNT(*) FROM listing_images WHERE listing_id = $1 AND status = 'active'`,
				listingID,
			).Scan(&remaining)
			fn(listingID, remaining > 0)
		}()
	}

	w.WriteHeader(http.StatusNoContent)
}
