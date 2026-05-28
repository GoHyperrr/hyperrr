package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalProvider implements ObjectStorage using the local filesystem.
type LocalProvider struct {
	rootDir string
}

// NewLocalProvider creates a new LocalProvider.
func NewLocalProvider(rootDir string) (*LocalProvider, error) {
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, err
	}
	
	return &LocalProvider{rootDir: absPath}, nil
}

func (p *LocalProvider) Upload(ctx context.Context, path string, data io.Reader) (string, error) {
	fullPath := filepath.Join(p.rootDir, path)
	
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", err
	}
	
	f, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	
	_, err = io.Copy(f, data)
	return path, err
}

func (p *LocalProvider) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(p.rootDir, path))
}

func (p *LocalProvider) Delete(ctx context.Context, path string) error {
	err := os.Remove(filepath.Join(p.rootDir, path))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (p *LocalProvider) GetURL(ctx context.Context, path string) (string, error) {
	// For local, just return a fake file:// URL or similar
	return fmt.Sprintf("file://%s", filepath.Join(p.rootDir, path)), nil
}
