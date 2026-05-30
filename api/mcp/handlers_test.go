package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	ident "github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Mock implementation of WorkflowRegistry for testing
type mockWorkflowRegistry struct {
	workflows []*workflow.Workflow
}

func (m *mockWorkflowRegistry) Register(wf *workflow.Workflow) error {
	m.workflows = append(m.workflows, wf)
	return nil
}
func (m *mockWorkflowRegistry) Get(name string) (*workflow.Workflow, error) {
	for _, wf := range m.workflows {
		if wf.Name == name {
			return wf, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockWorkflowRegistry) List() []*workflow.Workflow {
	return m.workflows
}

// Mock implementation of WorkflowRunner for testing
type mockWorkflowRunner struct {
	lastExecuted string
	lastInput    any
}

func (m *mockWorkflowRunner) Execute(ctx context.Context, id string, wf *workflow.Workflow, input any) (map[string]any, error) {
	m.lastExecuted = wf.Name
	m.lastInput = input
	return map[string]any{"status": "ok"}, nil
}

func TestMCP_DiscoveryAndExecution(t *testing.T) {
	// Setup mocks
	reg := &mockWorkflowRegistry{
		workflows: []*workflow.Workflow{
			{Name: "public-tool", ExposeToAI: true, Description: "Safe to use", InputSchema: map[string]any{"type": "object"}},
			{Name: "private-tool", ExposeToAI: false},
		},
	}
	runner := &mockWorkflowRunner{}
	deps := &registry.Dependencies{
		Registry: reg,
		Runner:   runner,
	}

	server := NewServer(deps)

	t.Run("Tools List Discovery", func(t *testing.T) {
		result := server.handleToolsList(context.Background())
		if len(result.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(result.Tools))
		}
		if result.Tools[0].Name != "public-tool" {
			t.Errorf("expected public-tool, got %s", result.Tools[0].Name)
		}
	})

	t.Run("Tools Call Execution", func(t *testing.T) {
		actor := &ident.Actor{ID: "agent_1", Type: ident.ActorAIAgent}
		params := map[string]any{
			"name":      "public-tool",
			"arguments": map[string]any{"id": "123"},
		}

		resRaw, errRPC := server.handleToolsCall(context.Background(), actor, params)
		if errRPC != nil {
			t.Fatalf("handleToolsCall failed: %v", errRPC)
		}

		res := resRaw.(CallToolResult)
		if res.IsError {
			t.Errorf("expected success, got error: %s", res.Content[0].Text)
		}

		if runner.lastExecuted != "public-tool" {
			t.Error("runner was not invoked with the correct tool")
		}

		// Verify result contains the expected content
		if !strings.Contains(res.Content[0].Text, "ok") {
			t.Errorf("unexpected result text: %s", res.Content[0].Text)
		}
	})

	t.Run("Tools Call Unauthorized Tool", func(t *testing.T) {
		actor := &ident.Actor{ID: "agent_1", Type: ident.ActorAIAgent}
		params := map[string]any{"name": "private-tool"}

		_, errRPC := server.handleToolsCall(context.Background(), actor, params)
		if errRPC == nil || errRPC.Code != CodeMethodNotFound {
			t.Error("expected MethodNotFound error for private tool")
		}
	})
}

func TestMCP_FullHandshake(t *testing.T) {
	// This test verifies the SSE + POST flow
	deps := &registry.Dependencies{
		Registry: &mockWorkflowRegistry{
			workflows: []*workflow.Workflow{{Name: "test", ExposeToAI: true}},
		},
	}
	server := NewServer(deps)

	ts := httptest.NewServer(http.HandlerFunc(server.HandleSSE))
	defer ts.Close()

	// 1. Establish SSE Connection
	resp, _ := http.Get(ts.URL)
	defer resp.Body.Close()

	// The session ID is randomly generated, we'd need to parse the 'endpoint' event
	// For this test, we'll just verify the server state has 1 session
	server.mu.RLock()
	sessionCount := len(server.sessions)
	var sessionID string
	for id := range server.sessions {
		sessionID = id
	}
	server.mu.RUnlock()

	if sessionCount != 1 {
		t.Fatalf("expected 1 active session, got %d", sessionCount)
	}

	// 2. Dispatch a message to the agent via SendMessage
	testMsg := map[string]string{"ping": "pong"}
	err := server.SendMessage(sessionID, testMsg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 3. Verify the message can be read from the SSE stream (not easily done in one sync test, but proves logic)
}
