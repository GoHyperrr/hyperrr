package workflow

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestInMemStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemStore()

	execID := "test-exec-1"

	// Test Initial State
	state, err := store.GetState(ctx, execID)
	if err == nil {
		t.Error("expected error for non-existent state, got nil")
	}

	// Test SaveInput / GetInput
	inputData := []byte(`{"customer_id": "c1"}`)
	err = store.SaveInput(ctx, execID, inputData)
	if err != nil {
		t.Fatalf("SaveInput failed: %v", err)
	}

	gotInput, err := store.GetInput(ctx, execID)
	if err != nil {
		t.Fatalf("GetInput failed: %v", err)
	}
	if !reflect.DeepEqual(gotInput, inputData) {
		t.Errorf("expected input %s, got %s", inputData, gotInput)
	}

	// Test SaveState / GetState
	err = store.SaveState(ctx, execID, "step1", "RUNNING")
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	err = store.SaveState(ctx, execID, "step1", "COMPLETED")
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	err = store.SaveState(ctx, execID, "step2", "FAILED")
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	state, err = store.GetState(ctx, execID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if state["step1"] != "COMPLETED" {
		t.Errorf("expected step1 COMPLETED, got %s", state["step1"])
	}
	if state["step2"] != "FAILED" {
		t.Errorf("expected step2 FAILED, got %s", state["step2"])
	}

	// Test TTL
	err = store.SetTTL(ctx, execID, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("SetTTL failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	_, err = store.GetState(ctx, execID)
	if err == nil {
		t.Error("expected error for expired state, got nil")
	}

	_, err = store.GetInput(ctx, execID)
	if err == nil {
		t.Error("expected error for expired input, got nil")
	}
}
