//go:generate go run ../../scripts/gen_imports.go
package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/GoHyperrr/commerce/product"
	"github.com/GoHyperrr/commerce/customer"
	"github.com/GoHyperrr/commerce/cart"
	"github.com/GoHyperrr/commerce/order"
	"github.com/GoHyperrr/commerce/finance"
	"github.com/GoHyperrr/commerce/notification"
	"github.com/GoHyperrr/commerce/fulfillment"
	"github.com/GoHyperrr/commerce/support"
	"github.com/GoHyperrr/commerce/marketing"
	"github.com/GoHyperrr/commerce/search"
	"github.com/GoHyperrr/commerce/analytics"
	"github.com/GoHyperrr/hyperrr/internal"
	"github.com/GoHyperrr/hyperrr/api/graph"
	"github.com/GoHyperrr/hyperrr/api/mcp"
	ctxEngine "github.com/GoHyperrr/hyperrr/pkg/ctxengine"
	"github.com/GoHyperrr/hyperrr/internal/storage"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/locking"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
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
	registerBuiltInFactories()

	// If no modules are configured, default to loading all built-in commerce and auth modules
	modulesToLoad := cfg.Modules
	if len(modulesToLoad) == 0 {
		modulesToLoad = []config.ModuleConfig{
			{Resolve: "auth.emailpass"},
			{Resolve: "auth.apikey"},
			{Resolve: "commerce.product"},
			{Resolve: "commerce.customer"},
			{Resolve: "commerce.cart"},
			{Resolve: "commerce.order"},
			{Resolve: "commerce.finance"},
			{Resolve: "commerce.notification"},
			{Resolve: "commerce.fulfillment"},
			{Resolve: "commerce.support"},
			{Resolve: "commerce.marketing"},
			{Resolve: "commerce.search"},
			{Resolve: "commerce.analytics"},
		}
	}

	// Dynamic Module Registration
	for _, mCfg := range modulesToLoad {
		lookupID := mCfg.ID
		if lookupID == "" {
			lookupID = mCfg.Resolve
		}
		factory, ok := registry.GetFactory(lookupID)
		if !ok {
			return fmt.Errorf("module factory not found for resolve path or ID: %s (resolve: %s)", lookupID, mCfg.Resolve)
		}

		mod, err := factory(mCfg.Options)
		if err != nil {
			return fmt.Errorf("failed to instantiate module %s: %w", mCfg.Resolve, err)
		}

		registry.Register(mod)
	}

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
		logger.Info("Initializing module", "id", mod.ID())

		// Auto-register models
		if models := mod.Models(); len(models) > 0 {
			db.Register(models...)
		}

		// Auto-register task handlers
		for name, handler := range mod.Handlers() {
			runner.RegisterTask(name, handler)
		}

		// Module-specific initialization
		if err := mod.Init(context.Background(), deps); err != nil {
			return fmt.Errorf("failed to initialize module %s: %w", mod.ID(), err)
		}
	}

	// 8. Run database migrations for all registered models
	if err := database.AutoMigrateAll(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	// 9. Register system.about tool & workflow for AI agent context
	runner.RegisterTask("system.about", func(ctx context.Context, input any) (any, error) {
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
		return info, nil
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
				Uses: "system.about",
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

	// Dynamic Middleware chain
	var h http.Handler = srv
	var playH http.Handler = playground.Handler("GraphQL playground", "/query")
	
	if authMW, ok := registry.GetMiddleware("auth"); ok {
		h = authMW(h)
		playH = authMW(playH)
	}

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

func registerBuiltInFactories() {
	registry.RegisterFactory("commerce.product", func(opts map[string]any) (registry.Module, error) {
		return product.NewModule(), nil
	})
	registry.RegisterFactory("commerce.customer", func(opts map[string]any) (registry.Module, error) {
		return customer.NewModule(), nil
	})
	registry.RegisterFactory("commerce.cart", func(opts map[string]any) (registry.Module, error) {
		return cart.NewModule(), nil
	})
	registry.RegisterFactory("commerce.order", func(opts map[string]any) (registry.Module, error) {
		return order.NewModule(), nil
	})
	registry.RegisterFactory("commerce.finance", func(opts map[string]any) (registry.Module, error) {
		return finance.NewModule(), nil
	})
	registry.RegisterFactory("commerce.notification", func(opts map[string]any) (registry.Module, error) {
		return notification.NewModule(nil), nil
	})
	registry.RegisterFactory("commerce.fulfillment", func(opts map[string]any) (registry.Module, error) {
		return fulfillment.NewModule(), nil
	})
	registry.RegisterFactory("commerce.support", func(opts map[string]any) (registry.Module, error) {
		return support.NewModule(), nil
	})
	registry.RegisterFactory("commerce.marketing", func(opts map[string]any) (registry.Module, error) {
		return marketing.NewModule(), nil
	})
	registry.RegisterFactory("commerce.search", func(opts map[string]any) (registry.Module, error) {
		return search.NewModule(), nil
	})
	registry.RegisterFactory("commerce.analytics", func(opts map[string]any) (registry.Module, error) {
		return analytics.NewModule(), nil
	})
}
