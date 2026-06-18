// Package storage provides a small, swappable object-storage seam (BlobStore)
// for listing images. The presign→direct-PUT→confirm upload flow keeps large
// image bytes off the API server (cheap on free-tier compute).
//
// The default implementation targets any S3-compatible endpoint via
// aws-sdk-go-v2, so Cloudflare R2 (recommended: zero egress), MinIO, Backblaze
// B2, or AWS S3 are all drop-in via BLOB_* env vars. Swapping to a non-S3
// provider (e.g. Vercel Blob) means writing a new BlobStore implementation;
// nothing else in the codebase depends on S3.
package storage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// BlobStore is the object-storage contract the listings image handlers depend
// on. Implementations are expected to be safe for concurrent use.
type BlobStore interface {
	// PresignPut mints a short-lived presigned PUT URL the client uses to
	// upload bytes directly to storage under key, pinned to contentType.
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	// Head reports whether an object exists at key (used to verify a direct
	// upload landed before activating the DB row).
	Head(ctx context.Context, key string) (exists bool, err error)
	// Delete removes the object at key. Deleting a missing object is not an error.
	Delete(ctx context.Context, key string) error
	// PublicURL returns the public-read URL for key (bucket/CDN base + key).
	PublicURL(key string) string
}

// s3Store is the S3-compatible BlobStore (R2/S3/MinIO/B2).
type s3Store struct {
	client    *s3.Client
	presign   *s3.PresignClient
	bucket    string
	publicURL string // base URL for public-read objects, no trailing slash
}

// NewFromEnv builds a BlobStore from BLOB_* environment variables:
//
//	BLOB_ENDPOINT          S3-compatible endpoint, e.g.
//	                       https://<accountid>.r2.cloudflarestorage.com
//	BLOB_BUCKET            bucket name
//	BLOB_ACCESS_KEY_ID     access key
//	BLOB_SECRET_ACCESS_KEY secret key
//	BLOB_PUBLIC_BASE_URL   public-read base URL for serving (e.g. the bucket's
//	                       r2.dev URL or a CDN custom domain)
//	BLOB_REGION            optional, defaults to "auto" (R2)
//	BLOB_FORCE_PATH_STYLE  optional "true" to force path-style addressing
//
// Returns (nil, nil) when BLOB_ENDPOINT is unset — callers treat a nil store as
// "image uploads disabled" and return 503, mirroring the search/auth graceful
// degradation pattern so the service still boots before storage is provisioned.
func NewFromEnv() (BlobStore, error) {
	endpoint := os.Getenv("BLOB_ENDPOINT")
	if endpoint == "" {
		return nil, nil
	}
	bucket := os.Getenv("BLOB_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("storage: BLOB_BUCKET not set")
	}
	accessKey := os.Getenv("BLOB_ACCESS_KEY_ID")
	secretKey := os.Getenv("BLOB_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("storage: BLOB_ACCESS_KEY_ID and BLOB_SECRET_ACCESS_KEY required")
	}
	publicURL := strings.TrimRight(os.Getenv("BLOB_PUBLIC_BASE_URL"), "/")
	if publicURL == "" {
		return nil, fmt.Errorf("storage: BLOB_PUBLIC_BASE_URL not set")
	}
	region := os.Getenv("BLOB_REGION")
	if region == "" {
		region = "auto" // R2 default
	}
	forcePathStyle := os.Getenv("BLOB_FORCE_PATH_STYLE") == "true"

	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: forcePathStyle,
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		),
	})

	return &s3Store{
		client:    client,
		presign:   s3.NewPresignClient(client),
		bucket:    bucket,
		publicURL: publicURL,
	}, nil
}

func (s *s3Store) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage: presign put: %w", err)
	}
	return req.URL, nil
}

func (s *s3Store) Head(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("storage: head: %w", err)
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("storage: delete: %w", err)
	}
	return nil
}

func (s *s3Store) PublicURL(key string) string {
	return s.publicURL + "/" + key
}

// isNotFound reports whether err is an S3 "object does not exist" error. HEAD
// surfaces this as a 404 with an empty smithy code, so the HTTP status is the
// reliable signal; DeleteObject is idempotent and rarely 404s.
func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	var respErr interface{ HTTPStatusCode() int }
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
		return true
	}
	return false
}

// ExtForContentType maps an allowed image MIME type to a file extension used in
// object keys. Returns "" for unsupported types.
func ExtForContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	default:
		return ""
	}
}

// ParseSizeEnv is a small helper for limit configuration (kept here so the
// storage seam owns its own defaults). Returns def when v is empty or invalid.
func ParseSizeEnv(v string, def int64) int64 {
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
