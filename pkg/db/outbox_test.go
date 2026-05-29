package db

import (
	"context"
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
)

func TestOutbox(t *testing.T) {
	dbFile := "outbox_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := Connect(cfg)
	sqlDB, _ := database.DB.DB()
	defer sqlDB.Close()

	database.AutoMigrateAll()

	t.Run("SaveToOutbox", func(t *testing.T) {
		payload := map[string]string{"foo": "bar"}
		err := database.SaveToOutbox(context.Background(), "test.event", payload)
		if err != nil {
			t.Fatalf("SaveToOutbox failed: %v", err)
		}

		var event OutboxEvent
		err = database.WithContext(context.Background()).First(&event).Error
		if err != nil {
			t.Fatalf("failed to retrieve event: %v", err)
		}

		if event.Type != "test.event" {
			t.Errorf("expected type test.event, got %s", event.Type)
		}
	})

	t.Run("SaveToOutbox Marshal Failure", func(t *testing.T) {
		// Use a type that cannot be marshaled to JSON
		payload := make(chan int)
		err := database.SaveToOutbox(context.Background(), "test.fail", payload)
		if err == nil {
			t.Error("expected marshal error, got nil")
		}
	})

	t.Run("SaveToOutbox DB Failure", func(t *testing.T) {
		dbFile := "outbox_fail.db"
		defer os.Remove(dbFile)
		cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		db, _ := Connect(cfg)
		sqlDB, _ := db.DB.DB()
		defer sqlDB.Close()
		
		db.AutoMigrateAll()
		
		sqlDB.Close() // Close to force failure

		err := db.SaveToOutbox(context.Background(), "test", map[string]string{"a": "b"})
		if err == nil {
			t.Error("expected DB error, got nil")
		}
	})
}
