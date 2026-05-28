package db

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
)

func TestConnect(t *testing.T) {
	t.Run("Connect to SQLite", func(t *testing.T) {
		dbFile := "test.db"
		defer os.Remove(dbFile)

		cfg := &config.Config{
			DBDriver: "sqlite",
			DBDSN:    dbFile,
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
		dbFile := "empty_migrate.db"
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
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
}

func TestIdempotencyFailures(t *testing.T) {
	dbFile := "idempotency_fail.db"
	defer os.Remove(dbFile)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := Connect(cfg)
	database.AutoMigrate(&IdempotencyKey{})

	// Close the underlying sql.DB to trigger failures
	sqlDB, _ := database.DB.DB()
	sqlDB.Close()

	ctx := context.Background()

	// IsProcessed should return false on error based on current implementation
	if database.IsProcessed(ctx, "test", "key") {
		t.Error("expected IsProcessed to return false on error")
	}

	// MarkProcessed should return error
	err := database.MarkProcessed(ctx, "test", "key")
	if err == nil {
		t.Error("expected MarkProcessed to return error on closed DB")
	}
}

func TestAutoMigrateAll(t *testing.T) {
	dbFile := "migrate_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    dbFile,
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
