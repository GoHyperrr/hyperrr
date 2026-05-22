package notification

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestNotificationModule(t *testing.T) {
	dbFile := "notif_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus)
	
	// Create mock provider
	mockProv := &MockProvider{}

	mod := NewModule(mockProv)
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(mod.Models()...)
	for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
	database.AutoMigrateAll()

	t.Run("Send Notification Success", func(t *testing.T) {
		recipient := fmt.Sprintf("test_%d@example.com", time.Now().UnixNano())
		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "send", Uses: "notification.send"}},
		}

		input := map[string]any{
			"recipient": recipient,
			"channel":   "EMAIL",
			"subject":   "Test",
			"body":      "Hello",
		}

		res, err := runner.Execute(context.Background(), "n1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		n := res["send"].(*Notification)
		if n.Status != StatusSent {
			t.Errorf("expected SENT status, got %s", n.Status)
		}

		// Verify Repo
		list, _ := mod.Repo().List(context.Background(), recipient)
		if len(list) != 1 {
			t.Error("expected 1 notification in repo")
		}
	})

	t.Run("Send Notification Failure", func(t *testing.T) {
		recipient := fmt.Sprintf("fail_%d@example.com", time.Now().UnixNano())
		mockProv.ShouldFail = true
		
		wf := &workflow.Workflow{
			Steps: []workflow.Step{{ID: "send", Uses: "notification.send"}},
		}

		input := map[string]any{
			"recipient": recipient,
			"channel":   "EMAIL",
			"subject":   "Test",
			"body":      "Hello",
		}

		_, err := runner.Execute(context.Background(), "n2", wf, input)
		if err == nil {
			t.Fatal("expected workflow failure")
		}

		// DB should still have it as FAILED
		list, _ := mod.Repo().List(context.Background(), recipient)
		if len(list) != 1 || list[0].Status != StatusFailed {
			t.Error("expected 1 FAILED notification in repo")
		}
		
		mockProv.ShouldFail = false // Reset
	})
	
	t.Run("Event Subscriptions", func(t *testing.T) {
		recipient := fmt.Sprintf("event_%d@example.com", time.Now().UnixNano())
		// Test identity.user_created
		bus.Publish(context.Background(), eventbus.Event{
			Type: "identity.user_created",
			Payload: map[string]any{
				"email": recipient,
				"name":  "Event User",
			},
		})
		
		// Wait for async workflow
		time.Sleep(100 * time.Millisecond)
		
		list, _ := mod.Repo().List(context.Background(), recipient)
		if len(list) != 1 {
			t.Error("expected welcome email to be sent")
		}
		
		// Test workflow.completed (fulfillment)
		bus.Publish(context.Background(), eventbus.Event{
			Type: "workflow.completed",
			Payload: map[string]any{
				"name": "fulfillment.v1",
			},
		})
		// Just ensure it doesn't panic, as it only logs for now
		time.Sleep(10 * time.Millisecond)
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		// Invalid input
		_, err := mod.SendNotification(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.SendNotification(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
	})
}
