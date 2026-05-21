package storage

import (
	"context"
	"fmt"
	"io"
)

// S3Provider is a placeholder for an S3-compatible storage provider.
type S3Provider struct {
	bucket string
}

// NewS3Provider creates a new S3Provider.
func NewS3Provider(bucket string) *S3Provider {
	return &S3Provider{bucket: bucket}
}

func (p *S3Provider) Upload(ctx context.Context, path string, data io.Reader) (string, error) {
	return fmt.Sprintf("s3://%s/%s", p.bucket, path), nil
}

func (p *S3Provider) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("s3 open not implemented")
}

func (p *S3Provider) Delete(ctx context.Context, path string) error {
	return nil
}

func (p *S3Provider) GetURL(ctx context.Context, path string) (string, error) {
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", p.bucket, path), nil
}
