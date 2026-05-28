package workflow

import "testing"

func TestWorkflowRegistry(t *testing.T) {
	r := NewRegistry()
	wf := &Workflow{Name: "test", Version: "v1"}
	r.Register(wf)

	t.Run("Get Success", func(t *testing.T) {
		got, err := r.Get("test")
		if err != nil || got.Name != "test" {
			t.Error("Get failed")
		}
	})

	t.Run("Get Fail", func(t *testing.T) {
		_, err := r.Get("ghost")
		if err == nil {
			t.Error("expected error for non-existent workflow")
		}
	})
}
