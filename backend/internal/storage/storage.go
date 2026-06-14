// Package storage abstracts where listing images are persisted.
// Local (filesystem) is used in dev; swap to an S3 implementation for prod.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store is the image persistence interface.
type Store interface {
	// Put writes content at key. The caller constructs the public URL from the key.
	Put(ctx context.Context, key string, r io.Reader, contentType string) error
}

// Local writes images to a directory on disk.
// Images are served via a static file handler mounted at imageBaseURL by the API process.
type Local struct {
	dir string
}

// NewLocal returns storage that writes images into dir.
func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("storage: mkdir %s: %w", dir, err)
	}
	return &Local{dir: dir}, nil
}

func (l *Local) Put(_ context.Context, key string, r io.Reader, _ string) error {
	dst := filepath.Join(l.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("storage: mkdir: %w", err)
	}
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("storage: create %s: %w", dst, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage: write: %w", err)
	}
	return nil
}
