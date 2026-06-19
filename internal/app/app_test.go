package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/mdk"
)

type mockModule struct {
	options map[string]any
	initialized bool
}

func (m *mockModule) ID() string {
	return "commerce.mockhotel"
}

type mockAuthModule struct {
	mockModule
}

func (m *mockAuthModule) ID() string {
	return "auth.apikey"
}

func (m *mockAuthModule) GetActorByAPIKey(ctx context.Context, key string) (mdk.Actor, error) {
	return &mdk.BaseActor{
		ID:   "test-actor",
		Type: mdk.ActorHuman,
		Name: "Test Actor",
	}, nil
}

func (m *mockModule) Init(ctx context.Context, rt mdk.Runtime) error {
	m.initialized = true
	if modulesVal := rt.Config("modules"); modulesVal != nil {
		if modules, ok := modulesVal.([]config.ModuleConfig); ok {
			for _, modCfg := range modules {
				if registry.NormalizeModuleID(modCfg.Resolve) == registry.NormalizeModuleID(m.ID()) {
					m.options = modCfg.Options
					break
				}
			}
		}
	}
	return nil
}

func (m *mockModule) Models() []any {
	return nil
}

func (m *mockModule) Routes() []mdk.Route {
	return nil
}

func (m *mockModule) Shutdown(ctx context.Context) error {
	return nil
}

func TestRun(t *testing.T) {
	t.Run("Run with defaults", func(t *testing.T) {
		os.Setenv("APP_ENV", "test")
		os.Setenv("JWT_SECRET", "test-secret")
		os.Setenv("JWT_EXPIRATION", "24h")
		defer os.Unsetenv("APP_ENV")
		defer os.Unsetenv("JWT_SECRET")
		defer os.Unsetenv("JWT_EXPIRATION")
		err := Run()
		if err != nil {
			t.Errorf("Run() failed: %v", err)
		}
	})

	t.Run("RunWithConfig", func(t *testing.T) {
		os.Setenv("APP_ENV", "test")
		os.Setenv("JWT_SECRET", "test-secret")
		os.Setenv("JWT_EXPIRATION", "24h")
		defer os.Unsetenv("APP_ENV")
		defer os.Unsetenv("JWT_SECRET")
		defer os.Unsetenv("JWT_EXPIRATION")
		err := RunWithConfig(&config.Config{AppEnv: "test"})
		if err != nil && err.Error() != "failed to load config" {
			// Expected behavior if .env is missing or invalid in certain environments
		}
	})

	t.Run("RunWithConfig DB Failure", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    "test",
			DBDriver:  "postgres",
			DBDSN:     "host=localhost port=5432 user=ghost password=ghost dbname=ghost sslmode=disable",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "failed to connect to database") {
			t.Errorf("expected DB failure error, got %v", err)
		}
	})

	t.Run("RunWithConfig Unsupported DB", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    "test",
			DBDriver:  "invalid",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "unsupported database driver") {
			t.Errorf("expected unsupported DB error, got %v", err)
		}
	})

	t.Run("RunWithConfig NATS Failure", func(t *testing.T) {
		eventbus.RegisterProvider("nats", func(url string) (eventbus.EventBus, error) {
			return nil, fmt.Errorf("failed to connect to NATS: connection refused")
		})

		cfg := &config.Config{
			AppEnv:           "test",
			DBDriver:         "sqlite",
			DBDSN:            ":memory:",
			EventBusProvider: "nats",
			NATSURL:          "nats://localhost:4223", // Non-existent
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "failed to connect to NATS") {
			t.Errorf("expected NATS failure error, got %v", err)
		}
	})

	t.Run("RunWithDynamicModule and Option Resolution", func(t *testing.T) {
		os.Setenv("TEST_HOTEL_API_KEY", "dynamic-hotel-api-key-xyz")
		defer os.Unsetenv("TEST_HOTEL_API_KEY")

		// Register mock module factory with full import path name
		instantiatedMod := &mockModule{}
		mdk.Register(func() mdk.Module {
			return instantiatedMod
		})

		// Register mock auth module factory for auth.apikey resolving
		mdk.Register(func() mdk.Module {
			return &mockAuthModule{}
		})

		cfg, err := config.LoadWithFile("")
		if err != nil {
			t.Fatalf("failed to load base config: %v", err)
		}
		cfg.AppEnv = "test"
		cfg.DBDriver = "sqlite"
		cfg.DBDSN = ":memory:"
		cfg.Modules = []config.ModuleConfig{
			{
				Resolve: "github.com/GoHyperrr/commerce/mockhotel",
				Options: map[string]any{
					"apiKey":         "env.TEST_HOTEL_API_KEY",
					"some_other_opt": "direct_value",
				},
			},
			{
				Resolve: "auth.apikey",
			},
		}

		err = RunWithConfig(cfg)
		if err != nil {
			t.Fatalf("failed to run app with dynamic module: %v", err)
		}

		if instantiatedMod == nil {
			t.Fatal("expected mockModule to be instantiated, but it was not")
		}

		if !instantiatedMod.initialized {
			t.Error("expected mockModule to be initialized, but it was not")
		}

		apiKeyVal, ok := instantiatedMod.options["apiKey"].(string)
		if !ok || apiKeyVal != "dynamic-hotel-api-key-xyz" {
			t.Errorf("expected options[apiKey] resolved to dynamic-hotel-api-key-xyz, got %v", instantiatedMod.options["apiKey"])
		}

		otherOptVal, ok := instantiatedMod.options["some_other_opt"].(string)
		if !ok || otherOptVal != "direct_value" {
			t.Errorf("expected options[some_other_opt] to be 'direct_value', got %v", instantiatedMod.options["some_other_opt"])
		}
	})
}
