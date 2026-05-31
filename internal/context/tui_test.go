package context

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "github.com/charmbracelet/bubbletea"
)

func TestWorkflowsTUI(t *testing.T) {
	// Initialize in-memory database
	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    ":memory:",
	}
	database, err := db.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to memory db: %v", err)
	}

	// Auto-migrate Lineage schema
	err = database.AutoMigrate(&LineageModel{})
	if err != nil {
		t.Fatalf("failed to auto-migrate lineage: %v", err)
	}

	// Setup event bus
	bus := eventbus.NewInMemBus()

	// Register module and setup projector
	m := NewModule()
	deps := &registry.Dependencies{
		DB:       database,
		EventBus: bus,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = m.Init(ctx, deps)
	if err != nil {
		t.Fatalf("failed to init context module: %v", err)
	}

	// Register Module globally to let page resolve it
	registry.Register(m)

	p := &workflowsPage{}

	// Verify Title
	if p.Title() != "Workflows" {
		t.Errorf("expected title 'Workflows', got %s", p.Title())
	}

	// Initialize TUI page
	cmdVal := p.Init(ctx, deps)
	if cmdVal == nil {
		t.Error("expected non-nil listening command on Init")
	}

	// Trigger a workflow start event
	event := eventbus.Event{
		ID:        "evt_1",
		Type:      workflow.EventWorkflowStarted,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"id":      "run_1",
			"name":    "test.workflow",
			"version": "1.0",
		},
	}
	err = bus.Publish(ctx, event)
	if err != nil {
		t.Fatalf("failed to publish workflow start event: %v", err)
	}

	// Update page with the event
	resPage, nextCmd := p.Update(event)
	p = resPage.(*workflowsPage)
	if nextCmd == nil {
		t.Error("expected non-nil command returned from Update on event")
	}

	// Verify the workflow run is captured by TUI page
	if len(p.lineages) != 1 {
		t.Fatalf("expected 1 lineage loaded, got %d", len(p.lineages))
	}
	l := p.lineages[0]
	if l.GetID() != "run_1" || l.GetName() != "test.workflow" {
		t.Errorf("incorrect lineage values: %+v", l)
	}

	// Test navigation keypresses
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if p.activeRow != 0 {
		t.Errorf("expected active row 0, got %d", p.activeRow)
	}

	// Verify list view rendering
	viewStr := p.View()
	if !strings.Contains(viewStr, "WORKFLOW MONITOR (LIVE)") || !strings.Contains(viewStr, "test.workflow") {
		t.Errorf("unexpected list view rendering: %s", viewStr)
	}

	// Test enter to view details
	p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.detailMode {
		t.Error("expected detail mode to be true after hitting Enter")
	}
	if p.selectedID != "run_1" {
		t.Errorf("expected selected run ID 'run_1', got %s", p.selectedID)
	}

	// Verify detail view rendering
	detailViewStr := p.View()
	if !strings.Contains(detailViewStr, "WORKFLOW RUN DETAILS: run_1") || !strings.Contains(detailViewStr, "EXECUTION STEPS") {
		t.Errorf("unexpected detail view rendering: %s", detailViewStr)
	}

	// Test esc to return to list
	p.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if p.detailMode {
		t.Error("expected detail mode to be false after ESC")
	}
}
