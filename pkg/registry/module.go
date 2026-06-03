package registry

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/locking"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
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
	ServerURL string
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

// MCPPromptArgument represents an argument for a prompt template.
type MCPPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// MCPPrompt represents a reusable prompt template.
type MCPPrompt struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Arguments   []MCPPromptArgument `json:"arguments,omitempty"`
}

// MCPPromptMessageContent represents the content inside a prompt message.
type MCPPromptMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCPPromptMessage represents a prompt message history node.
type MCPPromptMessage struct {
	Role    string                  `json:"role"`
	Content MCPPromptMessageContent `json:"content"`
}

// GetPromptResult is returned by GetPrompt.
type GetPromptResult struct {
	Description string             `json:"description,omitempty"`
	Messages    []MCPPromptMessage `json:"messages"`
}

// PromptProvider is implemented by modules that want to expose custom prompt templates to LLMs.
type PromptProvider interface {
	ListPrompts(ctx context.Context) ([]MCPPrompt, error)
	GetPrompt(ctx context.Context, name string, arguments map[string]string) (*GetPromptResult, error)
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

// GraphQLProvider is implemented by modules that expose GraphQL queries/mutations.
type GraphQLProvider interface {
	// Queries returns resolver function pointers keyed by GraphQL field name.
	Queries() map[string]any
	// Mutations returns resolver function pointers keyed by GraphQL field name.
	Mutations() map[string]any
	// FieldResolvers returns nested field resolvers (e.g. "WorkflowLineage.events").
	FieldResolvers() map[string]any
}

// CLICommand represents a dynamic subcommand registered by a module.
type CLICommand struct {
	Group       string   // Command group: "auth", "commerce", etc.
	Name        string   // Subcommand name: "apikey", "product", etc.
	Aliases     []string // Alternative names
	Short       string   // One-line description
	Long        string   // Detailed description (shown in --help)
	Usage       string   // Args pattern: "generate", "<email> <password>"
	Run         func(deps *Dependencies, args []string) error
	NeedsDB     bool     // If true, auto-connect DB before Run
	NeedsServer bool     // If true, requires running server (validates connectivity)
}

var (
	commandsMu sync.RWMutex
	commands   = make(map[string]CLICommand)
)

func commandKey(cmd CLICommand) string {
	if cmd.Group != "" {
		return cmd.Group + "/" + cmd.Name
	}
	return cmd.Name
}

// RegisterCommand adds a dynamic CLI subcommand to the registry.
func RegisterCommand(cmd CLICommand) {
	commandsMu.Lock()
	defer commandsMu.Unlock()
	key := commandKey(cmd)
	if existing, ok := commands[key]; ok {
		logger.Warn("dynamic CLI command overwrite detected", "key", key, "existingUsage", existing.Usage, "newUsage", cmd.Usage)
	}
	commands[key] = cmd
}

// GetCommand retrieves a registered custom command by its group/name key.
func GetCommand(key string) (CLICommand, bool) {
	commandsMu.RLock()
	defer commandsMu.RUnlock()
	cmd, ok := commands[key]
	return cmd, ok
}

// ListCommands returns a list of all registered custom CLI commands.
func ListCommands() []CLICommand {
	commandsMu.RLock()
	defer commandsMu.RUnlock()
	res := make([]CLICommand, 0, len(commands))
	for _, cmd := range commands {
		res = append(res, cmd)
	}
	return res
}


