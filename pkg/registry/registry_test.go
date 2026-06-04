package registry

import (
	"context"
	"testing"

	"github.com/GoHyperrr/mdk"
)

type mockModule struct {
	id      string
	initErr error
	models  []any
	routes  []mdk.Route
}

func (m *mockModule) ID() string { return m.id }
func (m *mockModule) Init(ctx context.Context, rt mdk.Runtime) error { return m.initErr }
func (m *mockModule) Shutdown(ctx context.Context) error { return nil }
func (m *mockModule) Models() []any { return m.models }
func (m *mockModule) Routes() []mdk.Route { return m.routes }

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
			routes: []mdk.Route{
				{Method: "GET", Pattern: "/test", Handler: nil},
			},
		}

		if m.ID() != "test" {
			t.Error("invalid ID")
		}
		if len(m.Models()) != 1 {
			t.Error("invalid Models")
		}
		if len(m.Routes()) != 1 {
			t.Error("invalid Routes")
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
