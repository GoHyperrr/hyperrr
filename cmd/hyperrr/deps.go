package main

import (
	"fmt"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Global CLI flags
var (
	cfgFile    string
	verbose    bool
	jsonOutput bool
)

// buildDeps loads the active configuration and conditionally connects to the database.
func buildDeps(needsDB bool) (*registry.Dependencies, error) {
	cfg, err := config.LoadWithFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	var database *db.DB
	if needsDB {
		database, err = db.Connect(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to connect database: %w", err)
		}
	}

	serverURL := fmt.Sprintf("http://localhost:%d", cfg.ServerPort)

	return &registry.Dependencies{
		Config:    cfg,
		DB:        database,
		ServerURL: serverURL,
	}, nil
}
