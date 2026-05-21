package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRun(t *testing.T) {
	oldTeaRun := teaRun
	defer func() { teaRun = oldTeaRun }()
	teaRun = func(m tea.Model) error { return nil }

	if err := run(); err != nil {
		t.Errorf("run() failed: %v", err)
	}
}
