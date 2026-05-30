package notification

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestNotificationModule(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: ":memory:"}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus, nil, nil)
	
	// Create mock provider
	mockProv := &MockProvider{}

	mod := NewModule(mockProv)
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
	db.Register(mod.Models()...)
	for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
	database.AutoMigrateAll()

	t.Run("Send Notification Success", func(t *testing.T) {
		recipient := fmt.Sprintf("test_%s@example.com", uuid.New().String()[:8])
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
		recipient := fmt.Sprintf("fail_%s@example.com", uuid.New().String()[:8])
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
		recipient := fmt.Sprintf("event_%s@example.com", uuid.New().String()[:8])
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
		// 1. Invalid input
		_, err := mod.SendNotification(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		
		// 2. Missing workflow input
		_, err = mod.SendNotification(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
		
		// 3. Missing recipient
		_, err = mod.SendNotification(context.Background(), map[string]any{"input": map[string]any{}})
		if err == nil { t.Error("expected error for missing recipient") }
	})

	t.Run("Repository Edge Cases", func(t *testing.T) {
		repo := mod.Repo()
		ctx := context.Background()

		// 1. GetByID Not Found
		_, err := repo.GetByID(ctx, "ghost")
		if err == nil { t.Error("expected error for non-existent notif") }

		// 2. List with recipient filter
		n1 := &Notification{ID: "notif_1", Recipient: "user1", Status: StatusSent}
		n2 := &Notification{ID: "notif_2", Recipient: "user2", Status: StatusSent}
		repo.Save(ctx, n1)
		repo.Save(ctx, n2)

		list1, _ := repo.List(ctx, "user1")
		if len(list1) != 1 || list1[0].ID != "notif_1" { t.Error("List filter failed for user1") }

		listAll, _ := repo.List(ctx, "")
		if len(listAll) < 2 { t.Error("List with empty filter failed") }
	})
}
