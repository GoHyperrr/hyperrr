package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/GoHyperrr/hyperrr/api/middleware"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	ident "github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

type session struct {
	msgChan chan []byte
	ctx     context.Context
	cancel  context.CancelFunc
}

// Server handles Model Context Protocol (MCP) communication over HTTP/SSE.
type Server struct {
	deps          *registry.Dependencies
	sessions      map[string]*session
	subscriptions map[string]map[string]bool
	mu            sync.RWMutex
}

// NewServer creates a new MCP server instance.
func NewServer(deps *registry.Dependencies) *Server {
	srv := &Server{
		deps:          deps,
		sessions:      make(map[string]*session),
		subscriptions: make(map[string]map[string]bool),
	}
	srv.startEventSubscription()
	return srv
}

// HandleSSE establishes the Server-Sent Events connection.
func (s *Server) HandleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	sessionID := uuid.New().String()
	messageChan := make(chan []byte, 10)
	sessionCtx, cancelSession := context.WithCancel(r.Context())
	defer cancelSession()

	s.mu.Lock()
	s.sessions[sessionID] = &session{
		msgChan: messageChan,
		ctx:     sessionCtx,
		cancel:  cancelSession,
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		// Clean up subscriptions for this session
		for uri, sessMap := range s.subscriptions {
			delete(sessMap, sessionID)
			if len(sessMap) == 0 {
				delete(s.subscriptions, uri)
			}
		}
		s.mu.Unlock()
		logger.Info("MCP session closed", "session_id", sessionID)
	}()

	logger.Info("MCP session established", "session_id", sessionID)

	endpointURL := fmt.Sprintf("/mcp/messages?session_id=%s", sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", string(msg))
			flusher.Flush()
		case <-ticker.C:
			// Send comment keep-alive ping
			fmt.Fprintf(w, ":\n\n")
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// HandleMessages receives JSON-RPC messages from the agent.
func (s *Server) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeJSONError(w, "session_id required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	sess, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		writeJSONError(w, "Session not found", http.StatusNotFound)
		return
	}

	// AUTH: Requirement 4 - API Key Authentication
	apiKey := r.Header.Get("X-API-Key")

	if apiKey == "" {
		writeJSONError(w, "unauthorized: api key required", http.StatusUnauthorized)
		return
	}

	if s.deps.Resolver == nil {
		writeJSONError(w, "identity resolver not configured", http.StatusInternalServerError)
		return
	}

	actor, err := s.deps.Resolver.GetActorByAPIKey(r.Context(), apiKey)
	if err != nil {
		writeJSONError(w, "unauthorized: invalid api key", http.StatusUnauthorized)
		return
	}

	if actor.Type != ident.ActorAIAgent {
		writeJSONError(w, "unauthorized: actor is not an AI agent", http.StatusForbidden)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &Error{Code: CodeParseError, Message: "Parse error: " + err.Error()},
		})
		return
	}

	// Dispatch request using the session context (cancelled if the SSE connection drops)
	asyncCtx := middleware.WithActor(sess.ctx, actor)
	go s.dispatch(asyncCtx, sessionID, actor, req)

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) dispatch(ctx context.Context, sessionID string, actor *ident.Actor, req JSONRPCRequest) {
	var resp JSONRPCResponse
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	switch req.Method {
	case "initialize":
		resp.Result = s.handleInitialize(ctx)
	case "notifications/initialized":
		// Initialization notification does not expect a response
		return
	case "tools/list":
		resp.Result = s.handleToolsList(ctx)
	case "tools/call":
		resp.Result, resp.Error = s.handleToolsCall(ctx, actor, req.Params)
	case "resources/list":
		resp.Result = s.handleResourcesList(ctx)
	case "resources/read":
		resp.Result, resp.Error = s.handleResourcesRead(ctx, req.Params)
	case "resources/subscribe":
		resp.Result, resp.Error = s.handleResourcesSubscribe(ctx, sessionID, req.Params)
	case "resources/unsubscribe":
		resp.Result, resp.Error = s.handleResourcesUnsubscribe(ctx, sessionID, req.Params)
	default:
		resp.Error = &Error{Code: CodeMethodNotFound, Message: "Method not found: " + req.Method}
	}

	s.SendMessage(sessionID, resp)
}

func (s *Server) handleInitialize(ctx context.Context) InitializeResult {
	return InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools:     map[string]any{},
			Resources: map[string]any{},
		},
		ServerInfo: ServerInfo{
			Name:    "hyperrr",
			Version: "1.0.0",
		},
	}
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

	// Validate input schema if present (M1)
	if wf.InputSchema != nil {
		if err := validateInputSchema(args, wf.InputSchema); err != nil {
			return nil, &Error{Code: CodeInvalidParams, Message: "Invalid arguments: " + err.Error()}
		}
	}
	
	// Execute the workflow
	execID := "mcp_" + uuid.New().String()
	
	// Inject actor into context
	ctx = middleware.WithActor(ctx, actor)
	
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
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	select {
	case sess.msgChan <- data:
		return nil
	default:
		return fmt.Errorf("session message buffer full")
	}
}

func (s *Server) handleResourcesList(ctx context.Context) *ListResourcesResult {
	var list []Resource
	modules := registry.List()
	for _, m := range modules {
		if prov, ok := m.(registry.ResourceProvider); ok {
			resList, err := prov.ListResources(ctx)
			if err != nil {
				logger.Error("failed to list resources for module", "module", m.ID(), "error", err)
				continue
			}
			for _, r := range resList {
				list = append(list, Resource{
					URI:         r.URI,
					Name:        r.Name,
					Description: r.Description,
					MimeType:    r.MimeType,
				})
			}
		}
	}
	return &ListResourcesResult{Resources: list}
}

func (s *Server) handleResourcesRead(ctx context.Context, params map[string]any) (any, *Error) {
	uri, ok := params["uri"].(string)
	if !ok {
		return nil, &Error{Code: CodeInvalidParams, Message: "Resource URI required"}
	}

	modules := registry.List()
	for _, m := range modules {
		if prov, ok := m.(registry.ResourceProvider); ok {
			contentStr, err := prov.ReadResource(ctx, uri)
			if err == nil && contentStr != "" {
				return &ReadResourceResult{
					Contents: []ResourceContent{
						{
							URI:      uri,
							MimeType: "application/json",
							Text:     contentStr,
						},
					},
				}, nil
			}
		}
	}

	return nil, &Error{Code: CodeInvalidParams, Message: "Resource not found for URI: " + uri}
}

func (s *Server) handleResourcesSubscribe(ctx context.Context, sessionID string, params map[string]any) (any, *Error) {
	uri, ok := params["uri"].(string)
	if !ok {
		return nil, &Error{Code: CodeInvalidParams, Message: "Resource URI required"}
	}

	s.mu.Lock()
	if s.subscriptions[uri] == nil {
		s.subscriptions[uri] = make(map[string]bool)
	}
	s.subscriptions[uri][sessionID] = true
	s.mu.Unlock()

	logger.Info("Subscribed session to resource", "session_id", sessionID, "uri", uri)
	return map[string]any{"status": "subscribed"}, nil
}

func (s *Server) handleResourcesUnsubscribe(ctx context.Context, sessionID string, params map[string]any) (any, *Error) {
	uri, ok := params["uri"].(string)
	if !ok {
		return nil, &Error{Code: CodeInvalidParams, Message: "Resource URI required"}
	}

	s.mu.Lock()
	if sessMap, ok := s.subscriptions[uri]; ok {
		delete(sessMap, sessionID)
		if len(sessMap) == 0 {
			delete(s.subscriptions, uri)
		}
	}
	s.mu.Unlock()

	logger.Info("Unsubscribed session from resource", "session_id", sessionID, "uri", uri)
	return map[string]any{"status": "unsubscribed"}, nil
}

func (s *Server) startEventSubscription() {
	if s.deps.EventBus == nil {
		return
	}

	eventTypes := []string{
		"workflow.started",
		"workflow.completed",
		"workflow.failed",
		"order.created",
		"order.paid",
		"customer.created",
		"customer.updated",
		"cart.updated",
		"product.created",
		"product.updated",
	}

	for _, et := range eventTypes {
		_, _ = s.deps.EventBus.Subscribe(context.Background(), et, func(ctx context.Context, ev eventbus.Event) error {
			s.handleIncomingEvent(ev)
			return nil
		})
	}
}

func (s *Server) handleIncomingEvent(ev eventbus.Event) {
	var uris []string

	extractURIs := func(m map[string]any) {
		for k, v := range m {
			strVal, ok := v.(string)
			if !ok {
				continue
			}
			switch k {
			case "order_id", "order":
				uris = append(uris, "order://"+strVal, "order://"+strVal+"/status")
			case "product_id", "product":
				uris = append(uris, "product://"+strVal)
			case "customer_id", "customer":
				uris = append(uris, "customer://"+strVal)
			case "cart_id", "cart":
				uris = append(uris, "cart://"+strVal)
			case "id", "workflow_id":
				uris = append(uris, "workflow://"+strVal)
			}
		}
	}

	metaMap := make(map[string]any)
	for k, v := range ev.Metadata {
		metaMap[k] = v
	}
	extractURIs(metaMap)

	if payloadMap, ok := ev.Payload.(map[string]any); ok {
		extractURIs(payloadMap)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	notified := make(map[string]bool)
	for _, uri := range uris {
		if notified[uri] {
			continue
		}
		notified[uri] = true

		if sessMap, ok := s.subscriptions[uri]; ok {
			for sessionID := range sessMap {
				notificationPayload := map[string]any{
					"jsonrpc": "2.0",
					"method":  "notifications/resources/updated",
					"params": map[string]any{
						"uri": uri,
					},
				}
				_ = s.SendMessage(sessionID, notificationPayload)
			}
		}
	}
}

func validateInputSchema(args map[string]any, schema map[string]any) error {
	if schema == nil {
		return nil
	}

	if reqRaw, ok := schema["required"]; ok {
		if reqSlice, ok := reqRaw.([]any); ok {
			for _, r := range reqSlice {
				reqStr, ok := r.(string)
				if !ok {
					continue
				}
				if _, exists := args[reqStr]; !exists {
					return fmt.Errorf("missing required property: %s", reqStr)
				}
			}
		} else if reqStringSlice, ok := reqRaw.([]string); ok {
			for _, reqStr := range reqStringSlice {
				if _, exists := args[reqStr]; !exists {
					return fmt.Errorf("missing required property: %s", reqStr)
				}
			}
		}
	}

	if propsRaw, ok := schema["properties"]; ok {
		props, ok := propsRaw.(map[string]any)
		if !ok {
			return nil
		}

		for propName, propSchemaRaw := range props {
			propSchema, ok := propSchemaRaw.(map[string]any)
			if !ok {
				continue
			}

			val, exists := args[propName]
			if !exists {
				continue
			}

			expectedType, ok := propSchema["type"].(string)
			if !ok {
				continue
			}

			switch expectedType {
			case "string":
				if _, ok := val.(string); !ok {
					return fmt.Errorf("property %s must be a string", propName)
				}
			case "integer":
				switch val.(type) {
				case int, int32, int64, float64:
				default:
					return fmt.Errorf("property %s must be an integer", propName)
				}
			case "number":
				switch val.(type) {
				case int, int32, int64, float32, float64:
				default:
					return fmt.Errorf("property %s must be a number", propName)
				}
			case "boolean":
				if _, ok := val.(bool); !ok {
					return fmt.Errorf("property %s must be a boolean", propName)
				}
			case "array":
				if _, ok := val.([]any); !ok {
					return fmt.Errorf("property %s must be an array", propName)
				}
			case "object":
				if _, ok := val.(map[string]any); !ok {
					return fmt.Errorf("property %s must be an object", propName)
				}
			}
		}
	}

	return nil
}

func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
