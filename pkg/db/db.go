package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB represents the database connection wrapper.
type DB struct {
	*gorm.DB
}

// Connect initializes a database connection based on the provided configuration.
func Connect(cfg *config.Config) (*DB, error) {
	var dialect gorm.Dialector

	switch cfg.DBDriver {
	case "sqlite":
		dialect = sqlite.Open(cfg.DBDSN)
	case "postgres":
		dialect = postgres.Open(cfg.DBDSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.DBDriver)
	}

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(dialect, &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{db}, nil
}
