package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the master layout and state of the TUI.
type Model struct {
	pages     []registry.TUIPage
	activeTab int
	deps      *registry.Dependencies
	ctx       context.Context
}

// NewModel creates and instantiates the composable master TUI model.
func NewModel(ctx context.Context, deps *registry.Dependencies) *Model {
	m := &Model{
		ctx:  ctx,
		deps: deps,
	}
	m.scanPages()
	return m
}

// scanPages dynamically checks registered modules for TUI page providers.
func (m *Model) scanPages() {
	m.pages = nil
	// Gather pages from registered modules sorted alphabetically by module ID
	modules := registry.List()
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].ID() < modules[j].ID()
	})
	for _, mod := range modules {
		if provider, ok := mod.(registry.TUIProvider); ok {
			m.pages = append(m.pages, provider.TUIPages()...)
		}
	}
}

// Init initializes the master view and calls Init on all registered pages.
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, page := range m.pages {
		if cmdVal := page.Init(m.ctx, m.deps); cmdVal != nil {
			if cmd, ok := cmdVal.(tea.Cmd); ok {
				cmds = append(cmds, cmd)
			}
		}
	}
	return tea.Batch(cmds...)
}

// Update delegates updates to active pages and processes global nav triggers.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		// Global Quit Keys
		if keyStr == "q" || keyStr == "ctrl+c" {
			return m, tea.Quit
		}

		// Direct numeric tab switching (1-9)
		if len(keyStr) == 1 && keyStr[0] >= '1' && keyStr[0] <= '9' {
			idx := int(keyStr[0] - '1')
			if idx < len(m.pages) {
				m.activeTab = idx
				return m, nil
			}
		}

		// Tab and Arrow cycling
		if keyStr == "tab" || keyStr == "right" {
			if len(m.pages) > 0 {
				m.activeTab = (m.activeTab + 1) % len(m.pages)
				return m, nil
			}
		}
		if keyStr == "shift+tab" || keyStr == "left" {
			if len(m.pages) > 0 {
				m.activeTab = (m.activeTab - 1 + len(m.pages)) % len(m.pages)
				return m, nil
			}
		}
	}

	// Delegate processing to the active sub-page
	if len(m.pages) > 0 && m.activeTab < len(m.pages) {
		updatedPage, cmdVal := m.pages[m.activeTab].Update(msg)
		m.pages[m.activeTab] = updatedPage
		if cmd, ok := cmdVal.(tea.Cmd); ok && cmd != nil {
			return m, cmd
		}
	}

	return m, nil
}

// View compiles and renders the layout grid.
func (m *Model) View() string {
	var s strings.Builder

	// 1. Render Header Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#5F5FAF")).
		Padding(0, 1)
	s.WriteString(titleStyle.Render(" HYPERRR CORE ADMIN COMMAND CENTER "))
	s.WriteString("\n\n")

	// 2. Render Navigation Bar
	if len(m.pages) == 0 {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("No active admin views registered by modules."))
		s.WriteString("\n\n")
	} else {
		var tabs []string
		for i, page := range m.pages {
			tabNum := strconv.Itoa(i + 1)
			tabTitle := fmt.Sprintf("[%s] %s", tabNum, page.Title())
			
			if i == m.activeTab {
				// Highlight active tab
				activeStyle := lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#FFD700")).
					Underline(true)
				tabs = append(tabs, activeStyle.Render(tabTitle))
			} else {
				inactiveStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("#8A8A8A"))
				tabs = append(tabs, inactiveStyle.Render(tabTitle))
			}
		}
		s.WriteString(strings.Join(tabs, "   "))
		s.WriteString("\n")
		
		separator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4E4E4E")).
			Render(strings.Repeat("─", 80))
		s.WriteString(separator)
		s.WriteString("\n\n")
	}

	// 3. Render Active Page View
	if len(m.pages) > 0 && m.activeTab < len(m.pages) {
		s.WriteString(m.pages[m.activeTab].View())
	}
	s.WriteString("\n\n")

	// 4. Render Footer Helper
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#585858"))
	s.WriteString(footerStyle.Render("TAB / Left-Right: Switch View | 1-9: Switch to Tab | q: Exit"))

	return s.String()
}
