package workflow

import "testing"

func TestWorkflowRegistry(t *testing.T) {
	r := NewRegistry()
	wf := &Workflow{
		Name:        "test",
		Version:     "v1",
		Description: "A test workflow",
		ExposeToAI:  true,
		InputSchema: map[string]any{"type": "object"},
	}
	r.Register(wf)

	t.Run("Get Success and Metadata", func(t *testing.T) {
		got, err := r.Get("test")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.Name != "test" {
			t.Errorf("expected name test, got %s", got.Name)
		}
		if got.Description != "A test workflow" {
			t.Errorf("expected description, got %s", got.Description)
		}
		if !got.ExposeToAI {
			t.Error("expected expose_to_ai to be true")
		}
		if got.InputSchema["type"] != "object" {
			t.Error("input_schema not preserved")
		}
	})

	t.Run("Get Fail", func(t *testing.T) {
		_, err := r.Get("ghost")
		if err == nil {
			t.Error("expected error for non-existent workflow")
		}
	})

	t.Run("Register Empty Name Fail", func(t *testing.T) {
		badWf := &Workflow{Name: ""}
		err := r.Register(badWf)
		if err == nil {
			t.Error("expected error when registering workflow with empty name")
		}
	})

	t.Run("List workflows", func(t *testing.T) {
		list := r.List()
		if len(list) != 1 || list[0].Name != "test" {
			t.Errorf("expected list with 1 workflow named 'test', got %v", list)
		}
	})
}
