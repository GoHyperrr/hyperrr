package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("Load default shortcut", func(t *testing.T) {
		_, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
	})

	t.Run("Load defaults", func(t *testing.T) {
		cfg, err := LoadWithFile("")
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if cfg.AppName != "hyperrr" {
			t.Errorf("expected default AppName hyperrr, got %s", cfg.AppName)
		}
	})

	t.Run("Load from file", func(t *testing.T) {
		envFile := ".env.test"
		content := "APP_NAME=file-hyperrr\nAPP_ENV=test"
		os.WriteFile(envFile, []byte(content), 0644)
		defer os.Remove(envFile)

		cfg, err := LoadWithFile(envFile)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if cfg.AppName != "file-hyperrr" {
			t.Errorf("expected AppName file-hyperrr, got %s", cfg.AppName)
		}
	})

	t.Run("Load with invalid file", func(t *testing.T) {
		_, err := LoadWithFile("non-existent-file.env")
		// ConfigFileNotFoundError is ignored in LoadWithFile, so it should succeed with defaults
		if err != nil {
			t.Errorf("expected success with defaults for non-existent file, got error: %v", err)
		}
	})

	t.Run("Load with directory", func(t *testing.T) {
		// Passing a directory to SetConfigFile causes ReadInConfig to fail on most systems
		tmpDir, _ := os.MkdirTemp("", "config_test")
		defer os.RemoveAll(tmpDir)

		_, err := LoadWithFile(tmpDir)
		if err == nil {
			t.Errorf("expected error when loading from directory, got nil")
		}
	})

	t.Run("Load from environment", func(t *testing.T) {
		os.Setenv("APP_NAME", "env-hyperrr")
		defer os.Unsetenv("APP_NAME")

		cfg, err := LoadWithFile("")
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if cfg.AppName != "env-hyperrr" {
			t.Errorf("expected AppName env-hyperrr, got %s", cfg.AppName)
		}
	})
}
