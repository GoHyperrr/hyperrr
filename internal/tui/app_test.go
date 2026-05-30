package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTUI(t *testing.T) {
	t.Run("Update and View", func(t *testing.T) {
		m := NewModel(context.Background(), nil)
		
		// Initial view
		view := m.View()
		if !strings.Contains(view, "hyperrr Mission Control") {
			t.Errorf("unexpected initial view: %s", view)
		}

		// Update with event
		msg := WorkflowMsg{ID: "wf1", Name: "Test Workflow", State: workflow.StateRunning, Step: "step1"}
		newModel, cmd := m.Update(msg)
		m = newModel.(*Model)
		if cmd != nil {
			t.Error("expected nil cmd for WorkflowMsg")
		}

		if len(m.workflows) != 1 || m.workflows["wf1"].State != workflow.StateRunning {
			t.Error("workflow state not updated")
		}

		// View after update
		view = m.View()
		if !strings.Contains(view, workflow.StateRunning) || !strings.Contains(view, "wf1") {
			t.Errorf("unexpected updated view: %s", view)
		}
		
		// Update with waiting state
		m.Update(WorkflowMsg{ID: "wf1", State: workflow.StateWaitingHuman})
		view = m.View()
		if !strings.Contains(view, workflow.StateWaitingHuman) {
			t.Error("expected WAITING_HUMAN in view")
		}
	})

	t.Run("Quit command", func(t *testing.T) {
		m := NewModel(context.Background(), nil)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		if cmd() != tea.Quit() {
			t.Error("expected Quit cmd")
		}
	})
	
	t.Run("Init and Subscribe", func(t *testing.T) {
		m := NewModel(context.Background(), nil)
		cmd := m.Init()
		if cmd == nil {
			t.Fatal("expected non-nil Init cmd")
		}
		msg := cmd()
		if msg != nil {
			t.Error("expected nil msg for simulated subscribe")
		}
	})
}
