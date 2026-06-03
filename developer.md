# Hyperrr Core OS: Developer & Consumer Guide

Welcome to the **Hyperrr Core OS** Developer Guide. Hyperrr is an AI-native, modular "operating system" designed to run transaction-heavy workflows across multiple domains (such as Retail Commerce, Travel, Lodging, and Logistics).

This guide describes the core runtime architecture, the pluggable storage and locking kernel, the Event Fabric, and the Agent Gateway (MCP). It also provides a complete, step-by-step tutorial on how to develop, integrate, and deploy a custom module that is fully integrated with the GraphQL API and exposed to autonomous AI agents.

---

## 1. System Architecture Overview

Hyperrr is organized as a modular, multi-workspace monolith. The core kernel engine resides in the `hyperrr` directory, while functional modules are grouped into separate independent repositories and Go modules (such as `commerce` and `auth`), co-located and linked via a multi-module **Go Workspace (`go.work`)** at the project root. This ensures strict boundary separation and clean dependency management.

```
       +--------------------------------------------------------+
       |                  AI Agent / GraphQL Clients            |
       +---------------------------+----------------------------+
                                   |
                     MCP SSE / GraphQL HTTP Requests
                                   |
                                   v
       +--------------------------------------------------------+
       |                      API Gateway                       |
       |  (GraphQL Resolver Container & Model Context Protocol) |
       +---------------------------+----------------------------+
                                   |
                  Dynamic Module Discovery & Execution
                                   |
                                   v
       +--------------------------------------------------------+
       |                    Workflow Runner                     |
       |  (DAG Execution, Saga Rollbacks, State Persistence)    |
       +---------------------------+----------------------------+
                                   |
             +---------------------+---------------------+
             |                     |                     |
             v                     v                     v
      [Event Fabric]         [Lock Provider]       [State Store]
      (In-Memory/NATS)       (In-Memory/Redis)     (Redis/NATS JS)
```

### The Core Modules
*   **Workflow Runner (`internal/workflow`)**: Orchestrates complex workflows using Directed Acyclic Graphs (DAGs). Provides built-in support for step retries, transactional Saga compensations, and human-in-the-loop escalation gates.
*   **Agent Gateway (`api/mcp`)**: Implements the Model Context Protocol (MCP). Dynamically translates registered workflows into tools for LLMs, maps system resources, and publishes reactive change events.
*   **API Router (`api/graph`)**: Stitches GraphQL resolvers together, handles token authentication, and enforces RBAC identities.
*   **Kernel Services (`pkg/`)**: Holds standard implementations for database mapping (`pkg/db`), event buses (`pkg/eventbus`), and distributed synchronization (`pkg/locking`).

### Detailed Subsystem Architecture Guides
For in-depth explanations, code structures, and implementation philosophies of each major subsystem, refer to the following dedicated manuals:
1.  **[Workflows & DAG Execution Engine](file:///D:/hyperrr-commerce-ai/docs/workflows_and_dags.md)**: Explains declarative step definitions, dependencies evaluation, parallel branch execution, and state checkpointing.
2.  **[Distributed Transactions & Saga Compensations](file:///D:/hyperrr-commerce-ai/docs/distributed_transactions_sagas.md)**: Details the eventual consistency pattern, chronological rollback history execution, and critical transaction alert handling.
3.  **[Pluggable Event Fabric](file:///D:/hyperrr-commerce-ai/docs/event_fabric.md)**: Compares in-memory and NATS message distribution, namespace routing, and context tracing propagation.
4.  **[Model Context Protocol (MCP) Server](file:///D:/hyperrr-commerce-ai/docs/model_context_protocol.md)**: Describes translating workflows to LLM tools, context-bound lifecycles, and SSE-based resource subscriptions.
5.  **[Database Architecture & Schema Auto-Migrations](file:///D:/hyperrr-commerce-ai/docs/database_and_migrations.md)**: Outlines GORM dialect setups, dynamic module database registration, idempotency checking, and table isolation patterns.
6.  **[GraphQL API Gateway & Security Middleware](file:///D:/hyperrr-commerce-ai/docs/graphql_api_gateway.md)**: Explains modular schema aggregation, token parsing interceptors, RBAC actor context injection, and entity type mappers.
7.  **[Distributed Locking & Coordination Kernel](file:///D:/hyperrr-commerce-ai/docs/distributed_locking_coordination.md)**: Describes pluggable lock interfaces, compare-and-swap (CAS) lock mechanics, and lock ownership validations.


---

## 2. Core Capabilities & Design Patterns

### 2.1 Pluggable Workflows & DAG Execution
Workflows in Hyperrr are defined declaratively:
```go
type Workflow struct {
    Name        string         // Unique ID (e.g., "order.fulfillment")
    Version     string         // SemVer string
    Description string         // Exposed to LLMs as tool documentation
    ExposeToAI  bool           // Flag indicating auto-discovery by MCP
    InputSchema map[string]any // JSON Schema of parameters
    Steps       []Step         // DAG steps
}
```
*   **Parallel Execution**: The Runner evaluates the `DependsOn` fields of all steps, constructing a dependency tree and launching independent steps in parallel goroutines.
*   **State Checkpointing**: Every step transition (PENDING -> RUNNING -> COMPLETED/FAILED) is persisted to the `StateStore`. If the application crashes, the auto-recovery supervisor scans the store at startup and calls `ResumeExecution()` to pick up unfinished tasks.
*   **Saga Compensation Transactions**: Each workflow step can register a `Saga` rollback handler. If a step fails, the engine halts the execution DAG, rolls back through the execution history, and executes compensating actions in reverse order to ensure consistency.

### 2.2 Pluggable State Stores & Lockers
Hyperrr maintains three lock and store drivers: **In-Memory** (for local development and testing), **NATS JetStream KV**, and **Redis**.
*   **Consistency Guarantee**: To swap state stores without changing behavior, all drivers implement the same transactional expectations. Both NATS and Redis locker implementations use compare-and-swap (CAS) and owner validation check scripts during locks and releases to prevent delayed release races.
*   **Key-Level TTLs**: State stores implement `SetTTL` to automatically expire completed transaction state data.

### 2.3 Reactive Event Fabric
The `EventBus` handles system communication.
```go
type EventBus interface {
    Publish(ctx context.Context, event Event) error
    Subscribe(ctx context.Context, eventType string, handler EventHandler) (Subscription, error)
    Close() error
}
```
On publishing an event, the system maps structural metadata to track request lineage across goroutines, enabling full traceability in the workflow projection log (`internal/context`).

### 2.4 Agent Gateway (Model Context Protocol)
The native MCP server runs alongside the HTTP monolith, offering:
*   **Tools Auto-Discovery**: Workflows with `ExposeToAI: true` are formatted as JSON Schema tools for LLMs.
*   **Session-Bound Lifecycles**: Invoked tools run on contexts linked to the SSE stream. If an agent drops its connection, ongoing background tasks are cancelled immediately to prevent resource leakage.
*   **Resource Reading & Subscriptions**: Exposes system entities (such as orders or product catalogs) via URIs (e.g., `order://{id}/status`). Agents can subscribe to these URIs and receive reactive notification payloads whenever matching events are published to the `EventBus`.

---

## 3. Step-by-Step Module Development Guide

This tutorial shows how to build a new domain module (e.g., a "Hotels & Lodging" module), register it with the Core OS, implement database persistence, hook task handlers to workflows, and expose resources to AI agents.

### Step 1: Implement the Module Interface
Create a new directory `commerce/hotel` and declare the module structure matching the `registry.Module` interface:

```go
package hotel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"gorm.io/gorm"
)

type Module struct {
	db   *gorm.DB
	bus  eventbus.EventBus
	repo *Repository
}

func NewModule() *Module {
	return &Module{}
}

func (m *Module) ID() string {
	return "commerce.hotel"
}

func (m *Module) Models() []any {
	return []any{&Booking{}}
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}
```

### Step 2: Set Up Database Persistence
Define the database models and a simple repository structure. Hyperrr uses GORM for object mapping:

```go
type BookingStatus string

const (
	BookingPending   BookingStatus = "PENDING"
	BookingConfirmed BookingStatus = "CONFIRMED"
	BookingCancelled BookingStatus = "CANCELLED"
)

type Booking struct {
	ID         string        `gorm:"primaryKey" json:"id"`
	CustomerID string        `json:"customer_id"`
	RoomType   string        `json:"room_type"`
	Status     BookingStatus `json:"status"`
	Price      float64       `json:"price"`
}

type Repository struct {
	db *gorm.DB
}

func (r *Repository) Save(ctx context.Context, b *Booking) error {
	return r.db.WithContext(ctx).Save(b).Error
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Booking, error) {
	var b Booking
	if err := r.db.WithContext(ctx).First(&b, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}
```

### Step 3: Implement Task Handlers & Saga Compensations
Define task handlers that will be executed as steps in our workflows:

```go
// ReserveRoom creates a pending booking (Forward action)
func (m *Module) ReserveRoom(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid handler input")
	}

	workflowInput, _ := data["input"].(map[string]any)
	customerID, _ := workflowInput["customer_id"].(string)
	roomType, _ := workflowInput["room_type"].(string)
	bookingID, _ := workflowInput["booking_id"].(string)

	booking := &Booking{
		ID:         bookingID,
		CustomerID: customerID,
		RoomType:   roomType,
		Status:     BookingPending,
		Price:      150.00, // Fixed mock price
	}

	if err := m.repo.Save(ctx, booking); err != nil {
		return nil, err
	}

	return map[string]any{"booking": booking}, nil
}

// CancelReservation rolls back a booking (Saga compensation action)
func (m *Module) CancelReservation(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid handler input")
	}

	// Retrieve reserved booking from previous step output context
	reserveStep, ok := data["hotel.reserve_room"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("reservation data not found in saga context")
	}

	bookingMap, _ := reserveStep["booking"].(map[string]any)
	bookingID, _ := bookingMap["id"].(string)

	booking, err := m.repo.GetByID(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve booking for cancellation: %w", err)
	}

	booking.Status = BookingCancelled
	if err := m.repo.Save(ctx, booking); err != nil {
		return nil, err
	}

	return map[string]any{"cancelled_booking_id": bookingID}, nil
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{
		"hotel.reserve_room":       m.ReserveRoom,
		"hotel.cancel_reservation": m.CancelReservation,
	}
}
```

### Step 4: Define Workflows and Expose to AI
Initialize the module, load dependencies, and register the declarative workflows:

```go
func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.db = deps.DB.DB // Access underlying GORM DB wrapper
	m.bus = deps.EventBus
	m.repo = &Repository{db: m.db}

	// Register workflow definition
	err := deps.Registry.Register(&workflow.Workflow{
		Name:        "hotel.booking.v1",
		Description: "Books a hotel room and handles compensation if payment fails.",
		ExposeToAI:  true, // Expose to MCP Agent Gateway
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"customer_id": map[string]any{"type": "string"},
				"room_type":   map[string]any{"type": "string"},
				"booking_id":  map[string]any{"type": "string"},
			},
			"required": []string{"customer_id", "room_type", "booking_id"},
		},
		Steps: []workflow.Step{
			{
				ID:   "hotel.reserve_room",
				Uses: "hotel.reserve_room",
				Saga: &workflow.Saga{Uses: "hotel.cancel_reservation"},
			},
			{
				ID:        "finance.charge_card",
				Uses:      "finance.charge_card",
				DependsOn: []string{"hotel.reserve_room"},
				Saga:      &workflow.Saga{Uses: "finance.refund_payment"},
			},
		},
	})
	return err
}
```

### Step 5: Implement `ResourceProvider` for AI Agent Context
Make the module implement `registry.ResourceProvider` to expose hotel resources and real-time subscription update flags:

```go
func (m *Module) ListResources(ctx context.Context) ([]registry.MCPResource, error) {
	// Query GORM DB to expose current resources
	var bookings []Booking
	if err := m.db.WithContext(ctx).Find(&bookings).Error; err != nil {
		return nil, err
	}

	var res []registry.MCPResource
	for _, b := range bookings {
		res = append(res, registry.MCPResource{
			URI:         "hotel://" + b.ID + "/status",
			Name:        "Hotel Booking: " + b.ID,
			Description: "Fulfillment and payment status of lodging reservation " + b.ID,
			MimeType:    "application/json",
		})
	}
	return res, nil
}

func (m *Module) ReadResource(ctx context.Context, uri string) (string, error) {
	var bookingID string
	n, err := fmt.Sscanf(uri, "hotel://%s", &bookingID)
	if err != nil || n != 1 {
		return "", fmt.Errorf("invalid URI pattern")
	}
	bookingID = strings.TrimSuffix(bookingID, "/status")

	booking, err := m.repo.GetByID(ctx, bookingID)
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.Marshal(booking)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
```

### Step 6: Expose GraphQL Resolvers via `GraphQLProvider`
To expose your database query methods or mutation actions to the GraphQL API, your module must implement the `registry.GraphQLProvider` interface.

First, create a `hotel.graphqls` schema file inside `commerce/hotel/`:
```graphql
type HotelBooking {
  id: ID!
  customerId: String!
  roomType: String!
  status: String!
  price: Float!
}

extend type Query {
  getHotelBooking(id: ID!): HotelBooking
}

extend type Mutation {
  bookRoom(customerId: String!, roomType: String!, bookingId: String!): HotelBooking
}
```

Then, implement the GraphQLProvider methods in a new file `commerce/hotel/graphql.go`:
```go
package hotel

import (
	"context"
	"github.com/GoHyperrr/hyperrr/api/graph/model"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Ensure Module implements registry.GraphQLProvider at compile time.
var _ registry.GraphQLProvider = (*Module)(nil)

func (m *Module) Queries() map[string]any {
	return map[string]any{
		"getHotelBooking": m.GetHotelBookingResolver,
	}
}

func (m *Module) Mutations() map[string]any {
	return map[string]any{
		"bookRoom": m.BookRoomResolver,
	}
}

func (m *Module) FieldResolvers() map[string]any {
	return nil
}

// Resolver implementations:
func (m *Module) GetHotelBookingResolver(ctx context.Context, id string) (*model.HotelBooking, error) {
	b, err := m.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &model.HotelBooking{
		ID:         b.ID,
		CustomerID: b.CustomerID,
		RoomType:   b.RoomType,
		Status:     string(b.Status),
		Price:      b.Price,
	}, nil
}

func (m *Module) BookRoomResolver(ctx context.Context, customerID string, roomType string, bookingID string) (*model.HotelBooking, error) {
	// Execute the booking workflow and return the mapped booking model
	// ...
	return nil, nil
}
```

Now run the code generation build command:
```bash
go run ./cmd/hyperrr build
```
This automatically aggregates the new `hotel.graphqls` schema, executes GQLGen, and generates dynamic delegation code linking your module to the API Gateway.
```

---

## 4. Bootstrapping & Registering the Module

Hyperrr implements a dynamic, configuration-driven module registration pattern. Developers do **not** need to modify the core application bootstrapper [internal/app/app.go](file:///D:/hyperrr-commerce-ai/internal/app/app.go) or main entry points to register, load, or activate their modules.

Instead, module registration requires three steps:

### Step A: Register the Module Factory
Within the custom module package (e.g. `commerce/hotel/module.go` or a new `init.go`), define a Go package `init()` function that registers a `ModuleFactory` constructor to the global factory registry under **both** its short ID and its full Go import package path name:

```go
func init() {
	factory := func(options map[string]any) (registry.Module, error) {
		// Instantiate and configure module using provided option variables
		m := NewModule()
		return m, nil
	}
	
	// Register under the short ID and full package path
	registry.RegisterFactory("commerce.hotel", factory)
	registry.RegisterFactory("github.com/GoHyperrr/commerce/hotel", factory)
}
```

### Step B: Register dynamic CLI commands (Optional)
If your module provides dynamic subcommands (such as user creation or key generation), you can register them within your `init()` block:

```go
func init() {
	// ... factory registrations ...

	registry.RegisterCommand(registry.CLICommand{
		Name:        "hotel",
		Usage:       "book <customer_id> <room_type>",
		Description: "Book a hotel room dynamically via the CLI",
		Run: func(deps *registry.Dependencies, args []string) error {
			// Custom booking logic utilizing deps.DB
			return nil
		},
	})
}
```

### Step C: Configure Module Activation in the Config File
Add the module definition and its key/value options to the application configuration (JSON or YAML) under the `modules` array. 

To keep secrets and credentials secure, options can dynamically load variables from the environment by using the `"env."` prefix. Hyperrr automatically resolves these at startup:

# hyperrr.yaml (or hyperrr.yml / configs/hyperrr.json)
modules:
  - resolve: "github.com/GoHyperrr/commerce/hotel"
    id: "hyperrr.hotel"            # Optional: Override lookup ID mapping manually
    options:
      apiKey: "env.HOTEL_API_KEY"  # Resolves os.Getenv("HOTEL_API_KEY") at runtime
      apiUrl: "https://api.hotels.com"
```

#### **How Path Matching & Normalization Work:**
Hyperrr utilizes the `registry.NormalizeModuleID` function to match configured module paths to their registered factory IDs automatically:
* **Manual Override (`id`)**: If specified, the system looks up the module factory exactly under this custom name (e.g. `hyperrr.hotel`).
* **Fallback Normalization**: If `id` is omitted, the `resolve` import path is normalized automatically (e.g., `github.com/GoHyperrr/commerce/hotel` becomes `commerce.hotel`), mapping it seamlessly to the factory registered with that standard name inside the package `init()`.

### Step D: Run Code Generation
To compile the package into the binary without manual code edits, run the built-in Go generator command in your terminal from the workspace root:

```bash
go generate ./...
```

The generation script parses the configuration file, extracts the package paths in `resolve`, and automatically generates the blank imports registry file `internal/app/imports_generated.go`. This registers the constructors at compile-time.

On launch, the Core OS runtime resolves the configured `modules` list, instantiates them with resolved options, and completes bootstrapping:

1.  **Auto-Discovery & Schema Migrations**: The core auto-discovers GORM schemas registered in `Models()` and executes database schema migrations automatically.
2.  **Workflow Task Handlers Hooking**: Task handler maps returned by `Handlers()` are registered with the Workflow Runner.
3.  **Init & Dependency Resolution**: The core invokes `Init(...)` on each module, allowing them to look up dependent modules dynamically from the registry and prepare endpoints.
4.  **GraphQL Gateway Integration**: The core resolves GQLGen resolver interfaces and delegates fields dynamically to modules implementing `GraphQLProvider`.
5.  **Agent Gateway Exposure**: The MCP Gateway server registers resources and workflows dynamically, preparing tools and SSE notification channels.

---

## 5. Production & Execution Configuration

### 5.1 Environmental Configurations
Settings are parsed using `Viper` at boot. A configuration file (such as `hyperrr.yml`, `hyperrr.yaml`, or `hyperrr.json`) is automatically loaded if present in the workspace root or `configs/` directory, and is merged with environment variables loaded from `.env`:
```bash
# Event Bus and Distributed Locks
EVENT_BUS_PROVIDER=nats
NATS_URL=nats://localhost:4222
WORKFLOW_STORE_TYPE=redis
REDIS_URL=redis://localhost:6379

# Secrets and Server Configs
SERVER_PORT=8080
LOG_LEVEL=info

# MCP Gateway Security Providers (default: apikey)
# In configuration files (hyperrr.yml):
# MCP_AUTH_PROVIDERS: ["apikey"] (or ["none"] to bypass authentication during testing)
MCP_AUTH_PROVIDERS=apikey
```

### 5.2 Auto-Recovery Loop
If a node crashes mid-execution, workflows marked as `RUNNING` in the shared NATS/Redis database become stalled. Hyperrr runs a background monitoring loop at boot:

```go
stalled, _ := store.ListExecutions(ctx, workflow.StateRunning)
for _, execID := range stalled {
    states, _ := store.GetState(ctx, execID)
    wfName := states["__wf_name"]
    wf, _ := registryStore.Get(wfName)
    
// Resumes DAG execution from the last successfully completed step checkpoint
    go runner.ResumeExecution(ctx, execID, wf)
}
```
This guarantees transactional reliability for business logic without requiring complex manually-written recovery procedures.

---

## 6. Handling Cross-Module Dependencies & Relations

When building pluggable, independent modules (such as a `coupon` module that needs to access data or trigger actions inside `product` and `order` modules), maintaining a clean separation of concerns is vital. Hyperrr utilizes three design patterns to resolve cross-module coupling:

### 6.1 Dynamic Service Locator (`registry.Get`)
To prevent hardcoded compiler dependencies and initialization races, modules do not reference each other during instantiation. Instead, they resolve dependencies dynamically during their `Init` phase by querying the global registry.

For example, inside a `coupon.Module` struct:
```go
type Module struct {
	prodMod  product.ModuleInterface // resolved interface
	orderMod order.ModuleInterface   // resolved interface
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	// Look up peer modules from global registry
	if prodVal, ok := registry.Get("commerce.product"); ok {
		m.prodMod = prodVal.(product.ModuleInterface)
	}
	if orderVal, ok := registry.Get("commerce.order"); ok {
		m.orderMod = orderVal.(order.ModuleInterface)
	}
	return nil
}
```

### 6.2 Soft Database Relations
In a modular monolith or microservices architecture, hard foreign-key constraints across module databases (e.g. coupon database table hard-joining with the order table) are a major antipattern because they break boundary separation.

Instead, use **soft relations**:
1. Store foreign references as simple data types (e.g., `ProductID string` or `OrderID string` on your model).
2. Avoid using GORM's automatic `BelongsTo` or `HasMany` relation pointers across module-owned structs.
3. Fetch the linked entity data by calling the referenced module's repository/service interface at query time.

```go
type CouponApplied struct {
	ID        string `gorm:"primaryKey"`
	CouponID  string `json:"coupon_id"`
	ProductID string `json:"product_id"` // Soft relation: simple string field
}
```

### 6.3 Asynchronous Decoupling via Event Fabric
Instead of calling a peer module's methods directly (which creates synchronous compile-time coupling), modules should publish and subscribe to events on the `EventBus`.

* **Scenario**: A `coupon` module wants to mark a coupon code as used when an order is finalized.
* **Solution**: 
  - The `order` module publishes an `order.finalized` event containing the `order_id` and any applied coupon codes.
  - The `coupon` module subscribes to `order.finalized` and updates its internal state asynchronously.
  - Neither module requires compile-time references to the other.

```go
// Inside coupon module initialization:
deps.EventBus.Subscribe(ctx, "order.finalized", func(ctx context.Context, event eventbus.Event) error {
	// 1. Unmarshal order data
	// 2. Mark corresponding coupon as applied
	return nil
})
```

