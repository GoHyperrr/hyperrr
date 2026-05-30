package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/GoHyperrr/hyperrr/modules/identity"
	ident "github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Server handles Model Context Protocol (MCP) communication over HTTP/SSE.
type Server struct {
	deps     *registry.Dependencies
	sessions map[string]chan []byte
	mu       sync.RWMutex
}

// NewServer creates a new MCP server instance.
func NewServer(deps *registry.Dependencies) *Server {
	return &Server{
		deps:     deps,
		sessions: make(map[string]chan []byte),
	}
}

// HandleSSE establishes the Server-Sent Events connection.
func (s *Server) HandleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	sessionID := uuid.New().String()
	messageChan := make(chan []byte, 10)

	s.mu.Lock()
	s.sessions[sessionID] = messageChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		close(messageChan)
		logger.Info("MCP session closed", "session_id", sessionID)
	}()

	logger.Info("MCP session established", "session_id", sessionID)

	endpointURL := fmt.Sprintf("/mcp/messages?session_id=%s", sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", string(msg))
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// HandleMessages receives JSON-RPC messages from the agent.
func (s *Server) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	_, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// AUTH: Requirement 4 - API Key Authentication
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
	}

	if apiKey == "" {
		http.Error(w, "unauthorized: api key required", http.StatusUnauthorized)
		return
	}

	// Resolve Actor (Assuming Identity Module is registered and accessible)
	// For simplicity in the gateway, we'll try to get the identity module from the registry.
	var identMod *identity.Module
	for _, mod := range registry.List() {
		if m, ok := mod.(*identity.Module); ok {
			identMod = m
			break
		}
	}

	if identMod == nil {
		http.Error(w, "identity module not found", http.StatusInternalServerError)
		return
	}

	actor, err := identMod.GetActorByAPIKey(r.Context(), apiKey)
	if err != nil {
		http.Error(w, "unauthorized: invalid api key", http.StatusUnauthorized)
		return
	}

	if actor.Type != ident.ActorAIAgent {
		http.Error(w, "unauthorized: actor is not an AI agent", http.StatusForbidden)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON-RPC", http.StatusBadRequest)
		return
	}

	// Dispatch request
	go s.dispatch(r.Context(), sessionID, actor, req)

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) dispatch(ctx context.Context, sessionID string, actor *ident.Actor, req JSONRPCRequest) {
	var resp JSONRPCResponse
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	switch req.Method {
	case "tools/list":
		resp.Result = s.handleToolsList(ctx)
	case "tools/call":
		resp.Result, resp.Error = s.handleToolsCall(ctx, actor, req.Params)
	default:
		resp.Error = &Error{Code: CodeMethodNotFound, Message: "Method not found: " + req.Method}
	}

	s.SendMessage(sessionID, resp)
}

func (s *Server) handleToolsList(ctx context.Context) *ListToolsResult {
	workflows := s.deps.Registry.List()
	var tools []Tool

	for _, wf := range workflows {
		if wf.ExposeToAI {
			inputSchema := wf.InputSchema
			if inputSchema == nil {
				inputSchema = map[string]any{"type": "object"}
			}
			tools = append(tools, Tool{
				Name:        wf.Name,
				Description: wf.Description,
				InputSchema: inputSchema,
			})
		}
	}

	return &ListToolsResult{Tools: tools}
}

func (s *Server) handleToolsCall(ctx context.Context, actor *ident.Actor, params map[string]any) (any, *Error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, &Error{Code: CodeInvalidParams, Message: "Tool name required"}
	}

	wf, err := s.deps.Registry.Get(name)
	if err != nil || !wf.ExposeToAI {
		return nil, &Error{Code: CodeMethodNotFound, Message: "Tool not found or not exposed"}
	}

	args, _ := params["arguments"].(map[string]any)
	
	// Execute the workflow
	execID := "mcp_" + uuid.New().String()
	
	// Inject actor into context
	// Note: We need a way to pass the actor context to Execute.
	// Since Execute takes context, we can just use the provided context.
	
	results, err := s.deps.Runner.Execute(ctx, execID, wf, args)
	if err != nil {
		return CallToolResult{
			IsError: true,
			Content: []Content{{Type: "text", Text: err.Error()}},
		}, nil
	}

	resJSON, _ := json.MarshalIndent(results, "", "  ")
	return CallToolResult{
		Content: []Content{{Type: "text", Text: string(resJSON)}},
	}, nil
}

// SendMessage pushes a message to a specific agent session.
func (s *Server) SendMessage(sessionID string, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	s.mu.RLock()
	ch, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	select {
	case ch <- data:
		return nil
	default:
		return fmt.Errorf("session message buffer full")
	}
}
