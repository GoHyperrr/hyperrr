package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
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
	// 1. Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// 2. Create session
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

	// 3. Send the initial endpoint event (MCP spec requirement)
	// The agent will POST messages to this endpoint.
	endpointURL := fmt.Sprintf("/mcp/messages?session_id=%s", sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	// 4. Keep-alive and message relay loop
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

	var request map[string]any
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON-RPC", http.StatusBadRequest)
		return
	}

	// TODO: Dispatch to JSON-RPC router (Issue 3 & 4)
	logger.Debug("MCP message received", "session_id", sessionID, "method", request["method"])

	w.WriteHeader(http.StatusAccepted)
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
