package listings

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"
	"github.com/madura7/ai-elamachan/backend/internal/auth"
	"github.com/madura7/ai-elamachan/backend/internal/storage"
)

// Image upload limits (VER-299). Enforced server-side at presign time; the
// presigned PUT also pins Content-Type so the client cannot upload a different
// type than was validated.
const (
	maxImagesPerListing = 8
	defaultMaxImageBytes = 8 * 1024 * 1024 // 8MB
	presignTTL          = 10 * time.Minute
)

// allowedImageTypes is the closed set of acceptable upload MIME types.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// presignRequest is the body for POST /listings/{id}/images:presign.
type presignRequest struct {
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

// presignResponse is returned from the presign call. The client PUTs the file
// bytes directly to uploadUrl, then calls :confirm with imageId.
type presignResponse struct {
	ImageID   string    `json:"image_id"`
	ObjectKey string    `json:"object_key"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// confirmRequest is the body for POST /listings/{id}/images:confirm.
type confirmRequest struct {
	ImageID string `json:"image_id"`
	Width   *int   `json:"width"`
	Height  *int   `json:"height"`
}

// presignImage serves POST /api/v1/listings/{id}/images:presign — owner-only.
// Validates ownership, content-type, size, and the per-listing image cap, then
// inserts a pending listing_images row and returns a short-lived presigned PUT
// URL so the client can upload bytes directly to object storage.
func (h *Handler) presignImage(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		apierr.Write(w, http.StatusServiceUnavailable, "storage_unavailable", "image storage is not configured on this server")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}
	listingID := r.PathValue("id")

	// Validate the request body before the ownership DB lookup (mirrors
	// updateListing) so malformed uploads are rejected cheaply.
	var req presignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}
	if !allowedImageTypes[req.ContentType] {
		apierr.Write(w, http.StatusBadRequest, "invalid_content_type", "content_type must be image/jpeg, image/png, or image/webp")
		return
	}
	if req.SizeBytes <= 0 || req.SizeBytes > h.maxImageBytes() {
		apierr.Write(w, http.StatusBadRequest, "invalid_size", fmt.Sprintf("size_bytes must be between 1 and %d", h.maxImageBytes()))
		return
	}

	if !h.ownsListing(w, r, listingID, userID) {
		return
	}

	// Enforce the per-listing cap across pending + active rows so a flurry of
	// presigns cannot exceed the limit before any confirm lands.
	var count int
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM listing_images WHERE listing_id = $1`, listingID).Scan(&count); err != nil {
		log.Printf("listings: count images: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not count images")
		return
	}
	if count >= maxImagesPerListing {
		apierr.Write(w, http.StatusConflict, "image_limit_reached", fmt.Sprintf("a listing may have at most %d images", maxImagesPerListing))
		return
	}

	imageID, err := newUUID()
	if err != nil {
		log.Printf("listings: uuid: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not allocate image id")
		return
	}
	objectKey := fmt.Sprintf("listings/%s/%s.%s", listingID, imageID, storage.ExtForContentType(req.ContentType))

	// Next sort_order = max+1 to satisfy UNIQUE (listing_id, sort_order).
	var nextSort int
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(MAX(sort_order)+1, 0) FROM listing_images WHERE listing_id = $1`, listingID).Scan(&nextSort); err != nil {
		log.Printf("listings: next sort_order: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not allocate image slot")
		return
	}

	if _, err := h.db.ExecContext(r.Context(), `
		INSERT INTO listing_images (id, listing_id, object_key, sort_order, status, content_type, size_bytes)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6)
	`, imageID, listingID, objectKey, nextSort, req.ContentType, req.SizeBytes); err != nil {
		log.Printf("listings: insert pending image: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not create image record")
		return
	}

	uploadURL, err := h.store.PresignPut(r.Context(), objectKey, req.ContentType, presignTTL)
	if err != nil {
		log.Printf("listings: presign: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not presign upload")
		return
	}

	writeJSON(w, http.StatusCreated, presignResponse{
		ImageID:   imageID,
		ObjectKey: objectKey,
		UploadURL: uploadURL,
		ExpiresAt: time.Now().Add(presignTTL).UTC(),
	})
}

// confirmImage serves POST /api/v1/listings/{id}/images:confirm — owner-only.
// HEAD-verifies the object actually landed in storage, then activates the
// pending row (status='active', url derived from object_key).
func (h *Handler) confirmImage(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		apierr.Write(w, http.StatusServiceUnavailable, "storage_unavailable", "image storage is not configured on this server")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}
	listingID := r.PathValue("id")

	if !h.ownsListing(w, r, listingID, userID) {
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
	err := h.db.QueryRowContext(r.Context(), `
		SELECT object_key, sort_order FROM listing_images
		WHERE id = $1 AND listing_id = $2
	`, req.ImageID, listingID).Scan(&objectKey, &sortOrder)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	if err != nil {
		log.Printf("listings: load pending image: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not load image")
		return
	}

	exists, err := h.store.Head(r.Context(), objectKey)
	if err != nil {
		log.Printf("listings: head object: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not verify upload")
		return
	}
	if !exists {
		apierr.Write(w, http.StatusUnprocessableEntity, "upload_not_found", "no uploaded object found for this image; PUT the file before confirming")
		return
	}

	publicURL := h.store.PublicURL(objectKey)
	if _, err := h.db.ExecContext(r.Context(), `
		UPDATE listing_images
		SET status = 'active', url = $1, width = $2, height = $3
		WHERE id = $4 AND listing_id = $5
	`, publicURL, req.Width, req.Height, req.ImageID, listingID); err != nil {
		log.Printf("listings: activate image: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not activate image")
		return
	}

	writeJSON(w, http.StatusOK, Image{
		ID:        req.ImageID,
		URL:       publicURL,
		SortOrder: sortOrder,
		Width:     req.Width,
		Height:    req.Height,
	})
}

// deleteImage serves DELETE /api/v1/listings/{id}/images/{imageId} — owner-only.
// Removes the object from storage (best-effort) and the DB row.
func (h *Handler) deleteImage(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		apierr.Write(w, http.StatusServiceUnavailable, "storage_unavailable", "image storage is not configured on this server")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}
	listingID := r.PathValue("id")
	imageID := r.PathValue("imageId")

	if !h.ownsListing(w, r, listingID, userID) {
		return
	}

	var objectKey string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT object_key FROM listing_images WHERE id = $1 AND listing_id = $2
	`, imageID, listingID).Scan(&objectKey)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	if err != nil {
		log.Printf("listings: load image for delete: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not load image")
		return
	}

	// Best-effort object removal; a leaked object is recoverable, a dangling DB
	// row is the worse outcome, so the row delete is authoritative.
	if err := h.store.Delete(r.Context(), objectKey); err != nil {
		log.Printf("listings: delete object (continuing): %v", err)
	}

	if _, err := h.db.ExecContext(r.Context(),
		`DELETE FROM listing_images WHERE id = $1 AND listing_id = $2`, imageID, listingID); err != nil {
		log.Printf("listings: delete image row: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not delete image")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ownsListing resolves the listing owner and writes the appropriate error
// (404 unknown/removed, 403 non-owner, 500 on db error) when the caller is not
// the owner. Returns true only when userID owns an active listing.
func (h *Handler) ownsListing(w http.ResponseWriter, r *http.Request, listingID, userID string) bool {
	ownerID, _, err := h.listingOwner(r.Context(), listingID)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return false
	}
	if err != nil {
		log.Printf("listings: load listing owner: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not load listing")
		return false
	}
	if ownerID != userID {
		apierr.Write(w, http.StatusForbidden, "forbidden", "you do not own this listing")
		return false
	}
	return true
}

func (h *Handler) maxImageBytes() int64 {
	if h.maxBytes > 0 {
		return h.maxBytes
	}
	return defaultMaxImageBytes
}

// loadImages returns the active images for a listing ordered by sort_order.
func (h *Handler) loadImages(ctx context.Context, listingID string) ([]Image, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT id, COALESCE(url, ''), sort_order, width, height
		FROM listing_images
		WHERE listing_id = $1 AND status = 'active'
		ORDER BY sort_order
	`, listingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]Image, 0)
	for rows.Next() {
		var img Image
		var width, height sql.NullInt64
		if err := rows.Scan(&img.ID, &img.URL, &img.SortOrder, &width, &height); err != nil {
			return nil, err
		}
		if width.Valid {
			v := int(width.Int64)
			img.Width = &v
		}
		if height.Valid {
			v := int(height.Int64)
			img.Height = &v
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

// newUUID returns a random RFC 4122 v4 UUID string. Used for image ids and
// object keys so the key is known before the DB insert (object_key is NOT NULL).
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
