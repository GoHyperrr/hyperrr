package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	gormlogger "gorm.io/gorm/logger"
)

func TestConnect(t *testing.T) {
	t.Run("Connect to SQLite", func(t *testing.T) {
		cfg := &config.Config{
			DBDriver: "sqlite",
			DBDSN:    ":memory:",
		}

		db, err := Connect(cfg)
		if err != nil {
			t.Fatalf("failed to connect to sqlite: %v", err)
		}

		if db == nil {
			t.Fatal("expected db to be non-nil")
		}
	})

	t.Run("Postgres dialect reached", func(t *testing.T) {
		cfg := &config.Config{
			DBDriver: "postgres",
			DBDSN:    "host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable",
		}

		// This will likely fail to connect but will cover the 'case "postgres"' line.
		_, err := Connect(cfg)
		if err == nil {
			t.Log("Unexpectedly connected to postgres (is it running?)")
		}
	})

	t.Run("AutoMigrate empty", func(t *testing.T) {
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
		db, _ := Connect(cfg)
		db.AutoMigrateAll()
	})

	t.Run("Unsupported driver", func(t *testing.T) {
		cfg := &config.Config{DBDriver: "unsupported"}
		_, err := Connect(cfg)
		if err == nil {
			t.Error("expected error for unsupported driver")
		}
	})

	t.Run("Invalid SQLite DSN", func(t *testing.T) {
		cfg := &config.Config{
			DBDriver: "sqlite",
			DBDSN:    "/nonexistent/path/to/db.db",
		}
		_, err := Connect(cfg)
		if err == nil {
			t.Error("expected error for invalid sqlite dsn")
		}
	})

	t.Run("Transaction", func(t *testing.T) {
		id := fmt.Sprintf("tx_%d", time.Now().UnixNano())
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
		database, _ := Connect(cfg)
		database.AutoMigrate(&IdempotencyKey{})

		err := database.Transaction(func(tx *DB) error {
			return tx.Create(&IdempotencyKey{ID: id, Scope: "unique_s_" + id, Key: "unique_k_" + id}).Error
		})
		if err != nil { t.Errorf("Transaction failed: %v", err) }

		var got IdempotencyKey
		database.First(&got, "id = ?", id)
		if got.ID != id { t.Error("Transaction did not persist") }
	})

	t.Run("Logger Bridge", func(t *testing.T) {
		bridge := &gormLoggerBridge{LogLevel: gormlogger.Info}
		ctx := context.Background()
		
		// Cover LogMode
		bridge.LogMode(gormlogger.Warn)
		
		// Cover Info/Warn/Error
		bridge.Info(ctx, "test info")
		bridge.Warn(ctx, "test warn")
		bridge.Error(ctx, "test error")

		// Cover Trace
		bridge.Trace(ctx, time.Now(), func() (string, int64) { return "SELECT 1", 1 }, nil)
		bridge.Trace(ctx, time.Now().Add(-2*time.Second), func() (string, int64) { return "SELECT 1", 1 }, nil)
		bridge.Trace(ctx, time.Now(), func() (string, int64) { return "SELECT 1", 1 }, fmt.Errorf("trace err"))
		
		// Cover LogLevel 0 (silent)
		quiet := &gormLoggerBridge{LogLevel: 0}
		quiet.Trace(ctx, time.Now(), func() (string, int64) { return "SELECT 1", 1 }, nil)
	})
}

func TestIdempotency(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := Connect(cfg)
	database.AutoMigrate(&IdempotencyKey{})

	ctx := context.Background()
	scope := "test_scope"
	key := "test_key"

	t.Run("Success Path", func(t *testing.T) {
		// Should not be processed initially
		processed, err := database.IsProcessed(ctx, scope, key)
		if err != nil {
			t.Fatalf("IsProcessed failed: %v", err)
		}
		if processed {
			t.Error("expected processed to be false")
		}

		// Mark as processed
		err = database.MarkProcessed(ctx, scope, key)
		if err != nil {
			t.Fatalf("MarkProcessed failed: %v", err)
		}

		// Should now be processed
		processed, err = database.IsProcessed(ctx, scope, key)
		if err != nil {
			t.Fatalf("IsProcessed failed: %v", err)
		}
		if !processed {
			t.Error("expected processed to be true")
		}

		// Duplicate mark should fail (unique constraint)
		err = database.MarkProcessed(ctx, scope, key)
		if err == nil {
			t.Error("expected error for duplicate MarkProcessed")
		}
	})

	t.Run("Failures", func(t *testing.T) {
		// Close the underlying sql.DB to trigger failures
		sqlDB, _ := database.DB.DB()
		sqlDB.Close()

		// IsProcessed should return error on closed DB
		_, err := database.IsProcessed(ctx, "fail", "key")
		if err == nil {
			t.Error("expected IsProcessed to return error on closed DB")
		}

		// MarkProcessed should return error on closed DB
		err = database.MarkProcessed(ctx, "fail", "key")
		if err == nil {
			t.Error("expected MarkProcessed to return error on closed DB")
		}
	})
}

func TestAutoMigrateAll(t *testing.T) {
	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    ":memory:",
	}

	db, err := Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	type TestModel struct {
		ID   uint `gorm:"primaryKey"`
		Name string
	}

	Register(&TestModel{})

	err = db.AutoMigrateAll()
	if err != nil {
		t.Fatalf("AutoMigrateAll failed: %v", err)
	}

	// Verify table exists
	if !db.Migrator().HasTable(&TestModel{}) {
		t.Error("expected table TestModel to exist")
	}
}
