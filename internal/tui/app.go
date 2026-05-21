package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WorkflowStatus represents the TUI's view of a workflow.
type WorkflowStatus struct {
	ID    string
	Name  string
	State string
	Step  string
}

// Model represents the TUI state.
type Model struct {
	workflows map[string]*WorkflowStatus
	order     []string
	bus       eventbus.EventBus
	ctx       context.Context
	err       error
}

// NewModel creates a new TUI model.
func NewModel(ctx context.Context, bus eventbus.EventBus) *Model {
	return &Model{
		workflows: make(map[string]*WorkflowStatus),
		bus:       bus,
		ctx:       ctx,
	}
}

// Init initializes the TUI.
func (m *Model) Init() tea.Cmd {
	return m.subscribe()
}

func (m *Model) subscribe() tea.Cmd {
	return func() tea.Msg {
		// Mock subscription for now
		return nil
	}
}

// WorkflowMsg is a message from the event bus.
type WorkflowMsg struct {
	ID    string
	Name  string
	State string
	Step  string
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case WorkflowMsg:
		if _, ok := m.workflows[msg.ID]; !ok {
			m.order = append(m.order, msg.ID)
			m.workflows[msg.ID] = &WorkflowStatus{ID: msg.ID, Name: msg.Name}
		}
		wf := m.workflows[msg.ID]
		if msg.State != "" {
			wf.State = msg.State
		}
		if msg.Step != "" {
			wf.Step = msg.Step
		}
	}
	return m, nil
}

func (m *Model) View() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Render("hyperrr Mission Control"))
	s.WriteString("\n\n")

	for _, id := range m.order {
		wf := m.workflows[id]
		stateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		if wf.State == "WAITING_HUMAN" {
			stateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
		}
		
		s.WriteString(fmt.Sprintf("[%s] %s: %s (Step: %s)\n", 
			stateStyle.Render(wf.State), 
			wf.Name, 
			wf.ID, 
			wf.Step))
	}

	s.WriteString("\n(press q to quit)\n")
	return s.String()
}
