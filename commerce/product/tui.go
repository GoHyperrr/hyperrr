package product

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type productPage struct {
	mode       int // 0: list, 1: add, 2: update
	db         *db.DB
	repo       *Repository
	products   []*Product
	activeRow  int
	inputs     []textinput.Model
	focusIndex int
	selectedID string
}

func (p *productPage) Title() string {
	return "Products"
}

func (p *productPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	p.db = deps.DB
	p.repo = NewRepository(deps.DB)
	p.loadProducts()

	p.inputs = make([]textinput.Model, 5)

	p.inputs[0] = textinput.New()
	p.inputs[0].Placeholder = "Product ID (e.g. prod_001)"
	p.inputs[0].Focus()

	p.inputs[1] = textinput.New()
	p.inputs[1].Placeholder = "Name"

	p.inputs[2] = textinput.New()
	p.inputs[2].Placeholder = "Description"

	p.inputs[3] = textinput.New()
	p.inputs[3].Placeholder = "Price (e.g. 19.99)"

	p.inputs[4] = textinput.New()
	p.inputs[4].Placeholder = "Currency (default: USD)"

	return nil
}

func (p *productPage) loadProducts() {
	if p.repo != nil {
		prods, err := p.repo.List(context.Background())
		if err == nil {
			p.products = prods
		}
	}
}

func (p *productPage) Update(msg any) (registry.TUIPage, any) {
	switch p.mode {
	case 0: // List Mode
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "j", "down":
				if len(p.products) > 0 {
					p.activeRow = (p.activeRow + 1) % len(p.products)
				}
			case "k", "up":
				if len(p.products) > 0 {
					p.activeRow = (p.activeRow - 1 + len(p.products)) % len(p.products)
				}
			case "a": // Add Mode
				p.mode = 1
				p.focusIndex = 0
				for i := range p.inputs {
					p.inputs[i].Reset()
				}
				p.inputs[0].Focus()
				var cmds []tea.Cmd
				for range p.inputs {
					cmds = append(cmds, textinput.Blink)
				}
				return p, tea.Batch(cmds...)
			case "enter", "u": // Update Mode
				if len(p.products) > 0 && p.activeRow < len(p.products) {
					target := p.products[p.activeRow]
					p.selectedID = target.ID
					p.mode = 2
					p.focusIndex = 1 // Start focus on Name (ID read-only)
					p.inputs[0].SetValue(target.ID)
					p.inputs[1].SetValue(target.Name)
					p.inputs[2].SetValue(target.Description)
					p.inputs[3].SetValue(fmt.Sprintf("%.2f", target.Price))
					p.inputs[4].SetValue(target.Currency)
					p.inputs[1].Focus()
					var cmds []tea.Cmd
					for range p.inputs {
						cmds = append(cmds, textinput.Blink)
					}
					return p, tea.Batch(cmds...)
				}
			case "d": // Delete Mode
				if len(p.products) > 0 && p.activeRow < len(p.products) {
					target := p.products[p.activeRow]
					_ = p.repo.Delete(context.Background(), target.ID)
					p.loadProducts()
					if p.activeRow >= len(p.products) && len(p.products) > 0 {
						p.activeRow = len(p.products) - 1
					}
				}
			}
		}

	case 1, 2: // Add (1) or Update (2) Mode
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				p.mode = 0
				return p, nil
			case "tab", "enter":
				// Submit if on the last field
				if p.focusIndex == len(p.inputs)-1 {
					id := strings.TrimSpace(p.inputs[0].Value())
					name := strings.TrimSpace(p.inputs[1].Value())
					description := strings.TrimSpace(p.inputs[2].Value())
					priceStr := strings.TrimSpace(p.inputs[3].Value())
					currency := strings.TrimSpace(p.inputs[4].Value())

					if currency == "" {
						currency = "USD"
					}

					price, _ := strconv.ParseFloat(priceStr, 64)
					if id != "" && name != "" {
						prod := &Product{
							ID:          id,
							Name:        name,
							Description: description,
							Price:       price,
							Currency:    currency,
						}
						_ = p.repo.Save(context.Background(), prod)
						p.loadProducts()
					}
					p.mode = 0
					return p, nil
				}

				// Cycle focus
				p.inputs[p.focusIndex].Blur()
				p.focusIndex = (p.focusIndex + 1) % len(p.inputs)
				if p.mode == 2 && p.focusIndex == 0 {
					p.focusIndex = 1
				}
				return p, p.inputs[p.focusIndex].Focus()

			case "shift+tab":
				p.inputs[p.focusIndex].Blur()
				p.focusIndex = (p.focusIndex - 1 + len(p.inputs)) % len(p.inputs)
				if p.mode == 2 && p.focusIndex == 0 {
					p.focusIndex = len(p.inputs) - 1
				}
				return p, p.inputs[p.focusIndex].Focus()
			}
		}

		// Update inputs
		var cmd tea.Cmd
		p.inputs[p.focusIndex], cmd = p.inputs[p.focusIndex].Update(msg)
		return p, cmd
	}

	return p, nil
}

func (p *productPage) View() string {
	var s strings.Builder

	switch p.mode {
	case 0: // List Mode
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render("PRODUCT CATALOG"))
		s.WriteString("\n\n")

		if len(p.products) == 0 {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("No products found in database. Press 'a' to add one."))
			s.WriteString("\n")
		} else {
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FAF87"))
			s.WriteString(fmt.Sprintf("%-12s  %-20s  %-25s  %-10s  %-8s\n",
				headerStyle.Render("ID"),
				headerStyle.Render("NAME"),
				headerStyle.Render("DESCRIPTION"),
				headerStyle.Render("PRICE"),
				headerStyle.Render("CURRENCY")))
			s.WriteString(strings.Repeat("─", 80) + "\n")

			for i, prod := range p.products {
				desc := prod.Description
				if len(desc) > 25 {
					desc = desc[:22] + "..."
				}
				rowText := fmt.Sprintf("%-12s  %-20s  %-25s  %8.2f  %-8s",
					prod.ID,
					prod.Name,
					desc,
					prod.Price,
					prod.Currency)

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
		helperStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A"))
		s.WriteString(helperStyle.Render("a: Add Product | Enter / u: Edit Product | d: Delete Product"))

	case 1: // Add Mode
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render("ADD NEW PRODUCT"))
		s.WriteString("\n\n")

		for i, input := range p.inputs {
			labelStyle := lipgloss.NewStyle().Bold(p.focusIndex == i).Foreground(lipgloss.Color("#FFFFFF"))
			s.WriteString(fmt.Sprintf("%-15s %s\n", labelStyle.Render(getLabel(i)+":"), input.View()))
		}
		s.WriteString("\n")
		helperStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A"))
		s.WriteString(helperStyle.Render("TAB: Next Field | Shift+TAB: Previous Field | Enter on last field to SAVE | ESC: Cancel"))

	case 2: // Update Mode
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render(fmt.Sprintf("EDIT PRODUCT: %s", p.selectedID)))
		s.WriteString("\n\n")

		for i, input := range p.inputs {
			labelStyle := lipgloss.NewStyle().Bold(p.focusIndex == i).Foreground(lipgloss.Color("#FFFFFF"))
			if i == 0 {
				readOnlyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A"))
				s.WriteString(fmt.Sprintf("%-15s %s (Read-only)\n", labelStyle.Render(getLabel(i)+":"), readOnlyStyle.Render(input.Value())))
			} else {
				s.WriteString(fmt.Sprintf("%-15s %s\n", labelStyle.Render(getLabel(i)+":"), input.View()))
			}
		}
		s.WriteString("\n")
		helperStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A"))
		s.WriteString(helperStyle.Render("TAB: Next Field | Shift+TAB: Previous Field | Enter on last field to SAVE | ESC: Cancel"))
	}

	return s.String()
}

func getLabel(index int) string {
	switch index {
	case 0:
		return "Product ID"
	case 1:
		return "Name"
	case 2:
		return "Description"
	case 3:
		return "Price"
	case 4:
		return "Currency"
	}
	return ""
}
