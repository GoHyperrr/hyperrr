package registry

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/locking"
)

// ActorResolver defines the interface for resolving identities.
type ActorResolver interface {
	GetActorByAPIKey(ctx context.Context, key string) (*identity.Actor, error)
}

// WorkflowRunner defines the interface for executing workflows.
type WorkflowRunner interface {
	Execute(ctx context.Context, id string, wf *workflow.Workflow, input any) (map[string]any, error)
}

// WorkflowRegistry defines the interface for managing workflow definitions.
type WorkflowRegistry interface {
	Register(wf *workflow.Workflow) error
	Get(name string) (*workflow.Workflow, error)
	List() []*workflow.Workflow
}

// Dependencies provides common utilities to modules.
type Dependencies struct {
	Config    *config.Config
	DB        *db.DB
	EventBus  eventbus.EventBus
	Runner    WorkflowRunner
	Registry  WorkflowRegistry
	Locker    locking.Locker
	Resolver  ActorResolver
}

// Middleware is a standard HTTP middleware function.
type Middleware func(http.Handler) http.Handler

var (
	middlewareMu sync.RWMutex
	middlewares  = make(map[string]Middleware)
)

// RegisterMiddleware adds a named middleware to the registry.
func RegisterMiddleware(name string, mw Middleware) {
	middlewareMu.Lock()
	defer middlewareMu.Unlock()
	middlewares[name] = mw
}

// GetMiddleware retrieves a middleware by name.
func GetMiddleware(name string) (Middleware, bool) {
	middlewareMu.RLock()
	defer middlewareMu.RUnlock()
	mw, ok := middlewares[name]
	return mw, ok
}

// LineageData defines the minimal interface for accessing workflow execution data.
type LineageData interface {
	GetID() string
	GetName() string
	GetState() string
	GetError() string
	GetStartedAt() time.Time
	GetEndedAt() *time.Time
}

// Projector defines the interface for querying execution lineages.
type Projector interface {
	ListLineages() []LineageData
	QueryLineages(filter func(LineageData) bool) []LineageData
}

// OrderResult defines the minimal interface for accessing order data across modules.
type OrderResult interface {
	GetOrderID() string
	GetTotal() float64
	GetCustomerID() string
}

// MCPResource represents a discoverable data resource exposed to AI agents.
type MCPResource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
}

// ResourceProvider is implemented by modules that want to expose resources to MCP.
type ResourceProvider interface {
	ListResources(ctx context.Context) ([]MCPResource, error)
	ReadResource(ctx context.Context, uri string) (string, error)
}


// Module is the standard interface for all hyperrr plugins.
type Module interface {
	// ID returns the unique identifier for the module.
	ID() string
	
	// Init initializes the module with its dependencies.
	Init(ctx context.Context, deps *Dependencies) error
	
	// Models returns the GORM models owned by this module.
	Models() []any
	
	// Handlers returns the workflow task handlers provided by this module.
	Handlers() map[string]workflow.TaskHandler

	// Shutdown allows the module to release resources cleanly.
	Shutdown(ctx context.Context) error
}

// TUIPage represents a self-contained view or tab in the admin panel.
type TUIPage interface {
	// Title returns the display name for the navigation tab.
	Title() string

	// Init initializes the page state with runtime dependencies.
	// Returns a Bubble Tea command (cast to any).
	Init(ctx context.Context, deps *Dependencies) any

	// Update processes input keystrokes or system events for the page.
	// Returns the updated page and an optional Bubble Tea command.
	Update(msg any) (TUIPage, any)

	// View renders the layout screen of the page.
	View() string
}

// TUIProvider is implemented by modules that want to expose custom admin views.
type TUIProvider interface {
	TUIPages() []TUIPage
}

