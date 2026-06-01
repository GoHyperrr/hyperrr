package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/api/middleware"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
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
	lastActor    *ident.Actor
}

func (m *mockWorkflowRunner) Execute(ctx context.Context, id string, wf *workflow.Workflow, input any) (map[string]any, error) {
	m.lastExecuted = wf.Name
	m.lastInput = input
	if a, ok := middleware.ForContext(ctx); ok {
		m.lastActor = a
	}
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
		if len(result.Tools) != 12 {
			t.Errorf("expected 12 tools, got %d", len(result.Tools))
		}
		foundPublic := false
		for _, tool := range result.Tools {
			if tool.Name == "public-tool" {
				foundPublic = true
				break
			}
		}
		if !foundPublic {
			t.Errorf("expected public-tool in the list of tools")
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

		if runner.lastActor == nil || runner.lastActor.ID != "agent_1" {
			t.Errorf("expected actor ID agent_1, got %v", runner.lastActor)
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
}

type mockResourceProvider struct{}

func (m *mockResourceProvider) ID() string { return "mock.resources" }
func (m *mockResourceProvider) Init(ctx context.Context, deps *registry.Dependencies) error { return nil }
func (m *mockResourceProvider) Models() []any { return nil }
func (m *mockResourceProvider) Handlers() map[string]workflow.TaskHandler { return nil }
func (m *mockResourceProvider) Shutdown(ctx context.Context) error { return nil }
func (m *mockResourceProvider) ListResources(ctx context.Context) ([]registry.MCPResource, error) {
	return []registry.MCPResource{
		{
			URI:         "mock://test-resource",
			Name:        "Test Resource",
			Description: "Mock description",
			MimeType:    "application/json",
		},
	}, nil
}
func (m *mockResourceProvider) ReadResource(ctx context.Context, uri string) (string, error) {
	if uri == "mock://test-resource" {
		return `{"value":"hello"}`, nil
	}
	return "", fmt.Errorf("not found")
}

func TestMCP_Resources(t *testing.T) {
	// Register mock provider
	prov := &mockResourceProvider{}
	registry.Register(prov)

	bus := eventbus.NewInMemBus()
	deps := &registry.Dependencies{
		EventBus: bus,
	}

	server := NewServer(deps)

	t.Run("Resources List", func(t *testing.T) {
		resList := server.handleResourcesList(context.Background())
		found := false
		for _, r := range resList.Resources {
			if r.URI == "mock://test-resource" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected mock://test-resource in resources list")
		}
	})

	t.Run("Resources Read Success", func(t *testing.T) {
		params := map[string]any{"uri": "mock://test-resource"}
		resRaw, errRPC := server.handleResourcesRead(context.Background(), params)
		if errRPC != nil {
			t.Fatalf("unexpected error: %v", errRPC)
		}
		res := resRaw.(*ReadResourceResult)
		if len(res.Contents) != 1 || res.Contents[0].Text != `{"value":"hello"}` {
			t.Errorf("unexpected content: %v", res.Contents)
		}
	})

	t.Run("Resources Read Failure", func(t *testing.T) {
		params := map[string]any{"uri": "mock://unknown-resource"}
		_, errRPC := server.handleResourcesRead(context.Background(), params)
		if errRPC == nil || errRPC.Code != CodeInvalidParams {
			t.Error("expected InvalidParams error for unknown resource")
		}
	})

	t.Run("Resources Subscribe and Event Trigger", func(t *testing.T) {
		sessionID := "sess_test_1"
		messageChan := make(chan []byte, 10)
		sessionCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		server.mu.Lock()
		server.sessions[sessionID] = &session{
			msgChan: messageChan,
			ctx:     sessionCtx,
			cancel:  cancel,
		}
		server.mu.Unlock()

		// Subscribe to product updates
		params := map[string]any{"uri": "product://prod_123"}
		_, errRPC := server.handleResourcesSubscribe(context.Background(), sessionID, params)
		if errRPC != nil {
			t.Fatalf("subscribe failed: %v", errRPC)
		}

		// Verify subscription registered
		server.mu.RLock()
		subbed := server.subscriptions["product://prod_123"][sessionID]
		server.mu.RUnlock()
		if !subbed {
			t.Fatal("expected session to be subscribed")
		}

		// Publish product update event
		ev := eventbus.Event{
			ID:   "evt_prod_upd_1",
			Type: "product.updated",
			Payload: map[string]any{
				"product_id": "prod_123",
			},
		}
		_ = bus.Publish(context.Background(), ev)

		// Check if we received the notification from the message channel
		select {
		case msgBytes := <-messageChan:
			var msg map[string]any
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				t.Fatalf("failed to unmarshal message: %v", err)
			}
			if msg["method"] != "notifications/resources/updated" {
				t.Errorf("expected notification method notifications/resources/updated, got %v", msg["method"])
			}
			paramsMap := msg["params"].(map[string]any)
			if paramsMap["uri"] != "product://prod_123" {
				t.Errorf("expected notified URI product://prod_123, got %v", paramsMap["uri"])
			}
		default:
			t.Error("expected notification message, got none")
		}

		// Unsubscribe
		_, errRPC = server.handleResourcesUnsubscribe(context.Background(), sessionID, params)
		if errRPC != nil {
			t.Fatalf("unsubscribe failed: %v", errRPC)
		}

		server.mu.RLock()
		_, subbed = server.subscriptions["product://prod_123"][sessionID]
		server.mu.RUnlock()
		if subbed {
			t.Fatal("expected session to be unsubscribed")
		}
	})
}
