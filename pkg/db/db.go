package db

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/logger"
	"github.com/GoHyperrr/hyperrr/pkg/utils"
	"github.com/GoHyperrr/mdk"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type DialectProvider = mdk.DialectProvider

func RegisterDialect(name string, provider DialectProvider) {
	mdk.RegisterDialect(name, provider)
}

func GetDialect(name string) (DialectProvider, bool) {
	return mdk.GetDialect(name)
}

// JSONMap is a custom type for map[string]string that implements GORM scanner/valuer.
type JSONMap map[string]string

func (m JSONMap) Value() (driver.Value, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	return json.Unmarshal(bytes, m)
}

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
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && l.LogLevel >= gormlogger.Error {
		logger.Error("DB Trace Error", "err", err, "elapsed", elapsed, "rows", rows, "sql", sql)
	} else if errors.Is(err, gorm.ErrRecordNotFound) && l.LogLevel >= gormlogger.Info {
		logger.Debug("DB Record Not Found", "elapsed", elapsed, "rows", rows, "sql", sql)
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
		dsn := cfg.DBDSN
		if dsn != ":memory:" && !filepath.IsAbs(dsn) && !strings.Contains(dsn, "/") && !strings.Contains(dsn, "\\") {
			root := utils.FindProjectRoot()
			workspace := filepath.Join(root, ".hyperrr")
			if err := os.MkdirAll(workspace, 0755); err != nil {
				return nil, fmt.Errorf("failed to create .hyperrr workspace: %w", err)
			}
			dsn = filepath.Join(workspace, dsn)
		}
		dialect = sqlite.Open(dsn)
	default:
		provider, ok := GetDialect(cfg.DBDriver)
		if !ok {
			return nil, fmt.Errorf("unsupported database driver: %s (did you register the database module?)", cfg.DBDriver)
		}
		dialect = provider(cfg.DBDSN)
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
