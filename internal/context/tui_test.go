package context

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "charm.land/bubbletea/v2"
)

func TestWorkflowsTUI(t *testing.T) {
	// Pre-populate test lineage run data
	startedAt := time.Now()
	testLineage := &tuiLineage{
		ID:        "run_1",
		Name:      "test.workflow",
		Version:   "1.0",
		State:     "RUNNING",
		StartedAt: startedAt,
		Steps: []*tuiStepLineage{
			{
				StepID:    "step_1",
				State:     "COMPLETED",
				StartedAt: startedAt,
				Attempts:  1,
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "listLineages") {
			resp := map[string]any{
				"data": map[string]any{
					"listLineages": []*tuiLineage{testLineage},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	deps := &registry.Dependencies{
		Config:    &config.Config{},
		ServerURL: ts.URL,
	}

	ctx := context.Background()
	p := &workflowsPage{}

	// Verify Title
	if p.Title() != "Workflows" {
		t.Errorf("expected title 'Workflows', got %s", p.Title())
	}

	// Initialize Page
	cmdVal := p.Init(ctx, deps)
	if cmdVal == nil {
		t.Error("expected non-nil listening command on Init")
	}

	// Verify the workflow run is captured by TUI page
	if len(p.lineages) != 1 {
		t.Fatalf("expected 1 lineage loaded, got %d", len(p.lineages))
	}
	l := p.lineages[0]
	if l.ID != "run_1" || l.Name != "test.workflow" {
		t.Errorf("incorrect lineage values: %+v", l)
	}

	// Test navigation keypresses
	p.Update(tea.KeyPressMsg{Text: "j", Code: 'j'})
	if p.activeRow != 0 {
		t.Errorf("expected active row 0, got %d", p.activeRow)
	}

	// Verify list view rendering
	viewStr := p.View()
	if !strings.Contains(viewStr, "WORKFLOW MONITOR (LIVE)") || !strings.Contains(viewStr, "test.workflow") {
		t.Errorf("unexpected list view rendering: %s", viewStr)
	}

	// Test enter to view details
	p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	p.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if p.detailMode {
		t.Error("expected detail mode to be false after ESC")
	}
}
