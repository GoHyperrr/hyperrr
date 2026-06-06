package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

// Default configuration constants.
const (
	DefaultAppName           = "hyperrr"
	DefaultAppEnv            = "local"
	DefaultLogLevel          = "info"
	DefaultLogFormat         = "text"
	DefaultServerPort        = 8080
	DefaultEventBusProvider  = "inmem"
	DefaultWorkflowStoreType = "mem"
	DefaultNatsStateBucket   = "hyperrr_state"
	DefaultNatsLocksBucket   = "hyperrr_locks"
	DefaultDBDriver          = "sqlite"
	DefaultDBDSN             = "hyperrr.db"
	DefaultStorageProvider   = "cloud"
	DefaultStorageBucketURL  = "mem://"
	DefaultNatsURL           = "nats://localhost:4222"
	DefaultCurrency          = "USD"
)

// ModuleConfig represents registration info for a dynamic module.
type ModuleConfig struct {
	Resolve string         `mapstructure:"resolve" json:"resolve" yaml:"resolve"`
	ID      string         `mapstructure:"id" json:"id" yaml:"id"`
	Options map[string]any `mapstructure:"options" json:"options" yaml:"options"`
}

// Config represents the application configuration.
type Config struct {
	AppName           string         `mapstructure:"APP_NAME"`
	AppEnv            string         `mapstructure:"APP_ENV"`
	LogLevel          string         `mapstructure:"LOG_LEVEL"`
	LogFormat         string         `mapstructure:"LOG_FORMAT"`
	ServerPort        int            `mapstructure:"SERVER_PORT"`
	EventBusProvider  string         `mapstructure:"EVENT_BUS_PROVIDER"`
	WorkflowStoreType string         `mapstructure:"WORKFLOW_STORE_TYPE"`
	NATSStateBucket   string         `mapstructure:"NATS_STATE_BUCKET"`
	NATSLocksBucket   string         `mapstructure:"NATS_LOCKS_BUCKET"`
	DBDriver          string         `mapstructure:"DB_DRIVER"`
	DBDSN             string         `mapstructure:"DB_DSN"`
	StorageProvider   string         `mapstructure:"STORAGE_PROVIDER"`
	StoragePath       string         `mapstructure:"STORAGE_PATH"`
	StorageBucketURL  string         `mapstructure:"STORAGE_BUCKET_URL"`
	NATSURL           string         `mapstructure:"NATS_URL"`
	Currency          string         `mapstructure:"CURRENCY"`
	MCPAuthProviders  []string       `mapstructure:"MCP_AUTH_PROVIDERS"`
	AuthProviders     []string       `mapstructure:"AUTH_PROVIDERS"`
	Modules           []ModuleConfig `mapstructure:"modules"`
}

// Load loads the configuration.
func Load() (*Config, error) {
	return LoadWithFile("")
}

var envPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandEnvConfig(data []byte) []byte {
	return envPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		expr := string(match[2 : len(match)-1]) // strip `${` and `}`
		
		varName := expr
		defaultVal := ""
		if idx := strings.Index(expr, ":"); idx != -1 {
			varName = expr[:idx]
			defaultVal = expr[idx+1:]
		}

		if strings.HasPrefix(varName, "env.") {
			varName = strings.TrimPrefix(varName, "env.")
		}

		val := os.Getenv(varName)
		if val == "" {
			return []byte(defaultVal)
		}
		return []byte(val)
	})
}

func locateConfigFile(filename string) (string, error) {
	if filename != "" {
		if _, err := os.Stat(filename); err == nil {
			return filename, nil
		}
		return "", os.ErrNotExist
	}

	searchPaths := []string{".", "./configs"}
	extensions := []string{"yml", "yaml", "json", "toml"}
	for _, path := range searchPaths {
		for _, ext := range extensions {
			fullName := filepath.Join(path, "hyperrr."+ext)
			if _, err := os.Stat(fullName); err == nil {
				return fullName, nil
			}
		}
	}
	return "", fmt.Errorf("config file not found")
}

// LoadWithFile loads the configuration from a specific file.
func LoadWithFile(filename string) (*Config, error) {
	v := viper.New()
	v.SetDefault("APP_NAME", DefaultAppName)
	v.SetDefault("APP_ENV", DefaultAppEnv)
	v.SetDefault("LOG_LEVEL", DefaultLogLevel)
	v.SetDefault("LOG_FORMAT", DefaultLogFormat)
	v.SetDefault("SERVER_PORT", DefaultServerPort)
	v.SetDefault("EVENT_BUS_PROVIDER", DefaultEventBusProvider)
	v.SetDefault("WORKFLOW_STORE_TYPE", DefaultWorkflowStoreType)
	v.SetDefault("NATS_STATE_BUCKET", DefaultNatsStateBucket)
	v.SetDefault("NATS_LOCKS_BUCKET", DefaultNatsLocksBucket)
	v.SetDefault("DB_DRIVER", DefaultDBDriver)
	v.SetDefault("DB_DSN", DefaultDBDSN)
	v.SetDefault("STORAGE_PROVIDER", DefaultStorageProvider)
	v.SetDefault("STORAGE_BUCKET_URL", DefaultStorageBucketURL)
	v.SetDefault("NATS_URL", DefaultNatsURL)
	v.SetDefault("CURRENCY", DefaultCurrency)
	v.SetDefault("MCP_AUTH_PROVIDERS", []string{"apikey"})
	v.SetDefault("AUTH_PROVIDERS", []string{"jwt"})

	v.AutomaticEnv()

	filePath, err := locateConfigFile(filename)
	if err == nil {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		expanded := expandEnvConfig(data)

		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == "" {
			ext = ".yaml"
		}
		v.SetConfigType(ext[1:])
		if err := v.ReadConfig(bytes.NewBuffer(expanded)); err != nil {
			return nil, fmt.Errorf("failed to parse config content: %w", err)
		}
	} else {
		if filename != "" {
			return nil, fmt.Errorf("config file not found: %s", filename)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	resolveEnvOptions(v, cfg.Modules)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks for configuration constraints and returns a formatted list of all errors.
func (cfg *Config) Validate() error {
	var errs []string

	if cfg.AppName == "" {
		errs = append(errs, "APP_NAME: cannot be empty")
	}

	validEnvs := map[string]bool{"local": true, "development": true, "staging": true, "production": true, "test": true}
	if !validEnvs[strings.ToLower(cfg.AppEnv)] {
		errs = append(errs, fmt.Sprintf("APP_ENV: must be one of 'local', 'development', 'staging', 'production', 'test' (got %q)", cfg.AppEnv))
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
	if !validLogLevels[strings.ToLower(cfg.LogLevel)] {
		errs = append(errs, fmt.Sprintf("LOG_LEVEL: must be one of 'debug', 'info', 'warn', 'warning', 'error' (got %q)", cfg.LogLevel))
	}

	if strings.ToLower(cfg.LogFormat) != "text" && strings.ToLower(cfg.LogFormat) != "json" {
		errs = append(errs, fmt.Sprintf("LOG_FORMAT: must be 'text' or 'json' (got %q)", cfg.LogFormat))
	}

	if cfg.ServerPort < 1 || cfg.ServerPort > 65535 {
		errs = append(errs, fmt.Sprintf("SERVER_PORT: must be between 1 and 65535 (got %d)", cfg.ServerPort))
	}

	if cfg.EventBusProvider != "inmem" && cfg.EventBusProvider != "nats" {
		errs = append(errs, fmt.Sprintf("EVENT_BUS_PROVIDER: must be 'inmem' or 'nats' (got %q)", cfg.EventBusProvider))
	}

	if cfg.WorkflowStoreType != "mem" && cfg.WorkflowStoreType != "nats" && cfg.WorkflowStoreType != "redis" {
		errs = append(errs, fmt.Sprintf("WORKFLOW_STORE_TYPE: must be 'mem', 'nats', or 'redis' (got %q)", cfg.WorkflowStoreType))
	}

	if cfg.DBDriver != "sqlite" && cfg.DBDriver != "postgres" {
		errs = append(errs, fmt.Sprintf("DB_DRIVER: must be 'sqlite' or 'postgres' (got %q)", cfg.DBDriver))
	}

	if cfg.DBDriver == "sqlite" && cfg.DBDSN == "" {
		errs = append(errs, "DB_DSN: cannot be empty for database driver 'sqlite'")
	}
	if cfg.DBDriver == "postgres" && cfg.DBDSN == "" {
		errs = append(errs, "DB_DSN: cannot be empty for database driver 'postgres'")
	}

	// Conditional validations
	if cfg.EventBusProvider == "nats" || cfg.WorkflowStoreType == "nats" {
		if cfg.NATSURL == "" {
			errs = append(errs, "NATS_URL: must be specified when using NATS as event bus or workflow store")
		}
	}
	if cfg.WorkflowStoreType == "nats" {
		if cfg.NATSStateBucket == "" {
			errs = append(errs, "NATS_STATE_BUCKET: must be specified when workflow store type is 'nats'")
		}
		if cfg.NATSLocksBucket == "" {
			errs = append(errs, "NATS_LOCKS_BUCKET: must be specified when workflow store type is 'nats'")
		}
	}

	// Validate Auth & MCP Auth providers
	validProviders := map[string]bool{"jwt": true, "emailpass": true, "apikey": true, "none": true}
	for _, p := range cfg.AuthProviders {
		if !validProviders[p] {
			errs = append(errs, fmt.Sprintf("AUTH_PROVIDERS: unsupported auth provider %q (must be one of 'jwt', 'emailpass', 'apikey', 'none')", p))
		}
	}
	for _, p := range cfg.MCPAuthProviders {
		if !validProviders[p] {
			errs = append(errs, fmt.Sprintf("MCP_AUTH_PROVIDERS: unsupported auth provider %q (must be one of 'jwt', 'emailpass', 'apikey', 'none')", p))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// ResolveEnvOptions resolves any "env.XXX" references in module options using the OS environment.
func (cfg *Config) ResolveEnvOptions() {
	for i, m := range cfg.Modules {
		for k, val := range m.Options {
			if str, ok := val.(string); ok && len(str) > 4 && str[:4] == "env." {
				envName := str[4:]
				envVal := os.Getenv(envName)
				if envVal != "" {
					cfg.Modules[i].Options[k] = envVal
				}
			}
		}
	}
}

func resolveEnvOptions(v *viper.Viper, modules []ModuleConfig) {
	for i, m := range modules {
		for k, val := range m.Options {
			if str, ok := val.(string); ok && len(str) > 4 && str[:4] == "env." {
				envName := str[4:]
				envVal := os.Getenv(envName)
				if envVal == "" {
					envVal = v.GetString(envName)
				}
				modules[i].Options[k] = envVal
			}
		}
	}
}
