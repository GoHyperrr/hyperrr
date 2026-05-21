package app

import (
	"fmt"

	"github.com/GoHyperrr/hyperrr/internal"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
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
	logger.Info("Config loaded", "app", cfg.AppName, "env", cfg.AppEnv)

	return nil
}
