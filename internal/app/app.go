package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/internal"
	"github.com/GoHyperrr/hyperrr/api/graph"
	ctxEngine "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/GoHyperrr/hyperrr/internal/storage"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/api/middleware"
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

	// 1. Initialize Logger
	l := logger.New(&logger.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})
	logger.SetGlobal(l)

	logger.Info("Starting hyperrr", "version", internal.Version)

	// 2. Initialize Database
	database, err := db.Connect(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 3. Initialize Event Fabric
	bus := eventbus.NewInMemBus()

	// 4. Initialize Workflow Engine
	runner := workflow.NewRunner(bus)

	// 5. Register Core Modules
	ctxMod := ctxEngine.NewModule()
	registry.Register(ctxMod)
	identMod := identity.NewModule()
	registry.Register(identMod)
	registry.Register(storage.NewModule())
	
	// Register Commerce Modules
	prodMod := product.NewModule()
	registry.Register(prodMod)
	custMod := customer.NewModule()
	registry.Register(custMod)

	// 6. Discover and Initialize Modules (Plugins)
	deps := &registry.Dependencies{
		DB:       database,
		EventBus: bus,
		Runner:   runner,
	}

	for _, mod := range registry.List() {
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

	// 7. Run database migrations for all registered models
	if err := database.AutoMigrateAll(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	// 8. Setup GraphQL
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			Projector:      ctxMod.Projector(),
			ProductModule:  prodMod,
			CustomerModule: custMod,
			IdentityModule: identMod,
		},
	}))

	// Middleware chain
	authMW := middleware.AuthMiddleware()

	http.Handle("/", authMW(playground.Handler("GraphQL playground", "/query")))
	http.Handle("/query", authMW(srv))

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	logger.Info("Server is ready", "addr", addr, "playground", "http://localhost"+addr)

	if cfg.AppEnv == "test" {
		return nil
	}

	return http.ListenAndServe(addr, nil)
}
