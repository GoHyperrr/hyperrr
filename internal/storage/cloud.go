package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// CloudProvider implements ObjectStorage using local filesystem and memory fallbacks,
// avoiding external heavy cloud SDK dependencies.
type CloudProvider struct {
	mu      sync.RWMutex
	mem     map[string][]byte
	url     string
	isMem   bool
	baseDir string
}

// NewCloudProvider creates a new CloudProvider from a bucket URL.
// Example URLs:
// - Mem: mem://
// - Local: file:///path/to/dir (use file:///C:/path on Windows)
// - Fallback relative/absolute directory path: ./uploads
func NewCloudProvider(ctx context.Context, bucketURL string) (*CloudProvider, error) {
	u, err := url.Parse(bucketURL)
	if err != nil {
		return nil, fmt.Errorf("invalid storage URL: %w", err)
	}

	p := &CloudProvider{
		url: bucketURL,
	}

	if u.Scheme == "mem" {
		p.isMem = true
		p.mem = make(map[string][]byte)
	} else if u.Scheme == "file" {
		dir := u.Path
		if os.PathSeparator == '\\' {
			// On Windows, remove leading slash if it precedes a drive letter
			if len(dir) > 2 && dir[0] == '/' && dir[2] == ':' {
				dir = dir[1:]
			}
			dir = filepath.FromSlash(dir)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create storage directory %s: %w", dir, err)
		}
		p.baseDir = dir
	} else {
		dir := filepath.Clean(bucketURL)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create storage directory %s: %w", dir, err)
		}
		p.baseDir = dir
	}

	return p, nil
}

// Upload saves a file to storage and returns its path or URL.
func (p *CloudProvider) Upload(ctx context.Context, path string, data io.Reader) (string, error) {
	if p.isMem {
		buf, err := io.ReadAll(data)
		if err != nil {
			return "", err
		}
		p.mu.Lock()
		p.mem[path] = buf
		p.mu.Unlock()
		return path, nil
	}

	fullPath := filepath.Join(p.baseDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		return "", err
	}

	return path, nil
}

// Open retrieves a file from storage.
func (p *CloudProvider) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	if p.isMem {
		p.mu.RLock()
		buf, ok := p.mem[path]
		p.mu.RUnlock()
		if !ok {
			return nil, os.ErrNotExist
		}
		return io.NopCloser(bytes.NewReader(buf)), nil
	}

	fullPath := filepath.Join(p.baseDir, path)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Delete removes a file from storage.
func (p *CloudProvider) Delete(ctx context.Context, path string) error {
	if p.isMem {
		p.mu.Lock()
		delete(p.mem, path)
		p.mu.Unlock()
		return nil
	}

	fullPath := filepath.Join(p.baseDir, path)
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// GetURL returns a public or signed URL for a file.
func (p *CloudProvider) GetURL(ctx context.Context, path string) (string, error) {
	u, err := url.Parse(p.url)
	if err != nil {
		return "", err
	}
	normalizedPath := filepath.ToSlash(path)
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(normalizedPath, "/")
	return u.String(), nil
}

// Close releases any resources.
func (p *CloudProvider) Close() error {
	return nil
}
