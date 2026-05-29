package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config represents the application configuration.
type Config struct {
	AppName           string `mapstructure:"APP_NAME"`
	AppEnv            string `mapstructure:"APP_ENV"`
	LogLevel          string `mapstructure:"LOG_LEVEL"`
	LogFormat         string `mapstructure:"LOG_FORMAT"`
	ServerPort        int    `mapstructure:"SERVER_PORT"`
	EventBusProvider  string `mapstructure:"EVENT_BUS_PROVIDER"`
	DBDriver          string `mapstructure:"DB_DRIVER"`
	DBDSN             string `mapstructure:"DB_DSN"`
	JWTSecret         string `mapstructure:"JWT_SECRET"`
	JWTExpiration     string `mapstructure:"JWT_EXPIRATION"`
	StorageProvider   string `mapstructure:"STORAGE_PROVIDER"`
	StoragePath       string `mapstructure:"STORAGE_PATH"`
	StorageBucketURL  string `mapstructure:"STORAGE_BUCKET_URL"`
	NATSURL           string `mapstructure:"NATS_URL"`
}

// Load loads the configuration.
func Load() (*Config, error) {
	return LoadWithFile(".env")
}

// LoadWithFile loads the configuration from a specific file.
func LoadWithFile(filename string) (*Config, error) {
	v := viper.New()
	v.SetDefault("APP_NAME", "hyperrr")
	v.SetDefault("APP_ENV", "local")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "text")
	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("EVENT_BUS_PROVIDER", "inmem")
	v.SetDefault("DB_DRIVER", "sqlite")
	v.SetDefault("DB_DSN", "hyperrr.db")
	v.SetDefault("JWT_SECRET", "hyperrr-secret-key")
	v.SetDefault("JWT_EXPIRATION", "24h")
	v.SetDefault("STORAGE_PROVIDER", "cloud")
	v.SetDefault("STORAGE_BUCKET_URL", "mem://")
	v.SetDefault("NATS_URL", "nats://localhost:4222")

	if filename != "" {
		v.SetConfigFile(filename)
		v.SetConfigType("env")
		v.AutomaticEnv()

		if err := v.ReadInConfig(); err != nil {
			// If we specifically passed a filename, maybe we care if it's missing?
			// But for tests, we often want to ignore it.
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
		}
	} else {
		v.AutomaticEnv()
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}
