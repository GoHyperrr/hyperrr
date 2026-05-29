package storage

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("Cloud Provider (In-Memory)", func(t *testing.T) {
		// Go Cloud DK supports mem:// for testing
		cp, err := NewCloudProvider(ctx, "mem://")
		if err != nil {
			t.Fatalf("failed to create cloud provider: %v", err)
		}
		defer cp.Close()

		content := "cloud data"
		path := "cloud.txt"

		_, err = cp.Upload(ctx, path, strings.NewReader(content))
		if err != nil {
			t.Fatalf("cloud upload failed: %v", err)
		}

		rc, err := cp.Open(ctx, path)
		if err != nil {
			t.Fatalf("cloud open failed: %v", err)
		}
		defer rc.Close()

		data, _ := io.ReadAll(rc)
		if string(data) != content {
			t.Errorf("expected %s, got %s", content, string(data))
		}

		err = cp.Delete(ctx, path)
		if err != nil {
			t.Errorf("cloud delete failed: %v", err)
		}
	})

	t.Run("Module Integration Cloud", func(t *testing.T) {
		mod := NewModule()
		deps := &registry.Dependencies{
			Config: &config.Config{
				StorageBucketURL: "mem://test-bucket",
			},
		}

		if err := mod.Init(ctx, deps); err != nil {
			t.Fatalf("init failed: %v", err)
		}
		if mod.Provider() == nil {
			t.Error("missing provider")
		}
	})

	t.Run("Module Default (In-Memory)", func(t *testing.T) {
		mod := NewModule()
		deps := &registry.Dependencies{
			Config: &config.Config{}, // No bucket URL provided
		}

		if err := mod.Init(ctx, deps); err != nil {
			t.Fatalf("init failed: %v", err)
		}
		
		url, _ := mod.Provider().GetURL(ctx, "test")
		if !strings.HasPrefix(url, "mem://") {
			t.Errorf("expected mem:// prefix for default storage, got %s", url)
		}
	})
}
