# Issues: Native MCP Server (Agent Gateway)

This document breaks down the implementation of the Native MCP Server into verifiable vertical slices, as defined in `docs/PRD_Native_MCP_Server.md`.

---

## Issue 1: AI-Enhanced Workflow Definitions

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`docs/PRD_Native_MCP_Server.md`

### What to build

Enhance the workflow DSL to support AI metadata. This allows developers to explicitly tag which workflows should be available as "Tools" for AI agents.

- Update the `Workflow` struct in `internal/workflow` to include:
    - `ExposeToAI bool`
    - `Description string` (Human/AI readable description)
    - `InputSchema map[string]any` (A JSON Schema representation of the required input)
- Update existing core workflows (e.g., `order.checkout`) with sample AI metadata for testing.

### How to verify

- **Manual**: Create a test workflow with `ExposeToAI: true` and verify it compiles and can be registered in the `WorkflowRegistry`.
- **Automated**: Unit test in `internal/workflow/registry_test.go` asserting that AI metadata is correctly preserved after registration.

### Acceptance criteria

- [ ] `Workflow` struct contains all three new fields.
- [ ] Any module can register a workflow with AI metadata without breaking existing orchestration logic.

---

## Issue 2: MCP HTTP/SSE Gateway

**Type**: AFK
**Blocked by**: Issue 1

### Parent PRD

`docs/PRD_Native_MCP_Server.md`

### What to build

Implement the transport layer for the Model Context Protocol. We will use Server-Sent Events (SSE) over HTTP to support remote agents.

- Create the `modules/mcp` module.
- Implement two HTTP endpoints:
    - `GET /mcp/sse`: Establishes the SSE connection.
    - `POST /mcp/messages`: Receives JSON-RPC messages from the agent.
- Integrate the module into the main application router in `internal/app/app.go`.
- Ensure the connection lifecycle is correctly managed (shutdown on server exit).

### How to verify

- **Manual**: Use `curl` to connect to `/mcp/sse` and verify the server sends an initial "endpoint" event.
- **Automated**: Integration test in `modules/mcp/transport_test.go` that opens a connection and verifies basic connectivity.

### Acceptance criteria

- [ ] `/mcp/sse` successfully streams events to a client.
- [ ] The monolith starts up and shuts down cleanly with the MCP module enabled.

---

## Issue 3: Dynamic Tool Discovery

**Type**: AFK
**Blocked by**: Issue 2

### Parent PRD

`docs/PRD_Native_MCP_Server.md`

### What to build

Implement the tool discovery logic. This enables AI agents to ask Hyperrr "What can you do?" and receive a list of available workflows.

- Implement the `tools/list` JSON-RPC handler within `modules/mcp`.
- The handler must:
    - Access the `WorkflowRegistry` via `registry.Dependencies`.
    - Filter all workflows where `ExposeToAI` is true.
    - Map the workflow metadata into MCP `Tool` objects (name, description, inputSchema).

### How to verify

- **Manual**: Connect via SSE and send a `tools/list` request via POST to `/mcp/messages`. Verify the response contains only the tagged workflows.
- **Automated**: Test case in `modules/mcp/discovery_test.go` asserting the tool list matches the registry state.

### Acceptance criteria

- [ ] Only workflows with `ExposeToAI: true` are returned to the agent.
- [ ] Tool names and schemas are correctly formatted according to the MCP spec.

---

## Issue 4: Secure Tool Execution

**Type**: HITL
**Blocked by**: Issue 3

### Parent PRD

`docs/PRD_Native_MCP_Server.md`

### What to build

Implement the execution bridge. This allows AI agents to actually run workflows.

- Implement the `tools/call` JSON-RPC handler within `modules/mcp`.
- The handler must:
    - Enforce API Key authentication (using existing `identity.GetActorByAPIKey`).
    - Verify the caller's actor type is `ActorAIAgent`.
    - Extract arguments from the MCP request and pass them to `WorkflowRunner.Execute()`.
    - Return the workflow results back to the agent as an MCP `CallToolResult`.
- Ensure all AI-triggered workflows carry the correct `Actor` context for auditing.

### How to verify

- **Manual**: Use an external agent (or script) to call a tool with a valid API Key and verify the workflow completes and results are returned via SSE.
- **Automated**: Full integration test verifying the path from `tools/call` -> `WorkflowRunner` -> `Projector` logs.

### Acceptance criteria

- [ ] Unauthenticated tool calls are rejected with 401 Unauthorized.
- [ ] Workflows execute with the AI Agent's identity in the context.
- [ ] Results and errors from the workflow runner are correctly wrapped in MCP responses.
