package storage

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestLocalProvider(t *testing.T) {
	tmpDir := "test_storage"
	defer os.RemoveAll(tmpDir)

	p, err := NewLocalProvider(tmpDir)
	if err != nil {
		t.Fatalf("failed to create local provider: %v", err)
	}

	ctx := context.Background()

	t.Run("Upload and Open", func(t *testing.T) {
		content := "hello world"
		path := "test.txt"
		
		_, err := p.Upload(ctx, path, strings.NewReader(content))
		if err != nil {
			t.Fatalf("failed to upload: %v", err)
		}

		rc, err := p.Open(ctx, path)
		if err != nil {
			t.Fatalf("failed to open: %v", err)
		}
		defer rc.Close()

		data_bytes, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}

		if string(data_bytes) != content {
			t.Errorf("expected %s, got %s", content, string(data_bytes))
		}
	})

	t.Run("GetURL", func(t *testing.T) {
		url, err := p.GetURL(ctx, "test.txt")
		if err != nil {
			t.Fatalf("failed to get url: %v", err)
		}
		if !strings.HasPrefix(url, "file://") {
			t.Errorf("expected file:// prefix, got %s", url)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := p.Delete(ctx, "test.txt")
		if err != nil {
			t.Fatalf("failed to delete: %v", err)
		}

		_, err = p.Open(ctx, "test.txt")
		if err == nil {
			t.Error("expected error opening deleted file")
		}
	})
	
	t.Run("S3 Provider Stub", func(t *testing.T) {
		p := NewS3Provider("my-bucket")
		url, _ := p.GetURL(ctx, "test.png")
		if !strings.Contains(url, "my-bucket.s3.amazonaws.com") {
			t.Error("invalid S3 URL")
		}
		p.Upload(ctx, "test", nil)
		p.Delete(ctx, "test")
		_, err := p.Open(ctx, "test")
		if err == nil {
			t.Error("expected error for s3 open stub")
		}
	})

	t.Run("Module Integration", func(t *testing.T) {
		mod := NewModule()
		if mod.ID() != "core.storage" {
			t.Error("invalid ID")
		}
		if err := mod.Init(ctx, nil); err != nil {
			t.Fatalf("init failed: %v", err)
		}
		if mod.Provider() == nil {
			t.Error("missing provider")
		}
		if len(mod.Models()) != 0 {
			t.Error("unexpected models")
		}
		if len(mod.Handlers()) != 0 {
			t.Error("unexpected handlers")
		}
		// Clean up the storage dir created by mod.Init
		os.RemoveAll("storage")
	})
}
