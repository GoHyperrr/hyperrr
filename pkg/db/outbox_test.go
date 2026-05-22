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
}
