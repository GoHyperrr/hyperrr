package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/theme"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Model represents the master layout and state of the TUI.
type Model struct {
	pages             []registry.TUIPage
	activeTab         int
	deps              *registry.Dependencies
	ctx               context.Context
	serverURL         string
	connected         bool
	connectionChecked bool
}

type connectionResultMsg struct {
	connected bool
	err       error
}

func checkConnection(serverURL string) tea.Cmd {
	return func() tea.Msg {
		// Verify server presence using standard typename introspection query
		query := `query { __typename }`
		var result any
		err := registry.QueryGraphQL(serverURL, query, nil, &result)
		return connectionResultMsg{
			connected: err == nil,
			err:       err,
		}
	}
}

// NewModel creates and instantiates the composable master TUI model.
func NewModel(ctx context.Context, deps *registry.Dependencies) *Model {
	var serverURL string
	if deps != nil {
		serverURL = deps.ServerURL
	}
	m := &Model{
		ctx:       ctx,
		deps:      deps,
		serverURL: serverURL,
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
	cmds = append(cmds, checkConnection(m.serverURL))
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
	case connectionResultMsg:
		m.connected = msg.connected
		m.connectionChecked = true
		return m, nil

	case tea.KeyPressMsg:
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
				updatedPage, cmdVal := m.pages[idx].Update(registry.PageFocusMsg{})
				m.pages[idx] = updatedPage
				if cmd, ok := cmdVal.(tea.Cmd); ok && cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		}

		// Tab and Arrow cycling
		if keyStr == "tab" || keyStr == "right" {
			if len(m.pages) > 0 {
				m.activeTab = (m.activeTab + 1) % len(m.pages)
				updatedPage, cmdVal := m.pages[m.activeTab].Update(registry.PageFocusMsg{})
				m.pages[m.activeTab] = updatedPage
				if cmd, ok := cmdVal.(tea.Cmd); ok && cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		}
		if keyStr == "shift+tab" || keyStr == "left" {
			if len(m.pages) > 0 {
				m.activeTab = (m.activeTab - 1 + len(m.pages)) % len(m.pages)
				updatedPage, cmdVal := m.pages[m.activeTab].Update(registry.PageFocusMsg{})
				m.pages[m.activeTab] = updatedPage
				if cmd, ok := cmdVal.(tea.Cmd); ok && cmd != nil {
					return m, cmd
				}
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
func (m *Model) View() tea.View {
	var s strings.Builder

	// 1. Render custom uppercase 'HYPERRR' Logo with custom asterisk glyph
	asteriskBlock := []string{
		"          ██        ",
		"      ██  ██  ██    ",
		"        ██████      ",
		"   ██ ██████████ ██ ",
		"        ██████      ",
		"      ██  ██  ██    ",
		"          ██        ",
	}
	hBlock := []string{
		" ██╗  ██╗██╗   ██╗██████╗ ███████╗██████╗ ██████╗ ██████╗ ",
		" ██║  ██║╚██╗ ██╔╝██╔══██╗██╔════╝██╔══██╗██╔══██╗██╔══██╗",
		" ███████║ ╚████╔╝ ██████╔╝█████╗  ██████╔╝██████╔╝██████╔╝",
		" ██╔══██║  ╚██╔╝  ██╔═══╝ ██╔══╝  ██╔══██╗██╔══██╗██╔══██╗",
		" ██║  ██║   ██║   ██║     ███████╗██║  ██║██║  ██║██║  ██║",
		" ╚═╝  ╚═╝   ╚═╝   ╚═╝     ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝",
	}

	glyphStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorAccentLime))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorWhite))

	var logoLines []string
	// Vertically center the 6-line wordmark with the 7-line glyph (empty line on line 0 for text)
	for i := 0; i < 7; i++ {
		glyphPart := glyphStyle.Render(asteriskBlock[i])
		var textPart string
		if i > 0 {
			textPart = textStyle.Render(hBlock[i-1])
		} else {
			textPart = ""
		}
		logoLines = append(logoLines, glyphPart+textPart)
	}
	s.WriteString(strings.Join(logoLines, "\n") + "\n\n")

	// 2. Render Header Title
	s.WriteString(theme.HeaderStyle.Render(" 🦄 HYPERRR CORE ADMIN COMMAND CENTER 🦄 "))
	s.WriteString("\n")

	// 3. Render Connection Status Bar
	statusText := fmt.Sprintf("Connected to: %s", m.serverURL)
	if !m.connectionChecked {
		statusText += " (Checking connection...)"
		s.WriteString(theme.MutedStyle.Render(statusText) + "\n\n")
	} else if m.connected {
		statusText += " (Online)"
		s.WriteString(theme.SuccessStyle.Render(statusText) + "\n\n")
	} else {
		statusText += " (Offline)"
		s.WriteString(theme.ErrorStyle.Render(statusText) + "\n\n")
	}

	// 2. Render Navigation Bar
	if len(m.pages) == 0 {
		s.WriteString(theme.MutedStyle.Render("No active admin views registered by modules."))
		s.WriteString("\n\n")
	} else {
		var tabs []string
		for i, page := range m.pages {
			tabNum := strconv.Itoa(i + 1)
			tabTitle := fmt.Sprintf("[%s] %s", tabNum, page.Title())
			
			if i == m.activeTab {
				// Highlight active tab
				tabs = append(tabs, theme.ActiveTabStyle.Render(tabTitle))
			} else {
				tabs = append(tabs, theme.InactiveTabStyle.Render(tabTitle))
			}
		}
		s.WriteString(strings.Join(tabs, "   "))
		s.WriteString("\n")
		
		separator := theme.SeparatorStyle.Render(strings.Repeat("─", 80))
		s.WriteString(separator)
		s.WriteString("\n\n")
	}

	// 3. Render Active Page View
	if len(m.pages) > 0 && m.activeTab < len(m.pages) {
		s.WriteString(m.pages[m.activeTab].View())
	}
	s.WriteString("\n\n")

	// 4. Render Footer Helper
	s.WriteString(theme.MutedStyle.Render("TAB / Left-Right: Switch View | 1-9: Switch to Tab | q: Exit"))

	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}
