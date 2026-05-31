package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/theme"
	tea "charm.land/bubbletea/v2"
)

type tuiLineage struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Version   string             `json:"version"`
	State     string             `json:"state"`
	StartedAt time.Time          `json:"startedAt"`
	EndedAt   *time.Time         `json:"endedAt,omitempty"`
	Steps     []*tuiStepLineage  `json:"steps"`
	Error     string             `json:"error,omitempty"`
}

type tuiStepLineage struct {
	StepID    string     `json:"stepId"`
	State     string     `json:"state"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	Attempts  int        `json:"attempts"`
	Error     *string    `json:"error,omitempty"`
}

type workflowsPage struct {
	serverURL  string
	lineages   []*tuiLineage
	activeRow  int
	detailMode bool
	selectedID string
}

func (p *workflowsPage) Title() string {
	return "Workflows"
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (p *workflowsPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	p.serverURL = deps.ServerURL
	p.loadLineages()

	// Return a tick command to trigger periodic polling in decoupled mode
	return tick()
}

func (p *workflowsPage) loadLineages() {
	if p.serverURL != "" {
		var result struct {
			ListLineages []*tuiLineage `json:"listLineages"`
		}
		query := `
			query {
				listLineages {
					id
					name
					version
					state
					startedAt
					endedAt
					steps {
						stepId
						state
						startedAt
						endedAt
						attempts
						error
					}
				}
			}
		`
		if err := registry.QueryGraphQL(p.serverURL, query, nil, &result); err == nil {
			p.lineages = result.ListLineages
		}
	}
}

func (p *workflowsPage) Update(msg any) (registry.TUIPage, any) {
	switch msg := msg.(type) {
	case tickMsg:
		p.loadLineages()
		return p, tick()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if !p.detailMode && len(p.lineages) > 0 {
				p.activeRow = (p.activeRow + 1) % len(p.lineages)
			}
		case "k", "up":
			if !p.detailMode && len(p.lineages) > 0 {
				p.activeRow = (p.activeRow - 1 + len(p.lineages)) % len(p.lineages)
			}
		case "enter":
			if !p.detailMode && len(p.lineages) > 0 && p.activeRow < len(p.lineages) {
				p.selectedID = p.lineages[p.activeRow].ID
				p.detailMode = true
			}
		case "esc", "backspace":
			if p.detailMode {
				p.detailMode = false
			}
		}
	}
	return p, nil
}

func (p *workflowsPage) View() string {
	var s strings.Builder

	if p.detailMode {
		s.WriteString(theme.TitleStyle.Render(fmt.Sprintf("WORKFLOW RUN DETAILS: %s", p.selectedID)))
		s.WriteString("\n\n")

		var selectedLineage *tuiLineage
		for _, l := range p.lineages {
			if l.ID == p.selectedID {
				selectedLineage = l
				break
			}
		}

		if selectedLineage == nil {
			s.WriteString("Workflow run detail not found in memory.\n")
		} else {
			s.WriteString(fmt.Sprintf("Name:        %s\n", selectedLineage.Name))
			s.WriteString(fmt.Sprintf("Version:     %s\n", selectedLineage.Version))
			s.WriteString(fmt.Sprintf("Status:      %s\n", selectedLineage.State))
			s.WriteString(fmt.Sprintf("Started At:  %s\n", selectedLineage.StartedAt.Format("2006-01-02 15:04:05")))
			if selectedLineage.EndedAt != nil {
				s.WriteString(fmt.Sprintf("Ended At:    %s\n", selectedLineage.EndedAt.Format("2006-01-02 15:04:05")))
				s.WriteString(fmt.Sprintf("Duration:    %s\n", selectedLineage.EndedAt.Sub(selectedLineage.StartedAt).Round(time.Millisecond)))
			} else {
				s.WriteString(fmt.Sprintf("Duration:    Running for %s\n", time.Since(selectedLineage.StartedAt).Round(time.Second)))
			}
			if selectedLineage.Error != "" {
				s.WriteString(theme.ErrorStyle.Render(fmt.Sprintf("Error:       %s\n", selectedLineage.Error)))
			}
			s.WriteString("\n")
			s.WriteString(theme.TitleStyle.Render("EXECUTION STEPS"))
			s.WriteString("\n")
			headerStyle := theme.TableHeaderStyle
			s.WriteString(fmt.Sprintf("%-30s  %-15s  %-10s  %-15s\n",
				headerStyle.Render("STEP ID"),
				headerStyle.Render("STATUS"),
				headerStyle.Render("ATTEMPTS"),
				headerStyle.Render("DURATION")))
			s.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 80)) + "\n")

			for _, step := range selectedLineage.Steps {
				duration := "N/A"
				if step.EndedAt != nil {
					duration = step.EndedAt.Sub(step.StartedAt).Round(time.Millisecond).String()
				} else if step.State == "RUNNING" {
					duration = time.Since(step.StartedAt).Round(time.Second).String()
				}
				s.WriteString(fmt.Sprintf("%-30s  %-15s  %10d  %-15s\n",
					step.StepID,
					step.State,
					step.Attempts,
					duration))
				if step.Error != nil && *step.Error != "" {
					s.WriteString(theme.ErrorStyle.Render(fmt.Sprintf("  ↳ Error: %s\n", *step.Error)))
				}
			}
		}
		s.WriteString("\n")
		s.WriteString(theme.MutedStyle.Render("ESC / Backspace: Back to Workflows List"))
	} else {
		s.WriteString(theme.TitleStyle.Render("WORKFLOW MONITOR (LIVE)"))
		s.WriteString("\n\n")

		if len(p.lineages) == 0 {
			s.WriteString(theme.MutedStyle.Render("No workflow executions recorded yet."))
			s.WriteString("\n")
		} else {
			headerStyle := theme.TableHeaderStyle
			s.WriteString(fmt.Sprintf("%-24s  %-24s  %-12s  %-20s\n",
				headerStyle.Render("RUN ID"),
				headerStyle.Render("WORKFLOW NAME"),
				headerStyle.Render("STATUS"),
				headerStyle.Render("STARTED AT")))
			s.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 86)) + "\n")

			for i, lineage := range p.lineages {
				rowText := fmt.Sprintf("%-24s  %-24s  %-12s  %-20s",
					lineage.ID,
					lineage.Name,
					lineage.State,
					lineage.StartedAt.Format("2006-01-02 15:04:05"))

				if i == p.activeRow {
					s.WriteString(theme.SelectedRowStyle.Render(rowText) + "\n")
				} else {
					s.WriteString(rowText + "\n")
				}
			}
		}
		s.WriteString("\n")
		s.WriteString(theme.MutedStyle.Render("Enter: View Workflow Steps & Details"))
	}

	return s.String()
}
