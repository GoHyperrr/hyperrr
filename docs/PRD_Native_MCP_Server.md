# PRD: Native MCP Server (Agent Gateway)

## Problem Statement

As commerce shifts from human-driven UI interactions to AI-driven autonomous operations, traditional REST and GraphQL APIs become bottlenecks. AI agents require schemas, context, and tool definitions to interact with a system. Currently, if an external AI agent wants to refund an order or query inventory in Hyperrr, developers must build custom prompt wrappers, define OpenAPI schemas, and manage complex tool-calling logic on the client side. The OS lacks a native, standardized way to expose its capabilities and data to AI models.

## Solution

We will implement a native **Model Context Protocol (MCP) Server** directly into the Hyperrr OS kernel. MCP is the emerging industry standard for connecting AI models to data sources and tools. 

By running an MCP server over HTTP (Server-Sent Events), Hyperrr will allow any remote, authenticated AI agent to dynamically discover and execute commerce workflows as "Tools," and read commerce data as "Resources." The MCP integration will be a core composable feature: any commerce module can effortlessly expose its capabilities to the AI ecosystem without writing custom API endpoints.

## User Stories

1.  **As a Commerce Developer**, I want to expose a workflow to AI agents simply by setting a flag on the workflow definition, so that I don't have to maintain a separate MCP tool registry.
2.  **As an AI Agent (External)**, I want to connect to Hyperrr via HTTP/SSE, so that I can dynamically discover available commerce tools (e.g., `order.refund`) and resources.
3.  **As an AI Agent**, I want to execute a Hyperrr workflow as an MCP Tool, so that I can perform autonomous actions like updating inventory or contacting support.
4.  **As a Security Administrator**, I want AI agents to authenticate via API Keys and be identified as `ActorAIAgent`, so that all autonomous actions are securely audited in the system lineage.
5.  **As a Platform Operator**, I want the MCP server to run alongside the existing GraphQL server, so that human and AI interfaces are served from the same unified monolith.

## Implementation Decisions

*   **Transport Layer**: The MCP server will operate over **HTTP/SSE (Server-Sent Events)**. This allows remote agents to maintain a persistent connection to the Hyperrr monolith.
*   **Authentication**: We will leverage the existing `modules/identity` and `modules/auth`. AI agents will authenticate using API Keys. The API Key middleware will inject an `identity.Actor` of type `ActorAIAgent` into the request context.
*   **Auto-Discovery via Workflow Definition**: 
    *   We will add an `ExposeToAI bool` (or similar `MCPTool *MCPConfig`) field to the core `workflow.Workflow` struct.
    *   When the MCP server receives a `tools/list` request, it will query the `WorkflowRegistry`, filter for workflows with `ExposeToAI == true`, and dynamically generate the JSON Schema for the tool based on the workflow's expected input.
*   **Execution**: When an agent calls an MCP Tool, the MCP Server will map the request directly to `runner.Execute()`, passing the authenticated `Actor` context.
*   **Module Placement**: The MCP server will be built as a core kernel package (e.g., `pkg/mcp` or `modules/mcp`) that hooks into the standard application router in `internal/app/app.go`.

## Module Design

### 1. `modules/mcp` (The MCP Server)
*   **Name**: Agent Gateway (MCP)
*   **Responsibility**: Handle the HTTP/SSE lifecycle for the Model Context Protocol. Translate MCP Tool requests into Workflow Runner executions. Translate MCP Resource requests into internal queries.
*   **Interface**: 
    *   Exposes HTTP routes: `/mcp/sse` (connection) and `/mcp/messages` (command reception).
    *   Hooks into `registry.Dependencies` to access the `WorkflowRegistry` and `WorkflowRunner`.
*   **Tested**: Yes

### 2. `internal/workflow` (Definition Upgrade)
*   **Responsibility**: Enhance the workflow definition to support AI metadata.
*   **Interface Change**:
    ```go
    type Workflow struct {
        Name        string
        Version     string
        Description string // Used as the MCP Tool description
        Steps       []Step
        ExposeToAI  bool   // If true, automatically registered as an MCP Tool
        InputSchema any    // JSON Schema definition for the AI agent
    }
    ```
*   **Tested**: No (Struct modification)

## Testing Decisions

*   **Integration Tests**: We will build an in-memory HTTP test server that simulates an external AI agent connecting via SSE, requesting the tool list, and successfully executing a mock workflow via an MCP JSON-RPC payload.
*   **Authentication Tests**: Ensure that an MCP request without a valid API Key is rejected, and that successful executions correctly attribute the action to an `ActorAIAgent` in the execution context.

## Out of Scope

*   Building a frontend chat interface for the AI.
*   Integrating an actual LLM (like OpenAI or Anthropic SDKs) *inside* Hyperrr. Hyperrr is the *Server*; the LLM is the *Client*.
*   Implementing MCP "Prompts" initially. We will focus purely on "Tools" (Workflows) and "Resources" (Data) for V1.

## Open Questions

*   **Input Schema Generation**: How do we define the `InputSchema` for a workflow so the AI knows what arguments to pass? 
    *   *Suggested Resolution*: Initially require developers to provide a raw JSON Schema string in the `Workflow` struct. Later, we can explore reflection-based automatic schema generation from Go structs.
*   **UCP Integration**: How will MCP Resources format their data?
    *   *Suggested Resolution*: For V1, Resources will return raw JSON. When the UCP (Unified Commerce Protocol) is implemented later, the MCP Resources will be upgraded to return strict UCP-compliant JSON.

## Further Notes

By integrating MCP directly into the Workflow Registry, Hyperrr achieves zero-friction AI extensibility. A developer writing a new commerce feature simply flags `ExposeToAI: true`, and the feature is instantly available to the global ecosystem of autonomous agents.
