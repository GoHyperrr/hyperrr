# Model Context Protocol (MCP) Server

Hyperrr is built from the ground up to be **AI-native**. Rather than exposing raw REST or GraphQL APIs that AI agents must navigate manually, Hyperrr implements the **Model Context Protocol (MCP)** via a native SSE (Server-Sent Events) server (`api/mcp`). 

This architecture allows autonomous LLM-based agents to discover capabilities, execute transactions, and subscribe to system status updates.

---

## 1. Workflows as AI-Native Tools

Traditional tool calls are hardcoded in application gateways. Hyperrr implements **declarative tool discovery**:

*   **ExposeToAI**: Any workflow registered in the Workflow Registry that has the `ExposeToAI: true` flag set is automatically converted into an MCP tool.
*   **JSON Schema Translation**: The workflow's `InputSchema` and `Description` are formatted directly into the MCP `tools/list` protocol response.
*   **Tool Execution**: When an agent executes a tool (`tools/call`), the MCP server maps parameters, generates a unique execution ID, and runs the workflow DAG on the Workflow Runner.

---

## 2. Resource Mapping and Reading

Hyperrr modules can expose internal entities (such as orders, bookings, and product pages) as discoverable URIs using the `registry.ResourceProvider` interface:

```go
type ResourceProvider interface {
	ListResources(ctx context.Context) ([]MCPResource, error)
	ReadResource(ctx context.Context, uri string) (string, error)
}
```

*   **URI Schemes**: Resources are identified by standard URI schemes:
    - Order status: `order://{order_id}/status`
    - Product data: `product://{sku}`
*   **Listing Resources**: The MCP server gathers list payloads from all active resource providers when responding to `resources/list` queries.
*   **Reading Resources**: When an agent requests a URI (`resources/read`), the MCP server identifies the owning module, fetches the data, and returns a JSON payload.

---

## 3. SSE Resource Subscriptions and Events Mapping

To enable agents to watch for transaction state updates (such as payment success or shipment dispatch), Hyperrr implements **reactive resources**.

```
+-----------+                      +------------+                     +----------+
| AI Agent  |  -- subscribe -->    | MCP Server |  -- subscribes -->  | EventBus |
|           |                      |  Gateway   |                     |  Fabric  |
+-----------+                      +------------+                     +----------+
      ^                                  |                                 |
      |                                  |                                 |
      | <------ SSE Notification --------+ <------- Publish Event ---------+
```

### 3.1 Subscription Handlers
When an agent subscribes to a URI (e.g. `order://order_abc/status`), the MCP server registers a dynamic event subscription on the central `EventBus`:
- It listens for specific system events related to that resource URI (e.g. `commerce.order.*`).
- When a matching event is published, the handler extracts the updated data, matches the target URI, and writes an MCP `resources/update` notification directly to the client's Server-Sent Events (SSE) stream.

### 3.2 SSE Transport Protocol
Subscriptions use a dual-endpoint HTTP structure:
1.  **`/mcp/sse`**: Establishes a persistent Server-Sent Events stream from Hyperrr to the client. This stream transmits JSON-RPC notifications and events.
2.  **`/mcp/messages`**: An HTTP POST endpoint where the client sends JSON-RPC requests (e.g., calling tools, subscribing to resources).

---

## 4. Session Cancellation and Resource Lifecycles

To prevent resource leaks and runaways (e.g., an agent starts a long-running payment workflow and abruptly disconnects), Hyperrr couples execution lifecycles directly to transport sessions.

### Context Bound Lifecycles
1. When an SSE stream is opened, the server generates a **session context** (`context.WithCancel`).
2. Every tool call or workflow run initiated by that agent is executed under a child context derived from this session context.
3. If the HTTP stream closes or drops, the server triggers the session context cancellation.
4. Any running workflow steps or database locks associated with the cancelled session are immediately halted and rolled back via Saga compensations, protecting system integrity.

---

## 5. MCP Apps and UI Integration

Hyperrr supports the **MCP Apps UI spec** to enable rendering interactive, rich, and responsive dashboards inside compatible hosts (such as Claude Desktop or ChatGPT sandboxed `iframe` panels).

### 5.1 Tool UI Linking
Any tool exposed by the MCP server can declare a companion UI resource URI under its `_meta` field:
```json
{
  "name": "app.product",
  "description": "Dashboard and interactive console application for the commerce.product module.",
  "inputSchema": { "type": "object" },
  "_meta": {
    "ui": {
      "resourceUri": "ui://commerce.product"
    }
  }
}
```

### 5.2 Server-Side Rendered (SSR) HTML Resources
When a host queries or requests to read the `ui://` resource scheme, Hyperrr intercept it and renders a dedicated HTML/CSS dashboard populated with real-time data directly from the GORM database.
*   **MIME Type**: These resources are served with the standardized `text/html;profile=mcp-app` MIME type.
*   **Design & Theme**: The generated dashboards follow premium dark-mode styling with Outfit typography, glassmorphic card layouts, responsive CSS grids, CSS-only micro-animations, and HSL tailored color accents matching each specific module.

---

## 6. Dynamic Prompt Templates

To guide AI agents in auditing and inspecting various modules, Hyperrr registers standard prompt templates that hosts can retrieve using the `prompts/list` and `prompts/get` protocol:
1.  **System Diagnostics**: Run `system.about` to check server configurations, active modules, and environment flags.
2.  **Inventory Health Check**: Review the product catalog and identify inventory shortages in fulfillment.
3.  **Fulfillment Saga Tracker**: Audit PENDING orders and trace stuck transactions.
4.  **Product Catalog Audit**: Check listing consistency, prices, and descriptions.
5.  **Customer Churn Risk Analysis**: Inspect segments, personas, and identify VIP churn indicators.

---

## 7. Standard Validation and Utility Methods

*   **Ping-Pong Check**: Supports the JSON-RPC `ping` request method. The server returns a simple `"pong"` string to verify connectivity.
*   **Resource Templates**: Supports discovery of dynamic URI template schemas via `resources/templates/list` queries.
*   **Logging Level Control**: Supports the `logging/setLevel` method as a no-op success handler to prevent client connection warnings.
