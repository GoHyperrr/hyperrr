package registry

import (
	"context"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
)

type mockModule struct {
	id       string
	initErr  error
	models   []any
	handlers map[string]workflow.TaskHandler
}

func (m *mockModule) ID() string { return m.id }
func (m *mockModule) Init(ctx context.Context, deps *Dependencies) error { return m.initErr }
func (m *mockModule) Shutdown(ctx context.Context) error { return nil }
func (m *mockModule) Models() []any { return m.models }
func (m *mockModule) Handlers() map[string]workflow.TaskHandler { return m.handlers }

func TestRegistry(t *testing.T) {
	t.Run("Register and List", func(t *testing.T) {
		reg := NewRegistry()
		m1 := &mockModule{id: "mod1"}
		m2 := &mockModule{id: "mod2"}

		reg.Register(m1)
		reg.Register(m2)

		list := reg.List()
		if len(list) != 2 {
			t.Errorf("expected 2 modules, got %d", len(list))
		}

		got, ok := reg.Get("mod1")
		if !ok || got != m1 {
			t.Error("failed to get mod1")
		}
	})

	t.Run("Duplicate Registration", func(t *testing.T) {
		reg := NewRegistry()
		m1 := &mockModule{id: "dup"}
		m2 := &mockModule{id: "dup"}

		reg.Register(m1)
		reg.Register(m2) // Should trigger warning but succeed in overwriting

		got, _ := reg.Get("dup")
		if got != m2 {
			t.Error("expected second registration to overwrite")
		}
	})

	t.Run("Global Registry Wrappers", func(t *testing.T) {
		m := &mockModule{id: "global_mod"}
		Register(m)
		
		list := List()
		found := false
		for _, mod := range list {
			if mod.ID() == "global_mod" { found = true; break }
		}
		if !found { t.Error("global List failed") }

		got, ok := Get("global_mod")
		if !ok || got != m { t.Error("global Get failed") }
	})

	t.Run("Module Interface", func(t *testing.T) {
		m := &mockModule{
			id:     "test",
			models: []any{"model1"},
			handlers: map[string]workflow.TaskHandler{
				"task1": func(ctx context.Context, input any) (any, error) { return nil, nil },
			},
		}

		if m.ID() != "test" {
			t.Error("invalid ID")
		}
		if len(m.Models()) != 1 {
			t.Error("invalid Models")
		}
		if len(m.Handlers()) != 1 {
			t.Error("invalid Handlers")
		}
		if m.Init(context.Background(), nil) != nil {
			t.Error("invalid Init")
		}
	})
}

func TestDependencies(t *testing.T) {
	deps := &Dependencies{}
	if deps == nil {
		t.Fatal("expected deps to be non-nil")
	}
}
