package customer

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/charmbracelet/lipgloss"
)

type customerPage struct {
	db        *db.DB
	repo      *Repository
	customers []*Customer
	activeRow int
}

func (p *customerPage) Title() string {
	return "Customers"
}

func (p *customerPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	p.db = deps.DB
	p.repo = NewRepository(deps.DB)
	p.loadCustomers()
	return nil
}

func (p *customerPage) loadCustomers() {
	if p.db != nil {
		var list []*Customer
		err := p.db.WithContext(context.Background()).Find(&list).Error
		if err == nil {
			p.customers = list
		}
	}
}

func (p *customerPage) Update(msg any) (registry.TUIPage, any) {
	// Scroll controls
	if msgKey, ok := msg.(interface{ String() string }); ok {
		switch msgKey.String() {
		case "j", "down":
			if len(p.customers) > 0 {
				p.activeRow = (p.activeRow + 1) % len(p.customers)
			}
		case "k", "up":
			if len(p.customers) > 0 {
				p.activeRow = (p.activeRow - 1 + len(p.customers)) % len(p.customers)
			}
		}
	}
	return p, nil
}

func (p *customerPage) View() string {
	var s strings.Builder

	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render("CUSTOMER REGISTRATION LOGS"))
	s.WriteString("\n\n")

	if len(p.customers) == 0 {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("No customer accounts registered in database."))
		s.WriteString("\n")
	} else {
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FAF87"))
		s.WriteString(fmt.Sprintf("%-12s  %-12s  %-20s  %-25s  %-12s\n",
			headerStyle.Render("ID"),
			headerStyle.Render("USER ID"),
			headerStyle.Render("NAME"),
			headerStyle.Render("EMAIL"),
			headerStyle.Render("PERSONA")))
		s.WriteString(strings.Repeat("─", 89) + "\n")

		for i, cust := range p.customers {
			rowText := fmt.Sprintf("%-12s  %-12s  %-20s  %-25s  %-12s",
				cust.ID,
				cust.UserID,
				cust.Name,
				cust.Email,
				cust.Persona)

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

	return s.String()
}
