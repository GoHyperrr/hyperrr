package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/GoHyperrr/hyperrr/internal"
	domain "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/context/graph"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
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

	// Initialize Logger
	l := logger.New(&logger.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})
	logger.SetGlobal(l)

	logger.Info("Starting hyperrr", "version", internal.Version)

	// Initialize Event Fabric
	bus := eventbus.NewInMemBus()

	// Initialize Context Engine (Projector)
	projector := domain.NewProjector(bus)
	if err := projector.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start context projector: %w", err)
	}

	// Setup GraphQL
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{Projector: projector},
	}))

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	logger.Info("Server is ready", "addr", addr, "playground", "http://localhost"+addr)

	// In a real app, we'd handle graceful shutdown here.
	// For MVP/Testing, we'll start it. 
	// Note: http.ListenAndServe is blocking.
	
	// Check if we're in a test environment to avoid blocking tests
	if cfg.AppEnv == "test" {
		return nil
	}

	return http.ListenAndServe(addr, nil)
}
