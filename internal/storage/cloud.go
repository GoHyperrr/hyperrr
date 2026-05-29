package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/memblob"
	_ "gocloud.dev/blob/s3blob"
)

// CloudProvider implements ObjectStorage using Go Cloud Development Kit.
// It supports S3, GCS, Azure Blob Storage, and local filesystem via URL schemes.
type CloudProvider struct {
	bucket *blob.Bucket
	url    string
}

// NewCloudProvider creates a new CloudProvider from a bucket URL.
// Example URLs:
// - S3: s3://my-bucket?region=us-west-1
// - GCS: gs://my-bucket
// - Azure: azblob://my-container
// - Local: file:///path/to/dir (use file:///C:/path on Windows)
func NewCloudProvider(ctx context.Context, bucketURL string) (*CloudProvider, error) {
	b, err := blob.OpenBucket(ctx, bucketURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open bucket %s: %w", bucketURL, err)
	}
	return &CloudProvider{bucket: b, url: bucketURL}, nil
}

func (p *CloudProvider) Upload(ctx context.Context, path string, data io.Reader) (string, error) {
	opts := &blob.WriterOptions{
		ContentType: mime.TypeByExtension(filepath.Ext(path)),
	}

	w, err := p.bucket.NewWriter(ctx, path, opts)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(w, data); err != nil {
		w.Close()
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func (p *CloudProvider) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	return p.bucket.NewReader(ctx, path, nil)
}

func (p *CloudProvider) Delete(ctx context.Context, path string) error {
	return p.bucket.Delete(ctx, path)
}

func (p *CloudProvider) GetURL(ctx context.Context, path string) (string, error) {
	// gocloud doesn't have a single way to get a public URL because it depends on the provider's policy.
	// But we can return the path combined with the base URL for reference.
	return fmt.Sprintf("%s/%s", p.url, path), nil
}

func (p *CloudProvider) Close() error {
	return p.bucket.Close()
}
