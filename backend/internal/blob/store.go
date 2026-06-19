// Package blob provides the BlobStore interface for object storage operations.
// The R2Store implementation targets Cloudflare R2 (S3-compatible).
package blob

import (
	"context"
	"time"
)

// PresignResult holds the presigned upload URL returned to clients.
type PresignResult struct {
	UploadURL string
	ExpiresAt time.Time
}

// BlobStore abstracts object storage operations. A nil BlobStore means
// image upload is not configured — handlers return 503 gracefully.
type BlobStore interface {
	// PresignPut returns a short-lived presigned PUT URL for the given key.
	PresignPut(ctx context.Context, key, contentType string, maxBytes int64, ttl time.Duration) (PresignResult, error)
	// HeadObject returns true when the object exists in storage.
	HeadObject(ctx context.Context, key string) (bool, error)
	// DeleteObject removes an object from storage.
	DeleteObject(ctx context.Context, key string) error
	// PublicURL derives the public CDN URL for a stored object.
	PublicURL(key string) string
}
