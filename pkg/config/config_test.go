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

	t.Run("Load from YAML file", func(t *testing.T) {
		yamlFile := "hyperrr_test.yml"
		content := "app_name: file-hyperrr\napp_env: test"
		os.WriteFile(yamlFile, []byte(content), 0644)
		defer os.Remove(yamlFile)

		cfg, err := LoadWithFile(yamlFile)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if cfg.AppName != "file-hyperrr" {
			t.Errorf("expected AppName file-hyperrr, got %s", cfg.AppName)
		}
	})

	t.Run("Load with non-existent file expects error", func(t *testing.T) {
		_, err := LoadWithFile("non-existent-file.yml")
		if err == nil {
			t.Errorf("expected error when loading non-existent file, got nil")
		}
	})

	t.Run("Load from environment variable", func(t *testing.T) {
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

	t.Run("Environment Expansion", func(t *testing.T) {
		os.Setenv("TEST_JWT_SECRET", "super-secret")
		os.Setenv("TEST_PORT", "9090")
		defer os.Unsetenv("TEST_JWT_SECRET")
		defer os.Unsetenv("TEST_PORT")

		yamlFile := "hyperrr_test_env.yml"
		content := `
app_name: env-expand-test
server_port: ${TEST_PORT}
db_dsn: ${TEST_JWT_SECRET:fallback-secret}
storage_bucket_url: ${env.UNDEFINED_VAR:fallback-bucket}
`
		os.WriteFile(yamlFile, []byte(content), 0644)
		defer os.Remove(yamlFile)

		cfg, err := LoadWithFile(yamlFile)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if cfg.ServerPort != 9090 {
			t.Errorf("expected expanded ServerPort 9090, got %d", cfg.ServerPort)
		}
		if cfg.DBDSN != "super-secret" {
			t.Errorf("expected expanded DBDSN super-secret, got %q", cfg.DBDSN)
		}
		if cfg.StorageBucketURL != "fallback-bucket" {
			t.Errorf("expected fallback storage bucket, got %q", cfg.StorageBucketURL)
		}
	})

	t.Run("Configuration Validation", func(t *testing.T) {
		yamlFile := "hyperrr_invalid.yml"
		content := `
app_name: ""
app_env: "invalid_env"
server_port: 999999
db_driver: "postgres"
db_dsn: ""
event_bus_provider: "invalid_bus"
`
		os.WriteFile(yamlFile, []byte(content), 0644)
		defer os.Remove(yamlFile)

		_, err := LoadWithFile(yamlFile)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}

		errStr := err.Error()
		expectedErrors := []string{
			"APP_NAME: cannot be empty",
			"APP_ENV: must be one of",
			"SERVER_PORT: must be between",
			"DB_DSN: cannot be empty for database driver 'postgres'",
			"EVENT_BUS_PROVIDER: must be 'inmem' or 'nats'",
		}
		for _, e := range expectedErrors {
			if !contains(errStr, e) {
				t.Errorf("expected validation message to contain %q, got: %s", e, errStr)
			}
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (s != "" && substr != "" && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr))))))
}
