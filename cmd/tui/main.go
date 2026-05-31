package main

import (
	"context"
	"fmt"
	"os"

	"github.com/GoHyperrr/hyperrr/internal/app"
	"github.com/GoHyperrr/hyperrr/internal/tui"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "github.com/charmbracelet/bubbletea"
)

var teaRun = func(m tea.Model) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

var osExit = os.Exit

func main() {
	if err := run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		osExit(1)
	}
}

func run() error {
	// 1. Load active configurations
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 2. Set AppEnv to "test" to boot and register all modules
	// without launching the HTTP server.
	cfg.AppEnv = "test"
	cfg.DBDriver = "sqlite"
	cfg.DBDSN = "hyperrr.db"

	err = app.RunWithConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize modules: %w", err)
	}

	// 3. Re-connect to database to build dependencies wrapper
	database, err := db.Connect(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	deps := &registry.Dependencies{
		Config: cfg,
		DB:     database,
	}

	// 4. Create and run the master TUI
	m := tui.NewModel(context.Background(), deps)
	return teaRun(m)
}
