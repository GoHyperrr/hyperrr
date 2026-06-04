package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/mdk"
	"gorm.io/gorm"
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

type runtimeImpl struct {
	db  *gorm.DB
	cfg *config.Config
}

func (r *runtimeImpl) DB() *gorm.DB                  { return r.db }
func (r *runtimeImpl) Bus() mdk.EventBus             { return nil }
func (r *runtimeImpl) Workflows() mdk.WorkflowEngine { return nil }
func (r *runtimeImpl) Logger() *slog.Logger          { return slog.Default() }
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
	default:
		return nil
	}
}

