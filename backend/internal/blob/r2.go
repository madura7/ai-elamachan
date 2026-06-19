package blob

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Store implements BlobStore against Cloudflare R2 (S3-compatible API).
// Public-read bucket: BLOB_PUBLIC_BASE_URL/<key> is the CDN URL.
type R2Store struct {
	presigner *s3.PresignClient
	client    *s3.Client
	bucket    string
	baseURL   string // e.g. "https://pub-xxx.r2.dev" — no trailing slash
}

// NewFromEnv constructs an R2Store from BLOB_* env vars.
// Returns (nil, nil) when BLOB_ENDPOINT is unset — callers treat nil as
// "image upload disabled".
func NewFromEnv() (*R2Store, error) {
	endpoint := os.Getenv("BLOB_ENDPOINT")
	if endpoint == "" {
		return nil, nil
	}
	bucket := os.Getenv("BLOB_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("blob: BLOB_BUCKET required when BLOB_ENDPOINT is set")
	}
	accessKey := os.Getenv("BLOB_ACCESS_KEY_ID")
	secret := os.Getenv("BLOB_SECRET_ACCESS_KEY")
	if accessKey == "" || secret == "" {
		return nil, fmt.Errorf("blob: BLOB_ACCESS_KEY_ID and BLOB_SECRET_ACCESS_KEY required")
	}
	baseURL := strings.TrimRight(os.Getenv("BLOB_PUBLIC_BASE_URL"), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("blob: BLOB_PUBLIC_BASE_URL required")
	}

	cfg := aws.Config{
		Region: "auto",
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint, SigningRegion: "auto"}, nil
			},
		),
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secret, ""),
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &R2Store{
		presigner: s3.NewPresignClient(client),
		client:    client,
		bucket:    bucket,
		baseURL:   baseURL,
	}, nil
}

// PresignPut returns a short-lived presigned PUT URL for the given key.
func (r *R2Store) PresignPut(ctx context.Context, key, contentType string, _ int64, ttl time.Duration) (PresignResult, error) {
	req, err := r.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return PresignResult{}, fmt.Errorf("blob: presign put %q: %w", key, err)
	}
	return PresignResult{
		UploadURL: req.URL,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}, nil
}

// HeadObject returns true when the object exists in the bucket.
func (r *R2Store) HeadObject(ctx context.Context, key string) (bool, error) {
	_, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// NoSuchKey / 404 → object does not exist
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("blob: head %q: %w", key, err)
	}
	return true, nil
}

// DeleteObject removes the object from the bucket.
func (r *R2Store) DeleteObject(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("blob: delete %q: %w", key, err)
	}
	return nil
}

// PublicURL returns the public CDN URL for a stored object key.
func (r *R2Store) PublicURL(key string) string {
	return r.baseURL + "/" + key
}
