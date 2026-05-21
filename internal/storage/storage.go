package storage

import (
	"context"
	"io"
)

// ObjectStorage defines the contract for OS-level file handling.
type ObjectStorage interface {
	// Upload saves a file to storage and returns its path or URL.
	Upload(ctx context.Context, path string, data io.Reader) (string, error)
	
	// Open retrieves a file from storage.
	Open(ctx context.Context, path string) (io.ReadCloser, error)
	
	// Delete removes a file from storage.
	Delete(ctx context.Context, path string) error
	
	// GetURL returns a public or signed URL for a file.
	GetURL(ctx context.Context, path string) (string, error)
}
