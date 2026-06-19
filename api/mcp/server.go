package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/GoHyperrr/hyperrr/api/middleware"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	ident "github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/mdk"
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

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	endpointURL := fmt.Sprintf("%s://%s/mcp/messages?session_id=%s", scheme, r.Host, sessionID)
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

	// AUTH: Dynamic Multi-Provider Authentication
	providers := s.deps.Config.MCPAuthProviders
	if len(providers) == 0 {
		providers = []string{"apikey"} // Default fallback
	}

	var actor ident.Actor
	var authErr error
	authenticated := false

	for _, p := range providers {
		switch p {
		case "none":
			// Bypass authentication check entirely and mock a developer actor
			actor = &ident.BaseActor{
				ID:   "act_mcp_developer",
				Type: ident.ActorAIAgent,
				Name: "Developer Agent (No Auth)",
			}
			authenticated = true
		case "apikey":
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				authErr = fmt.Errorf("api key required")
				continue
			}
			if s.deps.Resolver == nil {
				authErr = fmt.Errorf("identity resolver not configured")
				continue
			}
			resActor, err := s.deps.Resolver.GetActorByAPIKey(r.Context(), apiKey)
			if err != nil {
				authErr = fmt.Errorf("invalid api key: %w", err)
				continue
			}
			if resActor.GetType() != ident.ActorAIAgent {
				authErr = fmt.Errorf("actor is not an AI agent")
				continue
			}
			actor = resActor
			authenticated = true
		}
		if authenticated {
			break
		}
	}

	if !authenticated {
		errMsg := "unauthorized"
		if authErr != nil {
			errMsg = fmt.Sprintf("unauthorized: %v", authErr)
		}
		writeJSONError(w, errMsg, http.StatusUnauthorized)
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

func (s *Server) dispatch(ctx context.Context, sessionID string, actor ident.Actor, req JSONRPCRequest) {
	var resp JSONRPCResponse
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	// If the request doesn't have an ID, it is a JSON-RPC notification and does not expect a response.
	isNotification := req.ID == nil

	switch req.Method {
	case "initialize":
		resp.Result = s.handleInitialize(ctx)
	case "notifications/initialized":
		// Initialization notification does not expect a response
		return
	case "ping":
		resp.Result = "pong"
	case "logging/setLevel":
		// Accept setLevel requests as a success no-op
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = s.handleToolsList(ctx)
	case "tools/call":
		resp.Result, resp.Error = s.handleToolsCall(ctx, actor, req.Params)
	case "resources/list":
		resp.Result = s.handleResourcesList(ctx)
	case "resources/templates/list":
		resp.Result = s.handleResourceTemplatesList(ctx)
	case "resources/read":
		resp.Result, resp.Error = s.handleResourcesRead(ctx, req.Params)
	case "resources/subscribe":
		resp.Result, resp.Error = s.handleResourcesSubscribe(ctx, sessionID, req.Params)
	case "resources/unsubscribe":
		resp.Result, resp.Error = s.handleResourcesUnsubscribe(ctx, sessionID, req.Params)
	case "prompts/list":
		resp.Result, resp.Error = s.handlePromptsList(ctx)
	case "prompts/get":
		resp.Result, resp.Error = s.handlePromptsGet(ctx, req.Params)
	default:
		resp.Error = &Error{Code: CodeMethodNotFound, Message: "Method not found: " + req.Method}
	}

	if !isNotification {
		s.SendMessage(sessionID, resp)
	}
}

func (s *Server) handleInitialize(ctx context.Context) InitializeResult {
	return InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Logging: &LoggingCapability{},
			Prompts: &PromptsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   true,
				ListChanged: false,
			},
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "hyperrr",
			Version: "1.0.0",
		},
	}
}

func (s *Server) handlePromptsList(ctx context.Context) (any, *Error) {
	prompts := []registry.MCPPrompt{
		{
			Name:        "System Diagnostics",
			Description: "Ask the agent to check the health, version, and active modules of the commerce server.",
		},
		{
			Name:        "Inventory Health Check",
			Description: "Diagnose inventory shortages or out-of-stock items in fulfillment.",
		},
		{
			Name:        "Fulfillment Saga Tracker",
			Description: "Analyze the lifecycle of the fulfillment saga to find stuck orders.",
		},
		{
			Name:        "Product Catalog Audit",
			Description: "Examine product catalog listings, prices, and descriptions.",
		},
		{
			Name:        "Customer Churn Risk Analysis",
			Description: "Analyze customer segments, personas, and identify high-risk profiles.",
		},
		{
			Name:        "Workflow & Event Map Auditing",
			Description: "Examine the dynamic DAG workflows and event subscription maps of the commerce application to understand the ripple effects of actions.",
		},
	}
	for _, mod := range registry.List() {
		if provider, ok := mod.(registry.PromptProvider); ok {
			pList, err := provider.ListPrompts(ctx)
			if err != nil {
				return nil, &Error{Code: CodeInternalError, Message: "failed to list prompts from " + mod.ID() + ": " + err.Error()}
			}
			prompts = append(prompts, pList...)
		}
	}
	return map[string]any{"prompts": prompts}, nil
}

func (s *Server) handlePromptsGet(ctx context.Context, params map[string]any) (any, *Error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, &Error{Code: CodeInvalidParams, Message: "prompt name required"}
	}

	switch name {
	case "System Diagnostics":
		return &registry.GetPromptResult{
			Description: "System Diagnostics",
			Messages: []registry.MCPPromptMessage{
				{
					Role: "user",
					Content: registry.MCPPromptMessageContent{
						Type: "text",
						Text: "Please execute system.about and analyze the health of the application. List all active modules and verify if the system environment is set up correctly.",
					},
				},
			},
		}, nil
	case "Inventory Health Check":
		return &registry.GetPromptResult{
			Description: "Inventory Health Check",
			Messages: []registry.MCPPromptMessage{
				{
					Role: "user",
					Content: registry.MCPPromptMessageContent{
						Type: "text",
						Text: "Inspect the product catalog and check available stock in fulfillment. Highlight any items with 0 available quantity or low stock.",
					},
				},
			},
		}, nil
	case "Fulfillment Saga Tracker":
		return &registry.GetPromptResult{
			Description: "Fulfillment Saga Tracker",
			Messages: []registry.MCPPromptMessage{
				{
					Role: "user",
					Content: registry.MCPPromptMessageContent{
						Type: "text",
						Text: "Review recent orders and shipments. Are there any PENDING orders that haven't been SHIPPED yet? Diagnose the bottleneck.",
					},
				},
			},
		}, nil
	case "Product Catalog Audit":
		return &registry.GetPromptResult{
			Description: "Product Catalog Audit",
			Messages: []registry.MCPPromptMessage{
				{
					Role: "user",
					Content: registry.MCPPromptMessageContent{
						Type: "text",
						Text: "Review all products in the catalog. Identify any pricing inconsistencies or missing descriptions.",
					},
				},
			},
		}, nil
	case "Customer Churn Risk Analysis":
		return &registry.GetPromptResult{
			Description: "Customer Churn Risk Analysis",
			Messages: []registry.MCPPromptMessage{
				{
					Role: "user",
					Content: registry.MCPPromptMessageContent{
						Type: "text",
						Text: "Check customer profiles. Pay special attention to their ML-calculated personas and highlight any churn risks or high-value VIP segments.",
					},
				},
			},
		}, nil
	case "Workflow & Event Map Auditing":
		return &registry.GetPromptResult{
			Description: "Workflow & Event Map Auditing",
			Messages: []registry.MCPPromptMessage{
				{
					Role: "user",
					Content: registry.MCPPromptMessageContent{
						Type: "text",
						Text: "Please call system.list_event_listeners and system.list_workflows. Audit the entire system architecture, explaining how different modules interact via events and what workflows trigger as a result.",
					},
				},
			},
		}, nil
	}

	argsRaw, _ := params["arguments"].(map[string]any)
	args := make(map[string]string)
	for k, v := range argsRaw {
		if strVal, ok := v.(string); ok {
			args[k] = strVal
		} else {
			args[k] = fmt.Sprintf("%v", v)
		}
	}

	for _, mod := range registry.List() {
		if provider, ok := mod.(registry.PromptProvider); ok {
			pList, err := provider.ListPrompts(ctx)
			if err != nil {
				continue
			}
			found := false
			for _, p := range pList {
				if p.Name == name {
					found = true
					break
				}
			}
			if found {
				result, err := provider.GetPrompt(ctx, name, args)
				if err != nil {
					return nil, &Error{Code: CodeInternalError, Message: err.Error()}
				}
				return result, nil
			}
		}
	}

	return nil, &Error{Code: CodeInvalidParams, Message: "Prompt not found: " + name}
}

func (s *Server) handleToolsList(ctx context.Context) *ListToolsResult {
	workflows := s.deps.Registry.List()
	tools := []Tool{}

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
				Meta: &ToolMeta{
					UI: &ToolMetaUI{
						ResourceURI: "ui://" + wf.Name,
					},
				},
			})
		}
	}

	// Dynamic app tools for each module
	modules := []string{
		"commerce.product",
		"commerce.customer",
		"commerce.cart",
		"commerce.order",
		"commerce.payments",
		"commerce.taxonomy",
		"commerce.seo",
		"commerce.store",
		"notification",
	}

	for _, modID := range modules {
		appName := "app." + strings.TrimPrefix(modID, "commerce.")
		tools = append(tools, Tool{
			Name:        appName,
			Description: fmt.Sprintf("Dashboard and interactive console application for the %s module.", modID),
			InputSchema: map[string]any{"type": "object"},
			Meta: &ToolMeta{
				UI: &ToolMetaUI{
					ResourceURI: "ui://" + modID,
				},
			},
		})
	}

	// Expose system event listener tracking tool
	tools = append(tools, Tool{
		Name:        "system.list_event_listeners",
		Description: "Retrieve a list of all registered event subscriptions, their namespaces, event types, and the handlers that process them.",
		InputSchema: map[string]any{"type": "object"},
	})

	// Expose system workflows list tool
	tools = append(tools, Tool{
		Name:        "system.list_workflows",
		Description: "Retrieve a list of all registered workflows, their detailed step sequences, step dependencies, and saga compensations.",
		InputSchema: map[string]any{"type": "object"},
	})

	return &ListToolsResult{Tools: tools}
}

func (s *Server) handleToolsCall(ctx context.Context, actor ident.Actor, params map[string]any) (any, *Error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, &Error{Code: CodeInvalidParams, Message: "Tool name required"}
	}

	if name == "system.list_event_listeners" {
		subs := []mdk.SubscriptionInfo{}
		if s.deps.EventBus != nil {
			subs = s.deps.EventBus.Subscribers()
		}
		dataBytes, err := json.MarshalIndent(subs, "", "  ")
		if err != nil {
			return nil, &Error{Code: CodeInternalError, Message: "failed to marshal event listeners: " + err.Error()}
		}
		return CallToolResult{
			Content: []Content{
				{
					Type: "text",
					Text: string(dataBytes),
				},
			},
		}, nil
	}

	if name == "system.list_workflows" {
		workflows := s.deps.Registry.List()
		dataBytes, err := json.MarshalIndent(workflows, "", "  ")
		if err != nil {
			return nil, &Error{Code: CodeInternalError, Message: "failed to marshal workflows: " + err.Error()}
		}
		return CallToolResult{
			Content: []Content{
				{
					Type: "text",
					Text: string(dataBytes),
				},
			},
		}, nil
	}

	if strings.HasPrefix(name, "app.") {
		modID := "commerce." + strings.TrimPrefix(name, "app.")
		return CallToolResult{
			Content: []Content{
				{
					Type: "text",
					Text: fmt.Sprintf("The %s application interface is loaded. You can view the interactive UI by opening the resource: ui://%s", name, modID),
				},
			},
		}, nil
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
	
	results, err := s.deps.Runner.ExecuteSyncWorkflow(ctx, execID, wf, args)
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
	list := []Resource{}

	// Add dynamic App UI resources for each module
	modules := []string{
		"commerce.product",
		"commerce.customer",
		"commerce.cart",
		"commerce.order",
		"commerce.payments",
		"commerce.taxonomy",
		"commerce.seo",
		"commerce.store",
		"notification",
	}

	for _, modID := range modules {
		list = append(list, Resource{
			URI:         "ui://" + modID,
			Name:        "App: " + modID,
			Description: "Interactive control panel and real-time dashboard for " + modID,
			MimeType:    "text/html;profile=mcp-app",
		})
	}

	// Add workflow UIs
	list = append(list, Resource{
		URI:         "ui://system.about",
		Name:        "App: system.about",
		Description: "Interactive dashboard for system metadata and system logs.",
		MimeType:    "text/html;profile=mcp-app",
	})
	list = append(list, Resource{
		URI:         "system://event_listeners",
		Name:        "System Event Listeners",
		Description: "A registry map of all active event subscriptions, namespaces, event types, and their handlers.",
		MimeType:    "application/json",
	})
	list = append(list, Resource{
		URI:         "system://workflows",
		Name:        "System Workflows",
		Description: "A list of all registered workflows, including their steps, dependencies, and saga compensation tasks.",
		MimeType:    "application/json",
	})

	mods := registry.List()
	for _, m := range mods {
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

	if uri == "system://event_listeners" {
		subs := []mdk.SubscriptionInfo{}
		if s.deps.EventBus != nil {
			subs = s.deps.EventBus.Subscribers()
		}
		dataBytes, err := json.MarshalIndent(subs, "", "  ")
		if err != nil {
			return nil, &Error{Code: CodeInternalError, Message: "failed to marshal event listeners: " + err.Error()}
		}
		return &ReadResourceResult{
			Contents: []ResourceContent{
				{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(dataBytes),
				},
			},
		}, nil
	}

	if uri == "system://workflows" {
		workflows := s.deps.Registry.List()
		dataBytes, err := json.MarshalIndent(workflows, "", "  ")
		if err != nil {
			return nil, &Error{Code: CodeInternalError, Message: "failed to marshal workflows: " + err.Error()}
		}
		return &ReadResourceResult{
			Contents: []ResourceContent{
				{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(dataBytes),
				},
			},
		}, nil
	}

	// Intercept UI resource requests to perform SSR rendering
	if strings.HasPrefix(uri, "ui://") {
		appName := strings.TrimPrefix(uri, "ui://")
		htmlContent := s.renderUI(ctx, appName)
		return &ReadResourceResult{
			Contents: []ResourceContent{
				{
					URI:      uri,
					MimeType: "text/html;profile=mcp-app",
					Text:     htmlContent,
				},
			},
		}, nil
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
		parts := strings.SplitN(et, ".", 2)
		var ns, evType string
		if len(parts) == 2 {
			ns, evType = parts[0], parts[1]
		} else {
			ns, evType = "", et
		}
		_, _ = s.deps.EventBus.Subscribe(ns, evType, func(ctx context.Context, ev eventbus.Event) error {
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

	extractURIs(ev.Payload)

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

func (s *Server) handleResourceTemplatesList(ctx context.Context) *ListResourceTemplatesResult {
	return &ListResourceTemplatesResult{
		ResourceTemplates: []ResourceTemplate{},
	}
}

func (s *Server) renderUI(ctx context.Context, appName string) string {
	accent := "#3b82f6" // Default Blue
	accentGlow := "rgba(59, 130, 246, 0.15)"
	title := appName
	content := ""

	switch appName {
	case "commerce.product":
		accent = "#a78bfa" // Violet
		accentGlow = "rgba(167, 139, 250, 0.15)"
		title = "Product Catalog"
		var list []map[string]any
		var count int64
		if s.deps.DB != nil {
			s.deps.DB.Table("products").Find(&list)
			s.deps.DB.Table("products").Count(&count)
		}
		
		content = fmt.Sprintf(`
			<div class="grid-container">
				<div class="glass-card stat-card">
					<span class="stat-label">Total Products</span>
					<span class="stat-value">%d</span>
				</div>
				<div class="glass-card stat-card">
					<span class="stat-label">Catalog Status</span>
					<span class="stat-value text-accent">Active</span>
				</div>
			</div>
			<div class="glass-card">
				<h2>Products List</h2>
				<div class="table-wrapper">
					<table>
						<thead>
							<tr>
								<th>ID</th>
								<th>Name</th>
								<th>Description</th>
								<th>Price</th>
								<th>Currency</th>
							</tr>
						</thead>
						<tbody>`, count)
		
		if len(list) == 0 {
			content += `<tr><td colspan="5" style="text-align: center; color: var(--text-secondary);">No products registered.</td></tr>`
		} else {
			for _, p := range list {
				currencyCode := "USD"
				if s.deps.Config != nil && s.deps.Config.Currency != "" {
					currencyCode = s.deps.Config.Currency
				}
				if metaVal, ok := p["metadata"]; ok && metaVal != nil {
					if metaStr, ok := metaVal.(string); ok && metaStr != "" {
						var metaMap map[string]any
						if err := json.Unmarshal([]byte(metaStr), &metaMap); err == nil {
							if curr, ok := metaMap["currency"].(string); ok {
								currencyCode = curr
							}
						}
					} else if metaMap, ok := metaVal.(map[string]any); ok {
						if curr, ok := metaMap["currency"].(string); ok {
							currencyCode = curr
						}
					}
				}

				priceStr := "N/A"
				var variants []map[string]any
				var pID string
				if idVal, ok := p["id"].(string); ok {
					pID = idVal
				}
				if s.deps.DB != nil && pID != "" {
					s.deps.DB.Table("product_variants").Where("product_id = ?", pID).Find(&variants)
				}
				if len(variants) > 0 {
					getPrice := func(v map[string]any) float64 {
						if prVal, ok := v["price"].(float64); ok {
							return prVal
						}
						if prVal, ok := v["price"].(int64); ok {
							return float64(prVal)
						}
						if prVal, ok := v["price"].(float32); ok {
							return float64(prVal)
						}
						if prVal, ok := v["price"].(int); ok {
							return float64(prVal)
						}
						return 0.0
					}
					minP := getPrice(variants[0])
					maxP := getPrice(variants[0])
					for _, v := range variants {
						pr := getPrice(v)
						if pr < minP {
							minP = pr
						}
						if pr > maxP {
							maxP = pr
						}
					}
					priceStr = formatPrice(minP, currencyCode)
					if minP != maxP {
						priceStr = fmt.Sprintf("%s - %s", formatPrice(minP, currencyCode), formatPrice(maxP, currencyCode))
					}
				}
				pName, _ := p["name"].(string)
				pDesc, _ := p["description"].(string)
				content += fmt.Sprintf(`
					<tr>
						<td><code>%s</code></td>
						<td>%s</td>
						<td>%s</td>
						<td class="text-accent">%s</td>
						<td>%s</td>
					</tr>`, pID, pName, pDesc, priceStr, currencyCode)
			}
		}
		content += `
						</tbody>
					</table>
				</div>
			</div>`

	case "commerce.customer":
		accent = "#f59e0b" // Amber
		accentGlow = "rgba(245, 158, 11, 0.15)"
		title = "Customer Directory"
		var list []map[string]any
		var count int64
		if s.deps.DB != nil {
			s.deps.DB.Table("customers").Find(&list)
			s.deps.DB.Table("customers").Count(&count)
		}

		content = fmt.Sprintf(`
			<div class="grid-container">
				<div class="glass-card stat-card">
					<span class="stat-label">Total Customers</span>
					<span class="stat-value">%d</span>
				</div>
				<div class="glass-card stat-card">
					<span class="stat-label">ML Segmentation</span>
					<span class="stat-value text-accent">Enabled</span>
				</div>
			</div>
			<div class="glass-card">
				<h2>Registered Customers</h2>
				<div class="table-wrapper">
					<table>
						<thead>
							<tr>
								<th>ID</th>
								<th>Name</th>
								<th>Email</th>
								<th>Type</th>
							</tr>
						</thead>
						<tbody>`, count)

		if len(list) == 0 {
			content += `<tr><td colspan="4" style="text-align: center; color: var(--text-secondary);">No customers registered.</td></tr>`
		} else {
			for _, c := range list {
				custType := "Registered"
				if isGuest, ok := c["is_guest"].(bool); ok && isGuest {
					custType = "Guest"
				} else if isGuestInt, ok := c["is_guest"].(int64); ok && isGuestInt != 0 {
					custType = "Guest"
				}
				cID, _ := c["id"].(string)
				cName, _ := c["name"].(string)
				cEmail, _ := c["email"].(string)
				content += fmt.Sprintf(`
					<tr>
						<td><code>%s</code></td>
						<td>%s</td>
						<td>%s</td>
						<td><span class="badge" style="background: rgba(245, 158, 11, 0.1); border: 1px solid var(--accent-color);">%s</span></td>
					</tr>`, cID, cName, cEmail, custType)
			}
		}
		content += `
						</tbody>
					</table>
				</div>
			</div>`

	case "commerce.cart":
		accent = "#60a5fa" // Light Blue
		accentGlow = "rgba(96, 165, 250, 0.15)"
		title = "Active Shopping Carts"
		var list []map[string]any
		var count int64
		if s.deps.DB != nil {
			s.deps.DB.Table("carts").Find(&list)
			s.deps.DB.Table("carts").Count(&count)
		}

		content = fmt.Sprintf(`
			<div class="grid-container">
				<div class="glass-card stat-card">
					<span class="stat-label">Total Carts</span>
					<span class="stat-value">%d</span>
				</div>
				<div class="glass-card stat-card">
					<span class="stat-label">Cart Engine</span>
					<span class="stat-value text-accent">Active</span>
				</div>
			</div>
			<div class="glass-card">
				<h2>Carts List</h2>
				<div class="table-wrapper">
					<table>
						<thead>
							<tr>
								<th>Cart ID</th>
								<th>Customer ID</th>
								<th>Status</th>
								<th>Items Count</th>
							</tr>
						</thead>
						<tbody>`, count)

		if len(list) == 0 {
			content += `<tr><td colspan="4" style="text-align: center; color: var(--text-secondary);">No shopping carts registered.</td></tr>`
		} else {
			for _, c := range list {
				var itemCount int64
				cID, _ := c["id"].(string)
				if s.deps.DB != nil && cID != "" {
					s.deps.DB.Table("cart_items").Where("cart_id = ?", cID).Count(&itemCount)
				}
				cCustID, _ := c["customer_id"].(string)
				cStatus, _ := c["status"].(string)
				content += fmt.Sprintf(`
					<tr>
						<td><code>%s</code></td>
						<td><code>%s</code></td>
						<td><span class="badge" style="background: rgba(96, 165, 250, 0.1); border: 1px solid var(--accent-color);">%s</span></td>
						<td>%d items</td>
					</tr>`, cID, cCustID, cStatus, itemCount)
			}
		}
		content += `
						</tbody>
					</table>
				</div>
			</div>`

	case "commerce.order":
		accent = "#10b981" // Emerald
		accentGlow = "rgba(16, 185, 129, 0.15)"
		title = "Order Management"
		var list []map[string]any
		var count int64
		var gross float64
		if s.deps.DB != nil {
			s.deps.DB.Table("orders").Find(&list)
			s.deps.DB.Table("orders").Count(&count)
			s.deps.DB.Table("orders").Select("sum(total_price)").Row().Scan(&gross)
		}

		content = fmt.Sprintf(`
			<div class="grid-container">
				<div class="glass-card stat-card">
					<span class="stat-label">Total Transactions</span>
					<span class="stat-value">%d</span>
				</div>
				<div class="glass-card stat-card">
					<span class="stat-label">Gross Revenue</span>
					<span class="stat-value text-accent">$%.2f</span>
				</div>
			</div>
			<div class="glass-card">
				<h2>Recent Orders</h2>
				<div class="table-wrapper">
					<table>
						<thead>
							<tr>
								<th>Order ID</th>
								<th>Customer ID</th>
								<th>Status</th>
								<th>Total Price</th>
							</tr>
						</thead>
						<tbody>`, count, gross)

		if len(list) == 0 {
			content += `<tr><td colspan="4" style="text-align: center; color: var(--text-secondary);">No orders registered.</td></tr>`
		} else {
			for _, o := range list {
				statusColor := "#9ca3af" // Muted
				status, _ := o["status"].(string)
				switch strings.ToLower(status) {
				case "paid", "fulfilled":
					statusColor = "#10b981"
				case "pending":
					statusColor = "#f59e0b"
				case "cancelled":
					statusColor = "#ef4444"
				}
				totalPrice := 0.0
				if tp, ok := o["total_price"].(float64); ok {
					totalPrice = tp
				} else if tp, ok := o["total_price"].(float32); ok {
					totalPrice = float64(tp)
				} else if tp, ok := o["total_price"].(int64); ok {
					totalPrice = float64(tp)
				}
				oID, _ := o["id"].(string)
				oCustID, _ := o["customer_id"].(string)
				content += fmt.Sprintf(`
					<tr>
						<td><code>%s</code></td>
						<td><code>%s</code></td>
						<td><span class="badge" style="background: rgba(16, 185, 129, 0.05); border: 1px solid %s; color: %s">%s</span></td>
						<td class="text-accent">$%.2f</td>
					</tr>`, oID, oCustID, statusColor, statusColor, status, totalPrice)
			}
		}
		content += `
						</tbody>
					</table>
				</div>
			</div>`

	case "notification":
		accent = "#f97316" // Orange
		accentGlow = "rgba(249, 115, 22, 0.15)"
		title = "Notifications Hub"
		var list []map[string]any
		var count int64
		if s.deps.DB != nil {
			s.deps.DB.Table("notifications").Find(&list)
			s.deps.DB.Table("notifications").Count(&count)
		}

		content = fmt.Sprintf(`
			<div class="grid-container">
				<div class="glass-card stat-card">
					<span class="stat-label">Total Logs Sent</span>
					<span class="stat-value">%d</span>
				</div>
				<div class="glass-card stat-card">
					<span class="stat-label">Delivery Rate</span>
					<span class="stat-value text-accent">100%%</span>
				</div>
			</div>
			<div class="glass-card">
				<h2>Dispatch logs</h2>
				<div class="table-wrapper">
					<table>
						<thead>
							<tr>
								<th>ID</th>
								<th>Recipient</th>
								<th>Channel</th>
								<th>Subject</th>
								<th>Status</th>
							</tr>
						</thead>
						<tbody>`, count)

		if len(list) == 0 {
			content += `<tr><td colspan="5" style="text-align: center; color: var(--text-secondary);">No notification records.</td></tr>`
		} else {
			for _, n := range list {
				nID, _ := n["id"].(string)
				nRecip, _ := n["recipient"].(string)
				nChan, _ := n["channel"].(string)
				nSubj, _ := n["subject"].(string)
				nStat, _ := n["status"].(string)
				content += fmt.Sprintf(`
					<tr>
						<td><code>%s</code></td>
						<td>%s</td>
						<td>%s</td>
						<td>%s</td>
						<td><span class="badge" style="background: rgba(249, 115, 22, 0.1); border: 1px solid var(--accent-color);">%s</span></td>
					</tr>`, nID, nRecip, nChan, nSubj, nStat)
			}
		}
		content += `
						</tbody>
					</table>
				</div>
			</div>`



	case "system.about":
		accent = "#a3e635" // Lime Green
		accentGlow = "rgba(163, 230, 53, 0.15)"
		title = "System Configuration"
		var activeMods []string
		for _, m := range registry.List() {
			activeMods = append(activeMods, fmt.Sprintf("<li><code>%s</code></li>", m.ID()))
		}
		content = fmt.Sprintf(`
			<div class="grid-container">
				<div class="glass-card stat-card">
					<span class="stat-label">System Version</span>
					<span class="stat-value text-accent">v1.0.0</span>
				</div>
				<div class="glass-card stat-card">
					<span class="stat-label">Environment</span>
					<span class="stat-value">Production</span>
				</div>
			</div>
			<div class="glass-card">
				<h2>Active Commerce Plug-In Nodes</h2>
				<ul style="margin: 16px 0; padding-left: 20px; color: var(--text-secondary); line-height: 1.8;">
					%s
				</ul>
			</div>`, strings.Join(activeMods, "\n"))



	default:
		content = `
			<div class="glass-card" style="text-align: center; padding: 48px;">
				<h2 style="color: #ef4444">Application Error</h2>
				<p style="color: var(--text-secondary); margin-top: 8px;">No app dashboard configuration was found for the module URI template.</p>
			</div>`
	}

	htmlSkeleton := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s Dashboard - Hyperrr</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0b0c10;
            --card-bg: rgba(22, 24, 37, 0.7);
            --border-color: rgba(255, 255, 255, 0.08);
            --text-primary: #f3f4f6;
            --text-secondary: #9ca3af;
            --accent-color: %s;
            --accent-glow: %s;
        }
        
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            font-family: 'Outfit', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-primary);
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            padding: 24px;
            overflow-x: hidden;
        }

        .glass-card {
            background: var(--card-bg);
            backdrop-filter: blur(12px);
            border: 1px solid var(--border-color);
            border-radius: 16px;
            padding: 24px;
            box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.37);
            transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1), border-color 0.3s ease;
        }

        .glass-card:hover {
            transform: translateY(-2px);
            border-color: rgba(255, 255, 255, 0.15);
            box-shadow: 0 12px 40px 0 rgba(0, 0, 0, 0.5), 0 0 15px var(--accent-glow);
        }

        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 32px;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 16px;
        }

        h1 {
            font-size: 2.2rem;
            font-weight: 700;
            background: linear-gradient(135deg, #ffffff 0%%, var(--accent-color) 100%%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            letter-spacing: -0.02em;
        }

        .badge {
            background: var(--accent-glow);
            border: 1px solid var(--accent-color);
            color: var(--text-primary);
            padding: 6px 12px;
            border-radius: 20px;
            font-size: 0.85rem;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .grid-container {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
            gap: 24px;
            margin-bottom: 24px;
        }

        .stat-card {
            display: flex;
            flex-direction: column;
            gap: 8px;
        }

        .stat-label {
            font-size: 0.9rem;
            color: var(--text-secondary);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .stat-value {
            font-size: 2rem;
            font-weight: 700;
            color: var(--text-primary);
        }

        .table-wrapper {
            overflow-x: auto;
            margin-top: 16px;
            border-radius: 12px;
            border: 1px solid var(--border-color);
        }

        table {
            width: 100%%;
            border-collapse: collapse;
            text-align: left;
        }

        th {
            background: rgba(255, 255, 255, 0.02);
            color: var(--text-secondary);
            font-weight: 600;
            font-size: 0.9rem;
            padding: 14px 16px;
            border-bottom: 1px solid var(--border-color);
        }

        td {
            padding: 14px 16px;
            border-bottom: 1px solid var(--border-color);
            font-size: 0.95rem;
            color: var(--text-primary);
            transition: background 0.2s ease;
        }

        tr:hover td {
            background: rgba(255, 255, 255, 0.03);
        }

        .text-accent {
            color: var(--accent-color);
        }

        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(10px); }
            to { opacity: 1; transform: translateY(0); }
        }

        .animate-fade-in {
            animation: fadeIn 0.5s ease forwards;
        }
    </style>
</head>
<body>
    <header class="animate-fade-in">
        <div>
            <h1>%s</h1>
            <p style="color: var(--text-secondary); margin-top: 4px;">Hyperrr Command Center - Live App Node</p>
        </div>
        <span class="badge">App Connected</span>
    </header>
    <main class="animate-fade-in" style="animation-delay: 0.1s;">
        %s
    </main>
</body>
</html>`

	return fmt.Sprintf(htmlSkeleton, title, accent, accentGlow, title, content)
}

func formatPrice(price float64, currencyCode string) string {
	switch strings.ToUpper(currencyCode) {
	case "USD":
		return fmt.Sprintf("$%.2f", price)
	case "EUR":
		return fmt.Sprintf("€%.2f", price)
	case "GBP":
		return fmt.Sprintf("£%.2f", price)
	case "JPY":
		return fmt.Sprintf("¥%.0f", price)
	case "INR":
		return fmt.Sprintf("₹%.2f", price)
	default:
		return fmt.Sprintf("%.2f %s", price, currencyCode)
	}
}
