//go:generate go run ../../scripts/gen_imports.go
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/GoHyperrr/hyperrr"
	"github.com/GoHyperrr/hyperrr/api/graph"
	"github.com/GoHyperrr/hyperrr/api/mcp"
	apiMiddleware "github.com/GoHyperrr/hyperrr/api/middleware"
	ctxEngine "github.com/GoHyperrr/hyperrr/pkg/ctxengine"
	"github.com/GoHyperrr/hyperrr/internal/storage"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/locking"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/mdk"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"gorm.io/gorm"
)

// Run initializes and starts the hyperrr application.
func Run() error {
	return RunWithConfig(nil)
}

// RunWithConfig initializes and starts the hyperrr application with a specific config.
func RunWithConfig(cfg *config.Config) error {
	if cfg == nil {
		var err error
		cfg, err = config.LoadWithFile("") // Load defaults if no file provided
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Resolve environment variables in module options
	cfg.ResolveEnvOptions()

	// 1. Initialize Logger
	l := logger.New(&logger.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})
	logger.SetGlobal(l)

	logger.Info("Starting hyperrr", "version", hyperrr.Version)

	// 3. Initialize Database
	database, err := db.Connect(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 4. Initialize Event Fabric
	var bus eventbus.EventBus
	if cfg.EventBusProvider == "inmem" || cfg.EventBusProvider == "" {
		bus = eventbus.NewInMemBus()
	} else {
		provider, ok := eventbus.GetProvider(cfg.EventBusProvider)
		if !ok {
			return fmt.Errorf("unsupported event bus provider: %s (did you register the event-bus module?)", cfg.EventBusProvider)
		}
		var err error
		bus, err = provider(cfg.NATSURL)
		if err != nil {
			return fmt.Errorf("failed to connect to event bus %s: %w", cfg.EventBusProvider, err)
		}
		if tiedBus, ok := bus.(eventbus.ContextTiedBus); ok {
			tiedBus.SetContext(context.Background())
		}
	}
	defer bus.Close()

	// 5. Initialize Workflow Engine & Locking
	var wfStore workflow.StateStore
	var wfLocker locking.Locker

	if cfg.WorkflowStoreType == "mem" || cfg.WorkflowStoreType == "" {
		wfStore = workflow.NewInMemStore()
	} else {
		provider, ok := workflow.GetStore(cfg.WorkflowStoreType)
		if ok {
			var err error
			wfStore, err = provider(cfg.NATSURL, cfg.NATSStateBucket)
			if err != nil {
				return fmt.Errorf("failed to initialize %s workflow store: %w", cfg.WorkflowStoreType, err)
			}
		} else {
			logger.Warn("Workflow store provider not found, falling back to in-memory store", "provider", cfg.WorkflowStoreType)
			wfStore = workflow.NewInMemStore()
		}
	}

	if cfg.EventBusProvider == "inmem" || cfg.EventBusProvider == "" {
		wfLocker = locking.NewInMemLocker()
	} else {
		provider, ok := locking.GetLocker(cfg.EventBusProvider)
		if ok {
			var err error
			wfLocker, err = provider(cfg.NATSURL, cfg.NATSLocksBucket)
			if err != nil {
				return fmt.Errorf("failed to initialize %s locker: %w", cfg.EventBusProvider, err)
			}
		} else {
			logger.Warn("Locker provider not found, falling back to in-memory locker", "provider", cfg.EventBusProvider)
			wfLocker = locking.NewInMemLocker()
		}
	}

	runner := workflow.NewRunner(bus, wfStore, wfLocker)
	registryStore := workflow.NewRegistry()

	// 6. Register Core Modules
	ctxMod := ctxEngine.NewModule()
	registry.Register(ctxMod)
	registry.Register(storage.NewModule())

	// Register built-in factories
	// If no modules are configured, default to loading all built-in commerce and auth modules
	modulesToLoad := cfg.Modules
	factories := mdk.Registered()

	if len(modulesToLoad) == 0 {
		for _, factory := range factories {
			mod := factory()
			registry.Register(mod)
		}
	} else {
		// Dynamic Module Registration
		for _, mCfg := range modulesToLoad {
			lookupID := mCfg.ID
			if lookupID == "" {
				lookupID = mCfg.Resolve
			}
			normalizedID := registry.NormalizeModuleID(lookupID)

			factory, ok := factories[normalizedID]
			if !ok {
				factory, ok = factories[lookupID]
			}
			if !ok {
				return fmt.Errorf("module factory not found for resolve path or ID: %s", lookupID)
			}

			mod := factory()
			registry.Register(mod)
		}
	}

	// Create dynamic Runtime wrapper
	rt := &runtimeImpl{
		db:        database.DB,
		bus:       bus,
		workflows: runner,
		cfg:       cfg,
		logger:    logger.Get().Logger,
	}
	runner.SetRuntime(rt)

	// 7. Discover and Initialize Modules (Plugins)
	deps := &registry.Dependencies{
		Config:    cfg,
		DB:        database,
		EventBus:  bus,
		Runner:    runner,
		Registry:  registryStore,
		Locker:    wfLocker,
	}

	modules := registry.List()

	// Resolve the ActorResolver from loaded modules
	for _, mod := range modules {
		if resolver, ok := mod.(registry.ActorResolver); ok {
			deps.Resolver = resolver
			break
		}
	}

	// Fallback to anonymous/system resolver if none loaded (for true decoupling and bare-slate out-of-the-box run)
	if deps.Resolver == nil {
		logger.Warn("No active identity resolver loaded. Falling back to anonymous/system actor resolver.")
		deps.Resolver = &fallbackActorResolver{}
	}

	// Ensure cleanup on error or exit
	defer func() {
		for _, mod := range modules {
			if err := mod.Shutdown(context.Background()); err != nil {
				logger.Error("failed to shutdown module", "id", mod.ID(), "error", err)
			}
		}
		if closer, ok := wfStore.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		if closer, ok := wfLocker.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	initDone := make(chan struct{})

	// Auto-Recovery Goroutine
	if store := runner.GetStateStore(); store != nil {
		go func() {
			<-initDone
			stalled, err := store.ListExecutions(context.Background(), workflow.StateRunning)
			if err != nil {
				logger.Error("failed to list stalled workflows", "error", err)
				return
			}
			for _, id := range stalled {
				// Retrieve workflow name from state checkpoints
				states, err := store.GetState(context.Background(), id)
				if err != nil {
					logger.Error("failed to retrieve states for stalled workflow", "id", id, "error", err)
					continue
				}
				
				wfName := states["__wf_name"]
				if wfName == "" {
					logger.Warn("found stalled workflow without saved workflow name, cannot auto-resume", "id", id)
					continue
				}
				
				wf, err := registryStore.Get(wfName)
				if err != nil {
					logger.Error("failed to find workflow definition in registry for auto-resume", "id", id, "name", wfName, "error", err)
					continue
				}
				
				logger.Info("auto-resuming stalled workflow", "id", id, "name", wfName)
				go func(execID string, w *workflow.Workflow) {
					if _, err := runner.ResumeExecution(context.Background(), execID, w); err != nil {
						logger.Error("failed to auto-resume workflow", "id", execID, "error", err)
					}
				}(id, wf)
			}
		}()
	}

	for _, mod := range modules {
		// Auto-register models
		if models := mod.Models(); len(models) > 0 {
			db.Register(models...)
		}
	}

	// 8. Run database migrations for all registered models
	if err := database.AutoMigrateAll(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	for _, mod := range modules {
		logger.Info("Initializing module", "id", mod.ID())
		// Module-specific initialization
		if err := mod.Init(context.Background(), rt); err != nil {
			return fmt.Errorf("failed to initialize module %s: %w", mod.ID(), err)
		}
	}

	// 9. Register system.about workflow for AI agent context
	if err := runner.RegisterHandler("system.about_handler", func(sCtx mdk.StepContext) mdk.StepResult {
		var activeModules []string
		for _, m := range registry.List() {
			activeModules = append(activeModules, m.ID())
		}

		info := map[string]any{
			"version":        hyperrr.Version,
			"environment":    cfg.AppEnv,
			"current_time":   time.Now().Format(time.RFC3339),
			"active_modules": activeModules,
			"event_bus":      cfg.EventBusProvider,
			"state_store":    cfg.WorkflowStoreType,
		}
		return mdk.StepResult{Output: info}
	}); err != nil {
		return fmt.Errorf("failed to register system.about handler: %w", err)
	}

	if err := registryStore.Register(&workflow.Workflow{
		Name:        "system.about",
		Description: "Returns metadata about the running system, including active modules, version, current server time, and environment configurations.",
		ExposeToAI:  true,
		InputSchema: map[string]any{
			"type": "object",
		},
		Steps: []workflow.Step{
			{
				ID:   "about",
				Name: "About System",
				Uses: "system.about_handler",
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to register system.about workflow: %w", err)
	}

	close(initDone)

	// 10. Setup MCP (Agent Gateway)
	mcpServer := mcp.NewServer(deps)

	// 11. Setup GraphQL
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: graph.NewResolver(registry.List(), runner, registryStore, ctxMod.Projector()),
	}))

	// 9. Setup Auth Middleware
	var tokenValidator apiMiddleware.TokenValidator
	for _, mod := range modules {
		if v, ok := mod.(apiMiddleware.TokenValidator); ok {
			tokenValidator = v
			break
		}
	}

	var h http.Handler = srv
	var playH http.Handler = playground.Handler("GraphQL playground", "/query")

	authMW := apiMiddleware.AuthMiddleware(cfg.AuthProviders, tokenValidator, deps.Resolver)
	h = authMW(h)
	playH = authMW(playH)

	mux := http.NewServeMux()
	mux.Handle("/", playH)
	mux.Handle("/query", h)
	mux.HandleFunc("/mcp/sse", mcpServer.HandleSSE)
	mux.HandleFunc("/mcp/messages", mcpServer.HandleMessages)

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	logger.Info("Server is ready", "addr", addr, "playground", "http://localhost"+addr)

	if cfg.AppEnv == "test" {
		return nil
	}

	return http.ListenAndServe(addr, mux)
}

type runtimeImpl struct {
	db        *gorm.DB
	bus       mdk.EventBus
	workflows mdk.WorkflowEngine
	cfg       *config.Config
	logger    *slog.Logger
}

func (r *runtimeImpl) DB() *gorm.DB { return r.db }
func (r *runtimeImpl) Bus() mdk.EventBus { return r.bus }
func (r *runtimeImpl) Workflows() mdk.WorkflowEngine { return r.workflows }
func (r *runtimeImpl) Logger() *slog.Logger { return r.logger }
func (r *runtimeImpl) Module(id string) (mdk.Module, bool) {
	return registry.Get(id)
}
func (r *runtimeImpl) Config(key string) any {
	switch strings.ToLower(key) {
	case "appname", "app_name":
		return r.cfg.AppName
	case "appenv", "app_env":
		return r.cfg.AppEnv
	case "loglevel", "log_level":
		return r.cfg.LogLevel
	case "serverport", "server_port":
		return r.cfg.ServerPort
	case "storagebucketurl", "storage_bucket_url":
		return r.cfg.StorageBucketURL
	case "natsurl", "nats_url":
		return r.cfg.NATSURL
	case "modules":
		return r.cfg.Modules
	default:
		return nil
	}
}

type fallbackActorResolver struct{}

func (f *fallbackActorResolver) GetActorByAPIKey(ctx context.Context, key string) (mdk.Actor, error) {
	return &mdk.BaseActor{
		ID:   "system-fallback",
		Type: mdk.ActorSystem,
		Name: "System Fallback",
	}, nil
}


