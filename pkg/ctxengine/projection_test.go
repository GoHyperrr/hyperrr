package ctxengine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/mdk"
)

func TestProjector(t *testing.T) {
	mdk.RegisterLineageEvent("order.created")
	mdk.RegisterLineageEvent("order.paid")

	bus := eventbus.NewInMemBus()
	projector := NewProjector(bus)
	
	ctx := context.Background()

	err := projector.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start projector: %v", err)
	}

	workflowID := "wf123"

	t.Run("Full Success Path", func(t *testing.T) {
		// Simulate workflow.started
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowStarted,
			OccurredAt: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"name":    "test-workflow",
				"version": "v1",
			},
		})

		// Simulate step 1
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepStarted,
			OccurredAt: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"step_id": "step1",
			},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepCompleted,
			OccurredAt: time.Now(),
			Payload: map[string]any{
				"id":      workflowID,
				"step_id": "step1",
			},
		})

		// Simulate workflow.completed
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowCompleted,
			OccurredAt: time.Now(),
			Payload: map[string]any{
				"id": workflowID,
			},
		})

		time.Sleep(50 * time.Millisecond) // Wait for async projection
		lineage, err := projector.GetLineage(workflowID)
		if err != nil {
			t.Fatalf("failed to get lineage: %v", err)
		}

		if lineage.Name != "test-workflow" {
			t.Errorf("expected test-workflow, got %s", lineage.Name)
		}
		if lineage.State != workflow.StateCompleted {
			t.Errorf("expected %s state, got %s", workflow.StateCompleted, lineage.State)
		}
		if len(lineage.Steps) != 1 {
			t.Errorf("expected 1 step, got %d", len(lineage.Steps))
		}
		if lineage.Steps[0].State != workflow.StateCompleted {
			t.Errorf("expected step %s, got %s", workflow.StateCompleted, lineage.Steps[0].State)
		}
	})

	t.Run("Failure and Retry Path", func(t *testing.T) {
		wfID := "wf_fail"
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowStarted,
			OccurredAt: time.Now(),
			Payload:   map[string]any{"id": wfID, "name": "fail", "version": "v1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepStarted,
			OccurredAt: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepRetrying,
			OccurredAt: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventStepFailed,
			OccurredAt: time.Now(),
			Payload:   map[string]any{"id": wfID, "step_id": "s1", "error": "boom"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowFailed,
			OccurredAt: time.Now(),
			Payload:   map[string]any{"id": wfID, "error": "workflow failed"},
		})

		time.Sleep(50 * time.Millisecond)
		l, _ := projector.GetLineage(wfID)
		if l.State != workflow.StateFailed || l.Error != "workflow failed" {
			t.Errorf("unexpected state: %s, error: %s", l.State, l.Error)
		}
		if l.Steps[0].Attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", l.Steps[0].Attempts)
		}
	})

	t.Run("Waiting Human", func(t *testing.T) {
		wfID := "wf_human"
		bus.Publish(ctx, eventbus.Event{
			Type:    workflow.EventWaitingHuman,
			Payload: map[string]any{"id": wfID},
		})
		time.Sleep(50 * time.Millisecond)
		l, _ := projector.GetLineage(wfID)
		if l.State != workflow.StateWaitingHuman {
			t.Errorf("expected %s, got %s", workflow.StateWaitingHuman, l.State)
		}
	})

	t.Run("Lineage Not Found", func(t *testing.T) {
		_, err := projector.GetLineage("ghost")
		if err == nil {
			t.Error("expected error for non-existent lineage")
		}

		_, err = projector.GetRelatedLineages(ctx, "ghost")
		if err == nil {
			t.Error("expected error for non-existent lineage in GetRelatedLineages")
		}
	})

	t.Run("ListLineages", func(t *testing.T) {
		res := projector.ListLineages()
		if len(res) < 3 {
			t.Errorf("expected at least 3 lineages, got %d", len(res))
		}
	})

	t.Run("Comprehensive handleEvent", func(t *testing.T) {
		wfID := "wf_comp"
		
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowStarted,
			Payload: map[string]any{"id": wfID, "name": "comp-wf", "version": "v2"},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepStarted,
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepRetrying,
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepCompleted,
			Payload: map[string]any{"id": wfID, "step_id": "step1"},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: "order.created",
			Payload: map[string]any{"id": wfID, "order_id": "ord123"},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepStarted,
			Payload: map[string]any{"id": wfID, "step_id": "step2"},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventStepFailed,
			Payload: map[string]any{"id": wfID, "step_id": "step2", "error": "step fail"},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWaitingHuman,
			Payload: map[string]any{"id": wfID},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: "order.paid",
			Payload: map[string]any{"id": wfID, "order_id": "ord123"},
			OccurredAt: time.Now(),
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowFailed,
			Payload: map[string]any{"id": wfID, "error": "final fail"},
			OccurredAt: time.Now(),
		})

		time.Sleep(100 * time.Millisecond)
		l, _ := projector.GetLineage(wfID)
		if l.State != workflow.StateFailed {
			t.Errorf("expected FAILED, got %s", l.State)
		}
		
		foundCreated := false
		foundPaid := false
		for _, e := range l.Events {
			if e.Type == "order.created" { foundCreated = true }
			if e.Type == "order.paid" { foundPaid = true }
		}
		if !foundCreated || !foundPaid {
			t.Error("missing order events in lineage")
		}
	})

	t.Run("QueryLineages", func(t *testing.T) {
		res := projector.QueryLineages(func(ld registry.LineageData) bool {
			return ld.GetName() == "comp-wf"
		})
		if len(res) != 1 {
			t.Errorf("expected 1 lineage for comp-wf, got %d", len(res))
		}
	})

	t.Run("Complex RelatedLineages", func(t *testing.T) {
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowStarted,
			Payload: map[string]any{"id": "rel_a", "name": "a", "customer_id": "cust_shared"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowStarted,
			Payload: map[string]any{"id": "rel_b", "name": "b", "customer_id": "cust_shared"},
		})
		bus.Publish(ctx, eventbus.Event{
			Type: workflow.EventWorkflowStarted,
			Payload: map[string]any{"id": "rel_c", "name": "c", "customer_id": "cust_shared"},
		})

		time.Sleep(100 * time.Millisecond)
		rel, err := projector.GetRelatedLineages(ctx, "rel_a")
		if err != nil { t.Fatalf("failed: %v", err) }
		if len(rel) != 2 {
			t.Errorf("expected 2 related lineages, got %d", len(rel))
		}
	})

	t.Run("findStep multiple steps", func(t *testing.T) {
		l := &Lineage{
			Steps: []*StepLineage{
				{StepID: "s1", State: workflow.StateCompleted},
				{StepID: "s2", State: workflow.StateRunning},
				{StepID: "s1", State: workflow.StateRetrying}, // Newer s1
			},
		}
		p := &Projector{}
		step := p.findStep(l, "s1")
		if step == nil || step.State != workflow.StateRetrying {
			t.Errorf("expected latest s1 step with %s state, got %v", workflow.StateRetrying, step)
		}
	})

	t.Run("Start with nil bus", func(t *testing.T) {
		p := &Projector{}
		err := p.Start(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("Coverage Edge Cases", func(t *testing.T) {
		p := NewProjector(nil)

		// 1. handleEvent with payload that is nil
		err := p.handleEvent(ctx, eventbus.Event{
			Type:    "test",
			Payload: nil,
		})
		if err != nil {
			t.Errorf("expected no error for invalid payload type, got %v", err)
		}

		// 2. handleEvent with empty id
		err = p.handleEvent(ctx, eventbus.Event{
			Type:    "test",
			Payload: map[string]any{"not-id": "val"},
		})
		if err != nil {
			t.Errorf("expected no error for missing id, got %v", err)
		}

		// 3. GetRelatedLineages with no events/correlation
		projector.mu.Lock()
		projector.lineages["no-events"] = &Lineage{ID: "no-events"}
		projector.mu.Unlock()

		related, err := projector.GetRelatedLineages(ctx, "no-events")
		if err != nil {
			t.Errorf("GetRelatedLineages failed: %v", err)
		}
		if len(related) != 0 {
			t.Errorf("expected 0 related lineages, got %d", len(related))
		}
	})

	t.Run("Exhaustive Event Types", func(t *testing.T) {
		p := NewProjector(nil)
		wfID := "wf_exhaustive"

		eventTypes := []string{
			workflow.EventWorkflowStarted,
			workflow.EventStepStarted,
			workflow.EventStepCompleted,
			workflow.EventStepFailed,
			workflow.EventStepRetrying,
			workflow.EventWaitingHuman,
			workflow.EventWorkflowCompleted,
			workflow.EventWorkflowFailed,
			"order.created",
			"order.paid",
		}

		for _, et := range eventTypes {
			payload := map[string]any{"id": wfID, "step_id": "s1", "name": "test", "version": "v1", "error": "some error", "m1": "v1"}
			err := p.handleEvent(ctx, eventbus.Event{
				Type:       et,
				Payload:    payload,
				OccurredAt: time.Now(),
			})
			if err != nil {
				t.Errorf("failed to handle event %s: %v", et, err)
			}
		}

		l, _ := p.GetLineage(wfID)
		if len(l.Events) != len(eventTypes) {
			t.Errorf("expected %d events, got %d", len(eventTypes), len(l.Events))
		}
	})

	t.Run("SQL Persistence & Fallback", func(t *testing.T) {
		cfg := &config.Config{
			DBDriver: "sqlite",
			DBDSN:    ":memory:",
		}
		database, err := db.Connect(cfg)
		if err != nil {
			t.Fatalf("failed to connect database: %v", err)
		}
		sqlDB, _ := database.DB.DB()
		defer sqlDB.Close()

		// Run migration
		err = database.AutoMigrate(&LineageModel{})
		if err != nil {
			t.Fatalf("failed auto-migrate: %v", err)
		}

		bus := eventbus.NewInMemBus()
		p := NewProjector(bus)
		p.SetDB(database.DB)

		wfID := "sql-wf-1"

		// Simulate starting workflow
		err = p.handleEvent(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowStarted,
			OccurredAt: time.Now(),
			Payload:   map[string]any{"id": wfID, "name": "sql-flow", "version": "v1"},
		})
		if err != nil {
			t.Fatalf("handleEvent failed: %v", err)
		}

		// Ensure it's not persisted yet (not a terminal state)
		var count int64
		database.Model(&LineageModel{}).Where("id = ?", wfID).Count(&count)
		if count != 0 {
			t.Error("expected 0 lineages in DB before completion")
		}

		// Simulate terminal state: completed
		err = p.handleEvent(ctx, eventbus.Event{
			Type:      workflow.EventWorkflowCompleted,
			OccurredAt: time.Now(),
			Payload:   map[string]any{"id": wfID},
		})
		if err != nil {
			t.Fatalf("handleEvent failed: %v", err)
		}

		// Verify it was persisted to database
		database.Model(&LineageModel{}).Where("id = ?", wfID).Count(&count)
		if count != 1 {
			t.Error("expected lineage to be saved to DB")
		}

		// Clear memory map to simulate a restart/cache miss
		p.mu.Lock()
		delete(p.lineages, wfID)
		p.mu.Unlock()

		// Retrieve lineage - should fallback to database and cache it
		l, err := p.GetLineage(wfID)
		if err != nil {
			t.Fatalf("GetLineage failed on fallback: %v", err)
		}
		if l.Name != "sql-flow" || l.State != workflow.StateCompleted {
			t.Errorf("lineage data mismatch: %v", l)
		}

		// Check list matches
		list := p.ListLineages()
		if len(list) != 1 {
			t.Errorf("expected 1 lineage in ListLineages, got %d", len(list))
		}

		// Check QueryLineages matches
		qList := p.QueryLineages(func(d registry.LineageData) bool {
			return d.GetState() == workflow.StateCompleted
		})
		if len(qList) != 1 {
			t.Errorf("expected 1 in QueryLineages, got %d", len(qList))
		}
	})

	t.Run("SQL Error Paths and Marshal failure", func(t *testing.T) {
		cfg := &config.Config{
			DBDriver: "sqlite",
			DBDSN:    ":memory:",
		}
		database, err := db.Connect(cfg)
		if err != nil {
			t.Fatalf("failed to connect database: %v", err)
		}
		sqlDB, _ := database.DB.DB()

		p := NewProjector(nil)
		p.SetDB(database.DB)

		lBad := &Lineage{
			ID: "bad-marshal",
			Events: []eventbus.Event{
				{Type: "test", Payload: map[string]any{"bad": make(chan int)}},
			},
		}
		p.saveToDB(context.Background(), lBad)

		sqlDB.Close()

		_, err = p.GetLineage("ghost")
		if err == nil {
			t.Error("expected error from GetLineage when DB connection is closed")
		}

		list := p.ListLineages()
		if len(list) != 0 {
			t.Errorf("expected 0 lineages on query error, got %d", len(list))
		}
	})

	t.Run("Start subscription error", func(t *testing.T) {
		p := NewProjector(&errorEventBus{})
		err := p.Start(context.Background())
		if err == nil {
			t.Error("expected subscription error from Start")
		}
	})
}

type errorEventBus struct {
	eventbus.EventBus
}

func (e *errorEventBus) Subscribe(namespace, eventType string, handler eventbus.EventHandler) (func(), error) {
	return nil, errors.New("mock subscription error")
}
