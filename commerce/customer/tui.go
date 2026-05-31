package customer

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/theme"
	tea "charm.land/bubbletea/v2"
)

type customerPage struct {
	serverURL string
	customers []*Customer
	activeRow int
}

func (p *customerPage) Title() string {
	return "Customers"
}

func (p *customerPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	p.serverURL = deps.ServerURL
	p.loadCustomers()
	return nil
}

func (p *customerPage) loadCustomers() {
	if p.serverURL != "" {
		var result struct {
			ListCustomers []*Customer `json:"listCustomers"`
		}
		query := `
			query {
				listCustomers {
					id
					userId
					name
					email
					persona
				}
			}
		`
		if err := registry.QueryGraphQL(p.serverURL, query, nil, &result); err == nil {
			p.customers = result.ListCustomers
		}
	}
}

func (p *customerPage) Update(msg any) (registry.TUIPage, any) {
	if _, ok := msg.(registry.PageFocusMsg); ok {
		p.loadCustomers()
		return p, nil
	}

	// Scroll controls in Bubble Tea v2
	if msgKey, ok := msg.(tea.KeyPressMsg); ok {
		switch msgKey.String() {
		case "r":
			p.loadCustomers()
			return p, nil
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

	s.WriteString(theme.TitleStyle.Render("CUSTOMER REGISTRATION LOGS"))
	s.WriteString("\n\n")

	if len(p.customers) == 0 {
		s.WriteString(theme.MutedStyle.Render("No customer accounts registered in database."))
		s.WriteString("\n")
	} else {
		headerStyle := theme.TableHeaderStyle
		s.WriteString(fmt.Sprintf("%-12s  %-12s  %-20s  %-25s  %-12s\n",
			headerStyle.Render("ID"),
			headerStyle.Render("USER ID"),
			headerStyle.Render("NAME"),
			headerStyle.Render("EMAIL"),
			headerStyle.Render("PERSONA")))
		s.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 89)) + "\n")

		for i, cust := range p.customers {
			rowText := fmt.Sprintf("%-12s  %-12s  %-20s  %-25s  %-12s",
				cust.ID,
				cust.UserID,
				cust.Name,
				cust.Email,
				cust.Persona)

			if i == p.activeRow {
				s.WriteString(theme.SelectedRowStyle.Render(rowText) + "\n")
			} else {
				s.WriteString(rowText + "\n")
			}
		}
	}

	s.WriteString("\n")
	s.WriteString(theme.MutedStyle.Render("r: Refresh List"))
	return s.String()
}
