# Pluggable Event Fabric

The Event Fabric in Hyperrr is a pluggable event-driven message distribution core (`pkg/eventbus`). It enables asynchronous communication between core services, decoupled commerce modules, and the external Model Context Protocol (MCP) subscription gateway.

---

## 1. Event Model and Structure

Every event published to the Event Fabric contains structured payload fields and tracking metadata to support tracing:

```go
type Event struct {
	ID        string    `json:"id"`        // Unique event identifier (e.g., "evt_334455")
	Type      string    `json:"type"`      // Dot-separated namespace (e.g., "commerce.order.placed")
	Payload   any       `json:"payload"`   // Raw structured payload data
	Timestamp time.Time `json:"timestamp"` // Execution timestamp
	Metadata  Metadata  `json:"metadata"`  // Tracing context, credentials, and correlation tags
}

type Metadata map[string]string
```

### Namespace Conventions
Hyperrr events follow a strict namespace format:
`[scope].[entity].[action]` (e.g., `commerce.order.created`, `system.workflow.started`).

---

## 2. Event Bus Providers

The `EventBus` interface is designed to support different backends depending on the deployment profile:

```go
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, eventType string, handler EventHandler) (Subscription, error)
	Close() error
}
```

### 2.1 In-Memory Event Bus (`InMemBus`)
Used for local testing and lightweight monolith deployments:
- Uses Go channels and standard mutexes to distribute events.
- Dispatch is **non-blocking** (each subscriber handler runs in its own goroutine) to prevent a slow handler from blocking other listeners.
- Thread-safe tracking of sub lists.

### 2.2 NATS Event Bus (`NATSBus`)
Used for production deployments and distributed systems:
- Connects to a distributed NATS cluster (`nats://localhost:4222`).
- Maps event types directly to NATS subjects (e.g. replacing dots with sub-namespaces for wildcard routing like `commerce.order.*`).
- Guarantees reliability and cluster-wide message delivery using NATS JetStream.

---

## 3. Transaction Lineage and Distributed Tracing

In an event-driven system, tracing transaction paths across parallel processes and async boundaries is essential. Hyperrr implements **Context Propagation** through the `Event.Metadata` field.

### Metadata Tracing Fields
When publishing an event, the publisher copies values from the active `context.Context` to the event's metadata map:
*   **`trace_id`**: A unique ID generated at the entry point of the transaction (e.g., GraphQL query or MCP call) that identifies the logical execution sequence.
*   **`correlation_id`**: Identifies specific parent workflow executions or child tasks.
*   **`actor_id`**: Identifies the security context (API user key or session actor) that initiated the transaction.

### Context Extraction
When a subscriber receives an event, it extracts the metadata and instantiates a new context containing those values:

```go
func HandleEvent(ctx context.Context, event eventbus.Event) error {
    // Reconstruct tracing and security context
    traceID := event.Metadata["trace_id"]
    actorID := event.Metadata["actor_id"]
    
    tracedCtx := context.WithValue(ctx, "trace_id", traceID)
    tracedCtx = context.WithValue(tracedCtx, "actor_id", actorID)
    
    // Process event using traced context
    return processOrder(tracedCtx, event.Payload)
}
```
This enables tracing a transaction from an agent's initial request to workflow steps, database migrations, and eventual resource status modifications.
