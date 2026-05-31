package main

import (
	"fmt"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMainCLI(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	// Store original globals and restore after test runs
	oldExit := osExit
	oldAppRun := appRun
	oldTeaRun := teaRun
	defer func() {
		osExit = oldExit
		appRun = oldAppRun
		teaRun = oldTeaRun
	}()

	// Mock functions
	osExit = func(err error) {}
	appRun = func() error { return nil }
	teaRun = func(m tea.Model) error { return nil }

	t.Run("no arguments prints help", func(t *testing.T) {
		os.Args = []string{"hyperrr"}
		if err := run(); err != nil {
			t.Errorf("expected no error for empty args, got: %v", err)
		}
	})

	t.Run("help command prints help", func(t *testing.T) {
		os.Args = []string{"hyperrr", "help"}
		if err := run(); err != nil {
			t.Errorf("expected no error for help, got: %v", err)
		}
	})

	t.Run("admin command starts TUI", func(t *testing.T) {
		os.Args = []string{"hyperrr", "admin", "--server", "http://localhost:9999"}
		if err := run(); err != nil {
			t.Errorf("expected run to succeed, got: %v", err)
		}
	})

	t.Run("server command starts core server", func(t *testing.T) {
		os.Args = []string{"hyperrr", "server"}
		appRunCalled := false
		appRun = func() error {
			appRunCalled = true
			return nil
		}
		if err := run(); err != nil {
			t.Errorf("expected run to succeed, got: %v", err)
		}
		if !appRunCalled {
			t.Error("expected appRun to be called")
		}
	})

	t.Run("invalid command returns error", func(t *testing.T) {
		os.Args = []string{"hyperrr", "unknown"}
		if err := run(); err == nil {
			t.Error("expected error for unknown command, got nil")
		}
	})

	t.Run("main executes successfully on command success", func(t *testing.T) {
		os.Args = []string{"hyperrr", "help"}
		main()
	})

	t.Run("main calls exit on command failure", func(t *testing.T) {
		os.Args = []string{"hyperrr", "server"}
		appRun = func() error { return fmt.Errorf("forced server boot failure") }
		called := false
		osExit = func(err error) { called = true }

		main()
		if !called {
			t.Error("expected osExit to be called on error")
		}
	})
}
