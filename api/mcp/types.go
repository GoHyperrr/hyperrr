package mcp

// JSONRPCRequest represents a standard MCP JSON-RPC request.
type JSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
	ID      any            `json:"id"`
}

// JSONRPCResponse represents a standard MCP JSON-RPC response.
type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
	ID      any    `json:"id"`
}

// Error represents a JSON-RPC error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP Spec Codes
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Tool represents an MCP Tool definition.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ListToolsResult is the result for tools/list.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolResult is the result for tools/call.
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents a piece of content in a tool result.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
