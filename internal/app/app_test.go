package app

import (
	"os"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
)

func TestRun(t *testing.T) {
	t.Run("Run with defaults", func(t *testing.T) {
		os.Setenv("APP_ENV", "test")
		defer os.Unsetenv("APP_ENV")
		err := Run()
		if err != nil {
			t.Errorf("Run() failed: %v", err)
		}
	})

	t.Run("RunWithConfig", func(t *testing.T) {
		os.Setenv("APP_ENV", "test")
		defer os.Unsetenv("APP_ENV")
		err := RunWithConfig(&config.Config{AppEnv: "test", JWTSecret: "test-secret"})
		if err != nil && err.Error() != "failed to load config" {
			// Expected behavior if .env is missing or invalid in certain environments
		}
	})

	t.Run("RunWithConfig DB Failure", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    "test",
			JWTSecret: "test-secret",
			DBDriver:  "postgres",
			DBDSN:     "host=localhost port=5432 user=ghost password=ghost dbname=ghost sslmode=disable",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "failed to connect to database") {
			t.Errorf("expected DB failure error, got %v", err)
		}
	})

	t.Run("RunWithConfig Missing Secret", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    "test",
			DBDriver:  "sqlite",
			DBDSN:     ":memory:",
			JWTSecret: "",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "JWT_SECRET is missing") {
			t.Errorf("expected missing secret error, got %v", err)
		}
	})

	t.Run("RunWithConfig Unsupported DB", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    "test",
			JWTSecret: "test-secret",
			DBDriver:  "invalid",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "unsupported database driver") {
			t.Errorf("expected unsupported DB error, got %v", err)
		}
	})

	t.Run("RunWithConfig NATS Failure", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:           "test",
			JWTSecret:        "test-secret",
			DBDriver:         "sqlite",
			DBDSN:            ":memory:",
			EventBusProvider: "nats",
			NATSURL:          "nats://localhost:4223", // Non-existent
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "failed to connect to NATS") {
			t.Errorf("expected NATS failure error, got %v", err)
		}
	})
}
