# Hyperrr Core Engine: Developer & Consumer Guide

Welcome to the **Hyperrr Core Engine** Developer Guide. Hyperrr is an AI-native, modular engine designed to run transaction-heavy workflows across multiple domains (such as Retail Commerce, Travel, Lodging, and Logistics).

This guide describes the core runtime architecture, the pluggable storage and locking kernel, the Event Fabric, and the Agent Gateway (MCP). It also provides a complete, step-by-step tutorial on how to develop, integrate, and deploy a custom module using the Module Development Kit (`mdk`) that is fully integrated with the GraphQL API and exposed to autonomous AI agents.

---

## 1. System Architecture Overview

Hyperrr is organized as a modular, multi-workspace monolith. The core kernel engine resides in the `hyperrr` directory, while functional modules are grouped into separate independent repositories and Go modules (such as `commerce` and `auth`), co-located and linked via a multi-module **Go Workspace (`go.work`)** at the project root. This ensures strict boundary separation, compiler decoupling, and clean dependency management.

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

### The Core Modules & Interfaces
*   **Module Development Kit (`mdk`)**: A shared lightweight boundary library containing all core type declarations, event types, workflow engine schemas, and MCP provider contracts. Custom modules depend *only* on `mdk` and have no compile-time dependencies on the core engine.
*   **Workflow Runner (`pkg/workflow`)**: Orchestrates complex workflows using Directed Acyclic Graphs (DAGs). Provides built-in support for step retries, transactional Saga compensations, and auto-resume checkpoint execution.
*   **Agent Gateway (`api/mcp`)**: Implements the Model Context Protocol (MCP). Dynamically translates registered workflows into tools for LLMs, maps system resources, and publishes reactive change events over SSE channels.
*   **API Router (`api/graph`)**: Stitches GraphQL resolvers together, handles multi-provider token authentication, and enforces RBAC actor contexts.

---

## 2. Core Capabilities & Design Patterns

### 2.1 Pluggable Workflows & DAG Execution
Workflows in Hyperrr are defined declaratively via `mdk.Workflow`:
```go
type Workflow struct {
    ID          string         // Unique ID (e.g. "hotel.booking.v1")
    Name        string         // Descriptive name
    Description string         // Exposed to LLMs as tool documentation
    ExposeToAI  bool           // Flag indicating auto-discovery by MCP
    InputSchema map[string]any // JSON Schema of parameters
    Steps       []Step         // DAG steps
}
```
*   **Parallel Execution**: The Runner evaluates the `DependsOn` fields of all steps, constructing a dependency tree and launching independent steps in parallel goroutines.
*   **State Checkpointing**: Every step transition is persisted to the `StateStore`. If the application crashes, the auto-recovery supervisor scans the store at startup and calls `ResumeExecution()` to pick up unfinished tasks.
*   **Saga Compensation Transactions**: Each step can register a `Saga` rollback handler. If a step fails, the engine halts execution, rolls back through the execution history, and executes compensating actions in reverse order to ensure eventual consistency.

### 2.2 Pluggable State Stores & Lockers
Hyperrr maintains three lock and store drivers: **In-Memory** (for local development and testing), **NATS JetStream KV**, and **Redis**. All drivers implement standard `StateStore` and `Locker` interfaces, ensuring transactional invariants are preserved.

### 2.3 Reactive Event Fabric
The Event Fabric propagates domain events asynchronously. On publishing an event, the system maps structural metadata to track request lineage across goroutines, enabling full traceability in the workflow projection logs.

---

## 3. Step-by-Step Module Development Guide

This tutorial shows how to build a new domain module (e.g., a "Hotels & Lodging" module), register it with the Core OS using `mdk`, implement database persistence, hook task handlers to workflows, and expose resources to AI agents.

### Step 1: Implement the Module Interface
Create a new directory `commerce/hotel` and declare the module structure matching the `mdk.Module` interface:

```go
package hotel

import (
	"context"
	"net/http"

	"github.com/GoHyperrr/mdk"
	"gorm.io/gorm"
)

type Module struct {
	db   *gorm.DB
	bus  mdk.EventBus
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

func (m *Module) Routes() []mdk.Route {
	return nil
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
Define task handlers that will be executed as steps in our workflows. Step handlers must match the signature `mdk.StepHandler`:

```go
// ReserveRoom creates a pending booking (Forward action)
func (m *Module) ReserveRoom(sCtx mdk.StepContext) mdk.StepResult {
	customerID, _ := sCtx.Input["customer_id"].(string)
	roomType, _ := sCtx.Input["room_type"].(string)
	bookingID, _ := sCtx.Input["booking_id"].(string)

	booking := &Booking{
		ID:         bookingID,
		CustomerID: customerID,
		RoomType:   roomType,
		Status:     BookingPending,
		Price:      150.00,
	}

	if err := m.repo.Save(sCtx.Ctx, booking); err != nil {
		return mdk.StepResult{Err: err}
	}

	return mdk.StepResult{Output: map[string]any{"booking": booking}}
}

// CancelReservation rolls back a booking (Saga compensation action)
func (m *Module) CancelReservation(sCtx mdk.StepContext) mdk.StepResult {
	reserveStep, ok := sCtx.Input["hotel.reserve_room"].(map[string]any)
	if !ok {
		return mdk.StepResult{Err: fmt.Errorf("reservation data not found in saga context")}
	}

	bookingMap, _ := reserveStep["booking"].(map[string]any)
	bookingID, _ := bookingMap["id"].(string)

	booking, err := m.repo.GetByID(sCtx.Ctx, bookingID)
	if err != nil {
		return mdk.StepResult{Err: fmt.Errorf("failed to retrieve booking: %w", err)}
	}

	booking.Status = BookingCancelled
	if err := m.repo.Save(sCtx.Ctx, booking); err != nil {
		return mdk.StepResult{Err: err}
	}

	return mdk.StepResult{Output: map[string]any{"cancelled_booking_id": bookingID}}
}
```

### Step 4: Define Workflows and Expose to AI
Initialize the module, load dependencies, and register the declarative workflows in the `Init` function:

```go
func (m *Module) Init(ctx context.Context, rt mdk.Runtime) error {
	m.db = rt.DB()
	m.bus = rt.Bus()
	m.repo = &Repository{db: m.db}

	if rt.Workflows() != nil {
		// Register step handlers
		_ = rt.Workflows().RegisterHandler("hotel.reserve_room", m.ReserveRoom)
		_ = rt.Workflows().RegisterHandler("hotel.cancel_reservation", m.CancelReservation)

		// Register workflow definition
		err := rt.Workflows().Register(mdk.Workflow{
			Name:        "hotel.booking.v1",
			Description: "Books a hotel room and handles compensation if payment fails.",
			ExposeToAI:  true,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"customer_id": map[string]any{"type": "string"},
					"room_type":   map[string]any{"type": "string"},
					"booking_id":  map[string]any{"type": "string"},
				},
				"required": []string{"customer_id", "room_type", "booking_id"},
			},
			Steps: []mdk.Step{
				{
					ID:   "hotel.reserve_room",
					Uses: "hotel.reserve_room",
					Saga: &mdk.Saga{Uses: "hotel.cancel_reservation"},
				},
				{
					ID:        "finance.charge_card",
					Uses:      "finance.charge_card",
					DependsOn: []string{"hotel.reserve_room"},
					Saga:      &mdk.Saga{Uses: "finance.refund_payment"},
				},
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}
```

### Step 5: Implement `ResourceProvider` for AI Agent Context
Make the module implement `mdk.ResourceProvider` to expose hotel resources and real-time subscription update channels to LLMs:

```go
// Ensure Module implements mdk.ResourceProvider at compile time.
var _ mdk.ResourceProvider = (*Module)(nil)

func (m *Module) ListResources(ctx context.Context) ([]mdk.MCPResource, error) {
	var bookings []Booking
	if err := m.db.WithContext(ctx).Find(&bookings).Error; err != nil {
		return nil, err
	}

	var res []mdk.MCPResource
	for _, b := range bookings {
		res = append(res, mdk.MCPResource{
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
To expose your database query methods or mutation actions to the GraphQL API, your module must implement the `registry.GraphQLProvider` interface defined in `hyperrr/pkg/registry` (since GraphQL stitching is controlled by the core API gateway).

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
```

Now compile the package into the binary:
```bash
go run ./cmd/hyperrr build
```
This automatically aggregates the new schema, runs gqlgen, and generates custom resolver stitching files.

---

## 4. Bootstrapping & Registering the Module

Hyperrr implements a dynamic, configuration-driven module registration pattern. Developers do **not** need to modify the core application bootstrapper or main entry points to load modules.

### Step A: Register the Module Constructor
Within the custom module package (e.g. `commerce/hotel/module.go`), define a Go package `init()` function that registers the module factory:

```go
func init() {
	mdk.Register(func() mdk.Module {
		return NewModule()
	})
}
```

### Step B: Register dynamic CLI commands (Optional)
If your module provides dynamic subcommands (such as user creation or key generation), you can register them within your `init()` block:

```go
func init() {
	mdk.RegisterCommand(mdk.CLICommand{
		Group:       "auth",
		Name:        "hotel",
		Usage:       "book <customer_id> <room_type>",
		Short:       "Book a hotel room dynamically via the CLI",
		NeedsDB:     true,
		Run: func(rt mdk.Runtime, args []string) error {
			// Custom booking logic utilizing rt.DB()
			return nil
		},
	})
}
```

### Step C: Configure Module Activation in the Config File
Add the module definition and its key/value options to the application configuration:

```yaml
# hyperrr.yml
modules:
  - resolve: "github.com/GoHyperrr/commerce/hotel"
    options:
      apiKey: "${HOTEL_API_KEY}" # Resolves environment variable at runtime
      apiUrl: "https://api.hotels.com"
```

### Step D: Run Code Generation
To compile the package into the binary, run the built-in Go generator command:

```bash
go generate ./...
```
This parses the config file, resolves module paths, and automatically writes the blank imports registry file `internal/app/imports_generated.go`.

---

## 5. Production & Execution Configuration

### 5.1 Environmental Configurations
Settings are parsed using `Viper` at boot. A configuration file (such as `hyperrr.yml`) is automatically loaded if present in the workspace root or `configs/` directory.

Standard environment variable substitution is supported anywhere inside `hyperrr.yml` using the `${VAR_NAME}` or `${env.VAR_NAME:fallback}` pattern.

Strict schema validation is performed at startup, validating ports, event bus drivers, database drivers, and authentication settings before boot.

### 5.2 Auto-Recovery Loop
If a node crashes mid-execution, workflows marked as `RUNNING` in the shared state store become stalled. Hyperrr runs a background monitoring loop at boot, scanning the store for stalled runs and calling `ResumeExecution()` to pick up from the last checkpoint.

---

## 6. Handling Cross-Module Dependencies & Relations

When building pluggable, independent modules, maintaining boundary separation is vital. Hyperrr utilizes three design patterns to resolve cross-module coupling:

### 6.1 Dynamic Service Locator (`rt.Module`)
To prevent hardcoded compiler dependencies and initialization races, modules do not reference each other during instantiation. Instead, they resolve dependencies dynamically during their `Init` phase by querying the shared runtime environment.

For example, inside a `coupon.Module` struct:
```go
type Module struct {
	prodMod  product.ModuleInterface
	orderMod order.ModuleInterface  
}

func (m *Module) Init(ctx context.Context, rt mdk.Runtime) error {
	// Look up peer modules from shared runtime context
	if prodVal, ok := rt.Module("commerce.product"); ok {
		m.prodMod = prodVal.(product.ModuleInterface)
	}
	if orderVal, ok := rt.Module("commerce.order"); ok {
		m.orderMod = orderVal.(order.ModuleInterface)
	}
	return nil
}
```

### 6.2 Soft Database Relations
Avoid using GORM's automatic `BelongsTo` or `HasMany` relation pointers across module-owned database structs. Instead, store foreign references as simple data types (e.g., `ProductID string`) and resolve references dynamically by invoking the referenced module's repository at query time.

### 6.3 Asynchronous Decoupling via Event Fabric
Instead of calling a peer module's methods directly (which creates synchronous compile-time coupling), publish and subscribe to events on the `EventBus`.

```go
// Inside coupon module initialization:
rt.Bus().Subscribe(ctx, "order.finalized", func(ctx context.Context, event mdk.Event) error {
	// Mark corresponding coupon as applied
	return nil
})
```
