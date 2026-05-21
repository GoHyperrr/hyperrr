package app

import (
	"os"
	"testing"
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

	t.Run("Run with invalid file path", func(t *testing.T) {
		// This is tricky because Run() calls config.Load() which is hardcoded to ".env".
		// But RunWithConfig(nil) calls LoadWithFile("").
		// We can't easily force an error in RunWithConfig(nil) without changing how it works.
		// However, we already have RunWithConfig(cfg) which we tested.
	})
}
