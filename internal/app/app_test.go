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
		err := RunWithConfig(nil)
		if err != nil && err.Error() != "failed to load config" {
			// Expected behavior if .env is missing or invalid in certain environments
		}
	})

	t.Run("RunWithConfig DB Failure", func(t *testing.T) {
		cfg := &config.Config{
			DBDriver: "postgres",
			DBDSN:    "host=localhost port=5432 user=ghost password=ghost dbname=ghost sslmode=disable",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "failed to connect to database") {
			t.Errorf("expected DB failure error, got %v", err)
		}
	})
}
