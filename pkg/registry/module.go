package registry

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/identity"
	"github.com/GoHyperrr/hyperrr/pkg/locking"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/mdk"
	"gorm.io/gorm"
)

// ActorResolver defines the interface for resolving identities.
type ActorResolver interface {
	GetActorByAPIKey(ctx context.Context, key string) (*identity.Actor, error)
}

// WorkflowRunner defines the interface for executing workflows.
type WorkflowRunner interface {
	ExecuteSyncWorkflow(ctx context.Context, id string, wf *workflow.Workflow, input any) (map[string]any, error)
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

// Satisfy mdk.Runtime interface via depsRuntime wrapper.

type depsRuntime struct {
	deps *Dependencies
}

func (r *depsRuntime) DB() *gorm.DB {
	if r.deps.DB == nil {
		return nil
	}
	return r.deps.DB.DB
}

func (r *depsRuntime) Bus() mdk.EventBus {
	return r.deps.EventBus
}

func (r *depsRuntime) Workflows() mdk.WorkflowEngine {
	if engine, ok := r.deps.Runner.(mdk.WorkflowEngine); ok {
		return engine
	}
	return nil
}

func (r *depsRuntime) Logger() *slog.Logger {
	return logger.Get().Logger
}

func (r *depsRuntime) Module(id string) (mdk.Module, bool) {
	return Get(id)
}

func (r *depsRuntime) Config(key string) any {
	if r.deps.Config == nil {
		return nil
	}
	switch strings.ToLower(key) {
	case "appname", "app_name":
		return r.deps.Config.AppName
	case "appenv", "app_env":
		return r.deps.Config.AppEnv
	case "loglevel", "log_level":
		return r.deps.Config.LogLevel
	case "serverport", "server_port":
		return r.deps.Config.ServerPort
	case "storagebucketurl", "storage_bucket_url":
		return r.deps.Config.StorageBucketURL
	case "natsurl", "nats_url":
		return r.deps.Config.NATSURL
	case "modules":
		return r.deps.Config.Modules
	default:
		return nil
	}
}

// NewRuntime wraps Dependencies to satisfy the mdk.Runtime interface.
func NewRuntime(d *Dependencies) mdk.Runtime {
	return &depsRuntime{deps: d}
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
type LineageData = mdk.LineageData

// Projector defines the interface for querying execution lineages.
type Projector = mdk.Projector

// OrderResult defines the minimal interface for accessing order data across modules.
type OrderResult interface {
	GetOrderID() string
	GetTotal() float64
	GetCustomerID() string
}

// Type aliases linking to mdk module/mcp structures to preserve backward compatibility.
type Module = mdk.Module
type Route = mdk.Route
type Factory = mdk.Factory
type MCPResource = mdk.MCPResource
type ResourceProvider = mdk.ResourceProvider
type MCPPromptArgument = mdk.MCPPromptArgument
type MCPPrompt = mdk.MCPPrompt
type MCPPromptMessageContent = mdk.MCPPromptMessageContent
type MCPPromptMessage = mdk.MCPPromptMessage
type GetPromptResult = mdk.GetPromptResult
type PromptProvider = mdk.PromptProvider

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
	Run         func(rt mdk.Runtime, args []string) error
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



