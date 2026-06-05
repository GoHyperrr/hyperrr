//go:generate go run ../../scripts/gen_imports.go
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/GoHyperrr/hyperrr/internal"
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

	logger.Info("Starting hyperrr", "version", internal.Version)

	// 3. Initialize Database
	database, err := db.Connect(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 4. Initialize Event Fabric
	var bus eventbus.EventBus
	if cfg.EventBusProvider == "nats" {
		var err error
		bus, err = eventbus.NewNATSBus(cfg.NATSURL)
		if err != nil {
			return fmt.Errorf("failed to connect to NATS: %w", err)
		}
		if natsBus, ok := bus.(*eventbus.NATSBus); ok {
			natsBus.SetContext(context.Background())
		}
	} else {
		bus = eventbus.NewInMemBus()
	}
	defer bus.Close()

	// 5. Initialize Workflow Engine & Locking
	var wfStore workflow.StateStore
	var wfLocker locking.Locker

	if natsBus, ok := bus.(*eventbus.NATSBus); ok && cfg.WorkflowStoreType == "nats" {
		natsConn := natsBus.Conn()
		natsStore, err := workflow.NewNATSStore(context.Background(), natsConn, cfg.NATSStateBucket)
		if err != nil {
			return fmt.Errorf("failed to initialize NATS state store: %w", err)
		}
		wfStore = natsStore
		
		natsLocker, err := locking.NewNATSLocker(context.Background(), natsConn, cfg.NATSLocksBucket)
		if err != nil {
			return fmt.Errorf("failed to initialize NATS locker: %w", err)
		}
		wfLocker = natsLocker
	} else {
		wfStore = workflow.NewInMemStore()
		wfLocker = locking.NewInMemLocker()
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

	// Fail-fast guard: ensure an identity resolver is registered
	if deps.Resolver == nil {
		return fmt.Errorf("Configuration Error: No active identity resolver (implementing registry.ActorResolver) is loaded. Please install an auth module (e.g. auth.apikey).")
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
	_ = runner.RegisterHandler("system.about_handler", func(sCtx mdk.StepContext) mdk.StepResult {
		var activeModules []string
		for _, m := range registry.List() {
			activeModules = append(activeModules, m.ID())
		}

		info := map[string]any{
			"version":        internal.Version,
			"environment":    cfg.AppEnv,
			"current_time":   time.Now().Format(time.RFC3339),
			"active_modules": activeModules,
			"event_bus":      cfg.EventBusProvider,
			"state_store":    cfg.WorkflowStoreType,
		}
		return mdk.StepResult{Output: info}
	})

	_ = registryStore.Register(&workflow.Workflow{
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
	})

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

