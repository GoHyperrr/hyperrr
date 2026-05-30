package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/commerce/cart"
	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/commerce/finance"
	"github.com/GoHyperrr/hyperrr/commerce/notification"
	"github.com/GoHyperrr/hyperrr/commerce/fulfillment"
	"github.com/GoHyperrr/hyperrr/commerce/support"
	"github.com/GoHyperrr/hyperrr/commerce/marketing"
	"github.com/GoHyperrr/hyperrr/commerce/search"
	"github.com/GoHyperrr/hyperrr/commerce/analytics"
	"github.com/GoHyperrr/hyperrr/modules/auth"
	"github.com/GoHyperrr/hyperrr/internal"
	"github.com/GoHyperrr/hyperrr/api/graph"
	ctxEngine "github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/modules/identity"
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

	// 2. Set Auth Key
	if cfg.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is missing from configuration")
	}

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
	runner := workflow.NewRunner(bus, nil)
	registryStore := workflow.NewRegistry()

	// 5. Register Core Modules
	ctxMod := ctxEngine.NewModule()
	registry.Register(ctxMod)
	identMod := identity.NewModule()
	registry.Register(identMod)
	authMod := auth.NewModule()
	registry.Register(authMod)
	registry.Register(storage.NewModule())
	
	// Register Commerce Modules
	prodMod := product.NewModule()
	registry.Register(prodMod)
	custMod := customer.NewModule()
	registry.Register(custMod)
	cartMod := cart.NewModule()
	registry.Register(cartMod)
	orderMod := order.NewModule()
	registry.Register(orderMod)
	financeMod := finance.NewModule()
	registry.Register(financeMod)
	notifMod := notification.NewModule(nil)
	registry.Register(notifMod)
	fulfillMod := fulfillment.NewModule()
	registry.Register(fulfillMod)
	supportMod := support.NewModule()
	registry.Register(supportMod)
	marketingMod := marketing.NewModule()
	registry.Register(marketingMod)
	searchMod := search.NewModule()
	registry.Register(searchMod)
	searchMod.SetProductModule(prodMod)
	analyticsMod := analytics.NewModule()
	registry.Register(analyticsMod)

	// 6. Discover and Initialize Modules (Plugins)
	deps := &registry.Dependencies{
		Config:    cfg,
		DB:        database,
		EventBus:  bus,
		Runner:    runner,
		Registry:  registryStore,
	}

	modules := registry.List()
	
	// Ensure cleanup on error or exit
	defer func() {
		for _, mod := range modules {
			if err := mod.Shutdown(context.Background()); err != nil {
				logger.Error("failed to shutdown module", "id", mod.ID(), "error", err)
			}
		}
	}()

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
			CartModule:     cartMod,
			OrderModule:    orderMod,
			FinanceModule:  financeMod,
			NotificationModule: notifMod,
			FulfillmentModule:  fulfillMod,
			SupportModule:      supportMod,
			MarketingModule:    marketingMod,
			SearchModule:       searchMod,
			AnalyticsModule:    analyticsMod,
			IdentityModule:     identMod,
			AuthModule:         authMod,

			Runner:         runner,
			Registry:       registryStore,
		},
	}))

	// Middleware chain
	authMW := middleware.AuthMiddleware(authMod.Store())

	mux := http.NewServeMux()
	mux.Handle("/", authMW(playground.Handler("GraphQL playground", "/query")))
	mux.Handle("/query", authMW(srv))

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	logger.Info("Server is ready", "addr", addr, "playground", "http://localhost"+addr)

	if cfg.AppEnv == "test" {
		return nil
	}

	return http.ListenAndServe(addr, mux)
}
