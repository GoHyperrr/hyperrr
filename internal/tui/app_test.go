package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "charm.land/bubbletea/v2"
)

type mockPage struct {
	title       string
	msgReceived any
}

func (p *mockPage) Title() string {
	return p.title
}

func (p *mockPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	return nil
}

func (p *mockPage) Update(msg any) (registry.TUIPage, any) {
	p.msgReceived = msg
	return p, nil
}

func (p *mockPage) View() string {
	return "View for " + p.title
}

type mockTUIModule struct {
	pages []registry.TUIPage
}

func (m *mockTUIModule) ID() string {
	return "commerce.mock_tui"
}

func (m *mockTUIModule) Init(ctx context.Context, deps *registry.Dependencies) error {
	return nil
}

func (m *mockTUIModule) Models() []any {
	return nil
}

func (m *mockTUIModule) Handlers() map[string]workflow.TaskHandler {
	return nil
}

func (m *mockTUIModule) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockTUIModule) TUIPages() []registry.TUIPage {
	return m.pages
}

func TestTUI(t *testing.T) {
	// Register mock module with two TUI pages
	p1 := &mockPage{title: "MockPage1"}
	p2 := &mockPage{title: "MockPage2"}
	
	mod := &mockTUIModule{pages: []registry.TUIPage{p1, p2}}
	registry.Register(mod)

	t.Run("Update and View Composable Layout", func(t *testing.T) {
		m := NewModel(context.Background(), nil)
		
		// Ensure both pages are dynamically loaded
		if len(m.pages) < 2 {
			t.Fatalf("expected at least 2 pages loaded, got %d", len(m.pages))
		}

		viewVal := m.View()
		view := viewVal.Content
		if !strings.Contains(view, "HYPERRR CORE ADMIN COMMAND CENTER") {
			t.Errorf("unexpected initial view header")
		}

		if !strings.Contains(view, "MockPage1") || !strings.Contains(view, "MockPage2") {
			t.Errorf("expected view to list dynamically registered tabs")
		}

		// Verify Tab Key navigation switches tabs
		nextModel, cmd := m.Update(tea.KeyPressMsg{Text: "tab", Code: tea.KeyTab})
		m = nextModel.(*Model)
		if cmd != nil {
			t.Error("expected nil cmd for tab navigation")
		}
		if m.activeTab != 1 {
			t.Errorf("expected active tab index to be 1 after Tab keypress, got %d", m.activeTab)
		}

		// Verify number key navigation switches tabs
		nextModel, _ = m.Update(tea.KeyPressMsg{Text: "1", Code: '1'})
		m = nextModel.(*Model)
		if m.activeTab != 0 {
			t.Errorf("expected active tab index to be 0 after '1' keypress, got %d", m.activeTab)
		}

		// Verify keystroke delegation to active sub-page
		testMsg := tea.KeyPressMsg{Text: "x", Code: 'x'}
		_, _ = m.Update(testMsg)

		activePage := m.pages[0].(*mockPage)
		keyMsg, ok := activePage.msgReceived.(tea.KeyPressMsg)
		if !ok || keyMsg.String() != "x" {
			t.Errorf("expected active page to receive 'x' keystroke, got %v", activePage.msgReceived)
		}
	})

	t.Run("Quit command", func(t *testing.T) {
		m := NewModel(context.Background(), nil)
		_, cmd := m.Update(tea.KeyPressMsg{Text: "q", Code: 'q'})
		if cmd() != tea.Quit() {
			t.Error("expected Quit command on q keypress")
		}
	})
}
