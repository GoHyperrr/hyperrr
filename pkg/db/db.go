package db

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB represents the database connection wrapper.
type DB struct {
	*gorm.DB
}

// gormLoggerBridge redirects GORM logs to the central structured logger.
type gormLoggerBridge struct {
	LogLevel gormlogger.LogLevel
}

func (l *gormLoggerBridge) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return &gormLoggerBridge{LogLevel: level}
}

func (l *gormLoggerBridge) Info(ctx context.Context, msg string, args ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		logger.Info(fmt.Sprintf(msg, args...))
	}
}

func (l *gormLoggerBridge) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		logger.Warn(fmt.Sprintf(msg, args...))
	}
}

func (l *gormLoggerBridge) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		logger.Error(fmt.Sprintf(msg, args...))
	}
}

func (l *gormLoggerBridge) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= 0 {
		return
	}
	elapsed := time.Since(begin)
	sql, rows := fc()
	if err != nil && l.LogLevel >= gormlogger.Error {
		logger.Error("DB Trace Error", "err", err, "elapsed", elapsed, "rows", rows, "sql", sql)
	} else if elapsed > time.Second && l.LogLevel >= gormlogger.Warn {
		logger.Warn("DB Slow Query", "elapsed", elapsed, "rows", rows, "sql", sql)
	} else if l.LogLevel >= gormlogger.Info {
		logger.Debug("DB Trace", "elapsed", elapsed, "rows", rows, "sql", sql)
	}
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

	gBridge := &gormLoggerBridge{LogLevel: gormlogger.Info}

	db, err := gorm.Open(dialect, &gorm.Config{
		Logger: gBridge,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{db}, nil
}

// Transaction executes the given function within a database transaction.
func (db *DB) Transaction(fn func(tx *DB) error) error {
	return db.DB.Transaction(func(gtx *gorm.DB) error {
		return fn(&DB{gtx})
	})
}
