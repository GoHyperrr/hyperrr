package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type workflowsPage struct {
	projector  *Projector
	lineages   []registry.LineageData
	activeRow  int
	eventCh    chan eventbus.Event
	detailMode bool
	selectedID string
}

func (p *workflowsPage) Title() string {
	return "Workflows"
}

func (p *workflowsPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	// 1. Resolve Projector
	if ctxModVal, ok := registry.Get("core.context"); ok {
		if ctxMod, ok := ctxModVal.(*Module); ok {
			p.projector = ctxMod.Projector()
		}
	}

	p.loadLineages()

	// 2. Setup subscription channel and register event handlers
	p.eventCh = make(chan eventbus.Event, 100)
	eventTypes := []string{
		workflow.EventWorkflowStarted,
		workflow.EventStepStarted,
		workflow.EventStepCompleted,
		workflow.EventStepFailed,
		workflow.EventStepRetrying,
		workflow.EventStepFallback,
		workflow.EventWaitingHuman,
		workflow.EventWorkflowCompleted,
		workflow.EventWorkflowFailed,
	}

	for _, t := range eventTypes {
		_, _ = deps.EventBus.Subscribe(ctx, t, func(ctx context.Context, ev eventbus.Event) error {
			select {
			case p.eventCh <- ev:
			default:
			}
			return nil
		})
	}

	// 3. Return the listening command
	return p.waitForEvent()
}

func (p *workflowsPage) loadLineages() {
	if p.projector != nil {
		p.lineages = p.projector.ListLineages()
	}
}

func (p *workflowsPage) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		return <-p.eventCh
	}
}

func (p *workflowsPage) Update(msg any) (registry.TUIPage, any) {
	switch msg := msg.(type) {
	case eventbus.Event:
		p.loadLineages()
		return p, p.waitForEvent()

	case tea.KeyMsg:
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
				p.selectedID = p.lineages[p.activeRow].GetID()
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
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render(fmt.Sprintf("WORKFLOW RUN DETAILS: %s", p.selectedID)))
		s.WriteString("\n\n")

		var selectedLineage *Lineage
		if p.projector != nil {
			p.projector.mu.RLock()
			if l, ok := p.projector.lineages[p.selectedID]; ok {
				selectedLineage = l
			}
			p.projector.mu.RUnlock()
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
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")).Render(fmt.Sprintf("Error:       %s\n", selectedLineage.Error)))
			}
			s.WriteString("\n")
			s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FAF87")).Render("EXECUTION STEPS"))
			s.WriteString("\n")
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FAF87"))
			s.WriteString(fmt.Sprintf("%-30s  %-15s  %-10s  %-15s\n",
				headerStyle.Render("STEP ID"),
				headerStyle.Render("STATUS"),
				headerStyle.Render("ATTEMPTS"),
				headerStyle.Render("DURATION")))
			s.WriteString(strings.Repeat("─", 80) + "\n")

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
				if step.Error != "" {
					s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8787")).Render(fmt.Sprintf("  ↳ Error: %s\n", step.Error)))
				}
			}
		}
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("ESC / Backspace: Back to Workflows List"))
	} else {
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render("WORKFLOW MONITOR (LIVE)"))
		s.WriteString("\n\n")

		if len(p.lineages) == 0 {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("No workflow executions recorded yet."))
			s.WriteString("\n")
		} else {
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FAF87"))
			s.WriteString(fmt.Sprintf("%-24s  %-24s  %-12s  %-20s\n",
				headerStyle.Render("RUN ID"),
				headerStyle.Render("WORKFLOW NAME"),
				headerStyle.Render("STATUS"),
				headerStyle.Render("STARTED AT")))
			s.WriteString(strings.Repeat("─", 86) + "\n")

			for i, lineage := range p.lineages {
				rowText := fmt.Sprintf("%-24s  %-24s  %-12s  %-20s",
					lineage.GetID(),
					lineage.GetName(),
					lineage.GetState(),
					lineage.GetStartedAt().Format("2006-01-02 15:04:05"))

				if i == p.activeRow {
					selectedStyle := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#121212")).
						Background(lipgloss.Color("#5FAF87"))
					s.WriteString(selectedStyle.Render(rowText) + "\n")
				} else {
					s.WriteString(rowText + "\n")
				}
			}
		}
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("Enter: View Workflow Steps & Details"))
	}

	return s.String()
}
