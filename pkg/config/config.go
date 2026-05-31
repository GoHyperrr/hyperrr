package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
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
	JWTSecret         string         `mapstructure:"JWT_SECRET"`
	JWTExpiration     string         `mapstructure:"JWT_EXPIRATION"`
	StorageProvider   string         `mapstructure:"STORAGE_PROVIDER"`
	StoragePath       string         `mapstructure:"STORAGE_PATH"`
	StorageBucketURL  string         `mapstructure:"STORAGE_BUCKET_URL"`
	NATSURL           string         `mapstructure:"NATS_URL"`
	Modules           []ModuleConfig `mapstructure:"modules"`
}

// Load loads the configuration.
func Load() (*Config, error) {
	return LoadWithFile("")
}

// LoadWithFile loads the configuration from a specific file.
func LoadWithFile(filename string) (*Config, error) {
	// First, try to load environment variables from .env into the OS environment
	// so that standard os.Getenv calls can resolve env.XXX variables correctly.
	envViper := viper.New()
	envViper.SetConfigFile(".env")
	envViper.SetConfigType("env")
	if err := envViper.ReadInConfig(); err == nil {
		for _, key := range envViper.AllKeys() {
			val := envViper.GetString(key)
			upperKey := strings.ToUpper(key)
			if os.Getenv(upperKey) == "" {
				_ = os.Setenv(upperKey, val)
			}
		}
	}

	v := viper.New()
	v.SetDefault("APP_NAME", "hyperrr")
	v.SetDefault("APP_ENV", "local")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "text")
	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("EVENT_BUS_PROVIDER", "inmem")
	v.SetDefault("WORKFLOW_STORE_TYPE", "mem")
	v.SetDefault("NATS_STATE_BUCKET", "hyperrr_state")
	v.SetDefault("NATS_LOCKS_BUCKET", "hyperrr_locks")
	v.SetDefault("DB_DRIVER", "sqlite")
	v.SetDefault("DB_DSN", "hyperrr.db")
	v.SetDefault("JWT_SECRET", "hyperrr-secret-key")
	v.SetDefault("JWT_EXPIRATION", "24h")
	v.SetDefault("STORAGE_PROVIDER", "cloud")
	v.SetDefault("STORAGE_BUCKET_URL", "mem://")
	v.SetDefault("NATS_URL", "nats://localhost:4222")

	v.AutomaticEnv()

	if filename != "" {
		v.SetConfigFile(filename)
		// Determine config type from extension or default to "env"
		ext := filepath.Ext(filename)
		if ext == "" || (!strings.EqualFold(ext, ".yaml") && !strings.EqualFold(ext, ".yml") && !strings.EqualFold(ext, ".json") && !strings.EqualFold(ext, ".toml")) {
			v.SetConfigType("env")
		}
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
		}
	} else {
		// Look for hyperrr.yml, hyperrr.yaml, or hyperrr.json in working directory or configs/
		v.SetConfigName("hyperrr")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	resolveEnvOptions(v, cfg.Modules)

	return &cfg, nil
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
