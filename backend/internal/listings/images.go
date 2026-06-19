package listings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"
	"github.com/madura7/ai-elamachan/backend/internal/auth"
)

const (
	maxImages     = 8
	maxImageBytes = 8 * 1024 * 1024 // 8 MB
	presignTTL    = 15 * time.Minute
)

var allowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

type presignRequest struct {
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type presignResponse struct {
	ImageID   string    `json:"image_id"`
	ObjectKey string    `json:"object_key"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type confirmRequest struct {
	ImageID   string `json:"image_id"`
	SortOrder *int   `json:"sort_order,omitempty"`
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

// imagePresign handles POST /api/v1/listings/{id}/images:presign.
func (h *Handler) imagePresign(w http.ResponseWriter, r *http.Request) {
	if h.blob == nil {
		apierr.Write(w, http.StatusServiceUnavailable, "images_unavailable",
			"image upload is not configured on this server (check BLOB_* env vars)")
		return
	}

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
		log.Printf("images presign: owner lookup: %v", err)
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
		log.Printf("images presign: count: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not check image count")
		return
	}
	if count >= maxImages {
		apierr.Write(w, http.StatusUnprocessableEntity, "image_limit_exceeded",
			fmt.Sprintf("maximum %d images per listing", maxImages))
		return
	}

	var req presignRequest
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

	var imageID string
	if err := h.db.QueryRowContext(r.Context(), `SELECT gen_random_uuid()`).Scan(&imageID); err != nil {
		log.Printf("images presign: gen uuid: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not generate image id")
		return
	}

	objectKey := fmt.Sprintf("listings/%s/%s", listingID, imageID)

	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO listing_images (id, listing_id, object_key, sort_order, status, content_type, size_bytes)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6)
	`, imageID, listingID, objectKey, count, req.ContentType, req.SizeBytes)
	if err != nil {
		log.Printf("images presign: insert pending: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not create image record")
		return
	}

	result, err := h.blob.PresignPut(r.Context(), objectKey, req.ContentType, req.SizeBytes, presignTTL)
	if err != nil {
		log.Printf("images presign: presign put: %v", err)
		_, _ = h.db.ExecContext(r.Context(), `DELETE FROM listing_images WHERE id = $1`, imageID)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not generate upload URL")
		return
	}

	writeJSON(w, http.StatusOK, presignResponse{
		ImageID:   imageID,
		ObjectKey: objectKey,
		UploadURL: result.UploadURL,
		ExpiresAt: result.ExpiresAt,
	})
}

// imageConfirm handles POST /api/v1/listings/{id}/images:confirm.
func (h *Handler) imageConfirm(w http.ResponseWriter, r *http.Request) {
	if h.blob == nil {
		apierr.Write(w, http.StatusServiceUnavailable, "images_unavailable",
			"image upload is not configured on this server (check BLOB_* env vars)")
		return
	}

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
		log.Printf("images confirm: owner lookup: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not verify listing")
		return
	}
	if ownerID != callerID {
		apierr.Write(w, http.StatusForbidden, "forbidden", "not the listing owner")
		return
	}

	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}
	if req.ImageID == "" {
		apierr.Write(w, http.StatusBadRequest, "missing_image_id", "image_id is required")
		return
	}

	var objectKey string
	var sortOrder int
	err = h.db.QueryRowContext(r.Context(), `
		SELECT li.object_key, li.sort_order
		FROM listing_images li
		JOIN listings l ON l.id = li.listing_id
		WHERE li.id = $1 AND li.listing_id = $2 AND li.status = 'pending' AND l.user_id = $3
	`, req.ImageID, listingID, callerID).Scan(&objectKey, &sortOrder)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "pending image not found")
		return
	}
	if err != nil {
		log.Printf("images confirm: lookup: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not look up image")
		return
	}

	exists, err := h.blob.HeadObject(r.Context(), objectKey)
	if err != nil {
		log.Printf("images confirm: head object: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not verify upload")
		return
	}
	if !exists {
		apierr.Write(w, http.StatusUnprocessableEntity, "upload_not_found",
			"object not found in storage; upload the file before confirming")
		return
	}

	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	publicURL := h.blob.PublicURL(objectKey)

	if _, err := h.db.ExecContext(r.Context(), `
		UPDATE listing_images SET status = 'active', sort_order = $1, url = $2
		WHERE id = $3
	`, sortOrder, publicURL, req.ImageID); err != nil {
		log.Printf("images confirm: update: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not activate image")
		return
	}

	if h.onImageChange != nil {
		go h.onImageChange(listingID, true)
	}

	writeJSON(w, http.StatusOK, ImageRecord{
		ID:        req.ImageID,
		URL:       publicURL,
		SortOrder: sortOrder,
	})
}

// imageDelete handles DELETE /api/v1/listings/{id}/images/{imageId}.
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

	var objectKey string
	err = h.db.QueryRowContext(r.Context(),
		`SELECT object_key FROM listing_images WHERE id = $1 AND listing_id = $2`,
		imageID, listingID,
	).Scan(&objectKey)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	if err != nil {
		log.Printf("images delete: lookup: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not look up image")
		return
	}

	if _, err := h.db.ExecContext(r.Context(),
		`DELETE FROM listing_images WHERE id = $1 AND listing_id = $2`, imageID, listingID,
	); err != nil {
		log.Printf("images delete: row: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not delete image")
		return
	}

	if h.blob != nil {
		if err := h.blob.DeleteObject(r.Context(), objectKey); err != nil {
			log.Printf("images delete: storage object %q (best-effort): %v", objectKey, err)
		}
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
