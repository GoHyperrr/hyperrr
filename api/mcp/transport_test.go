package mcp

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	ident "github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestMCP_Transport(t *testing.T) {
	deps := &registry.Dependencies{
		Config: &config.Config{
			MCPAuthProviders: []string{"none"},
		},
	}
	server := NewServer(deps)

	t.Run("SSE Connection and Endpoint Event", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/mcp/sse", nil)
		rr := httptest.NewRecorder()

		// Use a channel to wait for the first event
		done := make(chan bool)
		go func() {
			server.HandleSSE(rr, req)
			done <- true
		}()

		// We need to wait a tiny bit for the goroutine to run and write the first event
		// Or better, use a pipe to read the stream
		
		// For simplicity in this test, we'll just check if the headers are set correctly
		// in a synchronous way if possible, or use a more robust SSE testing pattern.
	})

	t.Run("Headers Check", func(t *testing.T) {
		// Placeholder for detailed header logic if needed
	})

	t.Run("Message Posting - Session Validation", func(t *testing.T) {
		// 1. Post without session
		req := httptest.NewRequest("POST", "/mcp/messages", nil)
		rr := httptest.NewRecorder()
		server.HandleMessages(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}

		// 2. Post with invalid session
		req = httptest.NewRequest("POST", "/mcp/messages?session_id=ghost", nil)
		rr = httptest.NewRecorder()
		server.HandleMessages(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

// Helper to test SSE more realistically
func TestMCP_SSE_Integration(t *testing.T) {
	deps := &registry.Dependencies{
		Config: &config.Config{
			MCPAuthProviders: []string{"none"},
		},
	}
	server := NewServer(deps)
	ts := httptest.NewServer(http.HandlerFunc(server.HandleSSE))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read event: %v", err)
	}
	
	if !strings.HasPrefix(line, "event: endpoint") {
		t.Errorf("expected endpoint event, got %s", line)
	}
}

type mockActorResolver struct {
	keyActorMap map[string]*ident.Actor
}

func (m *mockActorResolver) GetActorByAPIKey(ctx context.Context, key string) (*ident.Actor, error) {
	if a, ok := m.keyActorMap[key]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("invalid key")
}

func TestMCP_Authentication(t *testing.T) {
	resolver := &mockActorResolver{
		keyActorMap: map[string]*ident.Actor{
			"valid-key": {
				ID:   "act_test_agent",
				Type: ident.ActorAIAgent,
				Name: "Test Agent",
			},
		},
	}
	reg := &mockWorkflowRegistry{}

	t.Run("Default Auth (apikey) - Fails without key", func(t *testing.T) {
		deps := &registry.Dependencies{
			Config:   &config.Config{MCPAuthProviders: []string{"apikey"}},
			Resolver: resolver,
			Registry: reg,
		}
		server := NewServer(deps)

		// Create session
		sessionID := "sess_1"
		server.mu.Lock()
		server.sessions[sessionID] = &session{
			msgChan: make(chan []byte, 10),
			ctx:     context.Background(),
		}
		server.mu.Unlock()

		req := httptest.NewRequest("POST", "/mcp/messages?session_id="+sessionID, strings.NewReader(`{}`))
		rr := httptest.NewRecorder()
		server.HandleMessages(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 unauthorized, got %d", rr.Code)
		}
	})

	t.Run("Default Auth (apikey) - Succeeds with valid key", func(t *testing.T) {
		deps := &registry.Dependencies{
			Config:   &config.Config{MCPAuthProviders: []string{"apikey"}},
			Resolver: resolver,
			Registry: reg,
		}
		server := NewServer(deps)

		sessionID := "sess_2"
		server.mu.Lock()
		server.sessions[sessionID] = &session{
			msgChan: make(chan []byte, 10),
			ctx:     context.Background(),
		}
		server.mu.Unlock()

		req := httptest.NewRequest("POST", "/mcp/messages?session_id="+sessionID, strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`))
		req.Header.Set("X-API-Key", "valid-key")
		rr := httptest.NewRecorder()
		server.HandleMessages(rr, req)

		// Since tools/list might succeed or return a response, it should not be 401.
		if rr.Code == http.StatusUnauthorized {
			t.Errorf("expected access to be allowed, but got 401 unauthorized")
		}
	})

	t.Run("Bypassed Auth (none) - Succeeds without key", func(t *testing.T) {
		deps := &registry.Dependencies{
			Config:   &config.Config{MCPAuthProviders: []string{"none"}},
			Resolver: resolver,
			Registry: reg,
		}
		server := NewServer(deps)

		sessionID := "sess_3"
		server.mu.Lock()
		server.sessions[sessionID] = &session{
			msgChan: make(chan []byte, 10),
			ctx:     context.Background(),
		}
		server.mu.Unlock()

		req := httptest.NewRequest("POST", "/mcp/messages?session_id="+sessionID, strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`))
		rr := httptest.NewRecorder()
		server.HandleMessages(rr, req)

		if rr.Code == http.StatusUnauthorized {
			t.Errorf("expected access to be allowed, but got 401 unauthorized")
		}
	})
}
