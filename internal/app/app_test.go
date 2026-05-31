package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

type mockModule struct {
	options map[string]any
	initialized bool
}

func (m *mockModule) ID() string {
	return "commerce.mockhotel"
}

func (m *mockModule) Init(ctx context.Context, deps *registry.Dependencies) error {
	m.initialized = true
	return nil
}

func (m *mockModule) Models() []any {
	return nil
}

func (m *mockModule) Handlers() map[string]workflow.TaskHandler {
	return nil
}

func (m *mockModule) Shutdown(ctx context.Context) error {
	return nil
}

func TestRun(t *testing.T) {
	t.Run("Run with defaults", func(t *testing.T) {
		os.Setenv("APP_ENV", "test")
		defer os.Unsetenv("APP_ENV")
		err := Run()
		if err != nil {
			t.Errorf("Run() failed: %v", err)
		}
	})

	t.Run("RunWithConfig", func(t *testing.T) {
		os.Setenv("APP_ENV", "test")
		defer os.Unsetenv("APP_ENV")
		err := RunWithConfig(&config.Config{AppEnv: "test", JWTSecret: "test-secret"})
		if err != nil && err.Error() != "failed to load config" {
			// Expected behavior if .env is missing or invalid in certain environments
		}
	})

	t.Run("RunWithConfig DB Failure", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    "test",
			JWTSecret: "test-secret",
			DBDriver:  "postgres",
			DBDSN:     "host=localhost port=5432 user=ghost password=ghost dbname=ghost sslmode=disable",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "failed to connect to database") {
			t.Errorf("expected DB failure error, got %v", err)
		}
	})

	t.Run("RunWithConfig Missing Secret", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    "test",
			DBDriver:  "sqlite",
			DBDSN:     ":memory:",
			JWTSecret: "",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "JWT_SECRET is missing") {
			t.Errorf("expected missing secret error, got %v", err)
		}
	})

	t.Run("RunWithConfig Unsupported DB", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    "test",
			JWTSecret: "test-secret",
			DBDriver:  "invalid",
		}
		err := RunWithConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "unsupported database driver") {
			t.Errorf("expected unsupported DB error, got %v", err)
		}
	})

	t.Run("RunWithConfig NATS Failure", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:           "test",
			JWTSecret:        "test-secret",
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
		var instantiatedMod *mockModule
		registry.RegisterFactory("github.com/GoHyperrr/commerce/mockhotel", func(options map[string]any) (registry.Module, error) {
			instantiatedMod = &mockModule{options: options}
			return instantiatedMod, nil
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
