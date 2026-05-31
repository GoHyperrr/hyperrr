package main

import (
	"fmt"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRun(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	oldTeaRun := teaRun
	defer func() { teaRun = oldTeaRun }()
	teaRun = func(m tea.Model) error { return nil }

	if err := run(); err != nil {
		t.Errorf("run() failed: %v", err)
	}
}

func TestMain(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	oldTeaRun := teaRun
	defer func() { teaRun = oldTeaRun }()
	teaRun = func(m tea.Model) error { return nil }

	oldExit := osExit
	defer func() { osExit = oldExit }()
	osExit = func(code int) {}

	main()
}

func TestMainError(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	oldTeaRun := teaRun
	defer func() { teaRun = oldTeaRun }()
	teaRun = func(m tea.Model) error { return fmt.Errorf("error") }

	oldExit := osExit
	defer func() { osExit = oldExit }()
	called := false
	osExit = func(code int) { called = true }

	main()
	if !called {
		t.Error("expected osExit to be called")
	}
}
