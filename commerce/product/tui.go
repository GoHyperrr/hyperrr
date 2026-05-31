package product

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/theme"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type pageMode int

const (
	modeList pageMode = iota
	modeAdd
	modeUpdate
)

type productPage struct {
	mode            pageMode
	serverURL       string
	products        []*Product
	activeRow       int
	inputs          []textinput.Model
	focusIndex      int
	selectedID      string
	validationError string
	statusMessage   string
}

func (p *productPage) Title() string {
	return "Products"
}

func (p *productPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	p.serverURL = deps.ServerURL
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
	if p.serverURL != "" {
		var result struct {
			ListProducts []*Product `json:"listProducts"`
		}
		query := `
			query {
				listProducts {
					id
					name
					description
					price
					currency
				}
			}
		`
		if err := registry.QueryGraphQL(p.serverURL, query, nil, &result); err == nil {
			p.products = result.ListProducts
		}
	}
}

func (p *productPage) Update(msg any) (registry.TUIPage, any) {
	switch p.mode {
	case modeList:
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
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
				p.mode = modeAdd
				p.focusIndex = 0
				p.validationError = ""
				p.statusMessage = ""
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
					p.mode = modeUpdate
					p.validationError = ""
					p.statusMessage = ""
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
					query := `
						mutation($id: ID!) {
							deleteProduct(id: $id)
						}
					`
					var result struct {
						DeleteProduct bool `json:"deleteProduct"`
					}
					err := registry.QueryGraphQL(p.serverURL, query, map[string]any{"id": target.ID}, &result)
					if err != nil {
						p.statusMessage = fmt.Sprintf("Error deleting product: %v", err)
					} else {
						p.statusMessage = fmt.Sprintf("Success: Deleted product %s", target.ID)
					}
					p.loadProducts()
					if p.activeRow >= len(p.products) && len(p.products) > 0 {
						p.activeRow = len(p.products) - 1
					}
				}
			}
		}

	case modeAdd, modeUpdate:
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "esc":
				p.mode = modeList
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

					price, err := strconv.ParseFloat(priceStr, 64)
					if err != nil {
						p.validationError = "Invalid price format. Must be a decimal number (e.g. 19.99)"
						return p, nil
					}
					if id == "" || name == "" {
						p.validationError = "Product ID and Name cannot be empty"
						return p, nil
					}

					p.validationError = ""
					if p.mode == modeAdd {
						// Create Product
						query := `
							mutation($input: CreateProductInput!) {
								createProduct(input: $input) {
									id
								}
							}
						`
						var result any
						variables := map[string]any{
							"input": map[string]any{
								"id":          id,
								"name":        name,
								"description": description,
								"price":       price,
								"currency":    currency,
							},
						}
						err = registry.QueryGraphQL(p.serverURL, query, variables, &result)
						if err != nil {
							p.validationError = fmt.Sprintf("Failed to create product: %v", err)
							return p, nil
						}
						p.statusMessage = fmt.Sprintf("Success: Created product %s", id)
					} else {
						// Update Product
						query := `
							mutation($id: ID!, $input: UpdateProductInput!) {
								updateProduct(id: $id, input: $input) {
									id
								}
							}
						`
						var result any
						variables := map[string]any{
							"id": id,
							"input": map[string]any{
								"name":        name,
								"description": description,
								"price":       price,
								"currency":    currency,
							},
						}
						err = registry.QueryGraphQL(p.serverURL, query, variables, &result)
						if err != nil {
							p.validationError = fmt.Sprintf("Failed to update product: %v", err)
							return p, nil
						}
						p.statusMessage = fmt.Sprintf("Success: Updated product %s", id)
					}
					p.loadProducts()
					p.mode = modeList
					return p, nil
				}

				// Cycle focus
				p.inputs[p.focusIndex].Blur()
				p.focusIndex = (p.focusIndex + 1) % len(p.inputs)
				if p.mode == modeUpdate && p.focusIndex == 0 {
					p.focusIndex = 1
				}
				return p, p.inputs[p.focusIndex].Focus()

			case "shift+tab":
				p.inputs[p.focusIndex].Blur()
				p.focusIndex = (p.focusIndex - 1 + len(p.inputs)) % len(p.inputs)
				if p.mode == modeUpdate && p.focusIndex == 0 {
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
	case modeList:
		s.WriteString(theme.TitleStyle.Render("PRODUCT CATALOG"))
		s.WriteString("\n\n")

		if p.statusMessage != "" {
			if strings.Contains(strings.ToLower(p.statusMessage), "error") || strings.Contains(strings.ToLower(p.statusMessage), "failed") {
				s.WriteString(theme.ErrorStyle.Render("⚠️  "+p.statusMessage) + "\n\n")
			} else {
				s.WriteString(theme.SuccessStyle.Render("✨ "+p.statusMessage) + "\n\n")
			}
		}

		if len(p.products) == 0 {
			s.WriteString(theme.MutedStyle.Render("No products found in database. Press 'a' to add one."))
			s.WriteString("\n")
		} else {
			headerStyle := theme.TableHeaderStyle
			s.WriteString(fmt.Sprintf("%-12s  %-20s  %-25s  %-10s  %-8s\n",
				headerStyle.Render("ID"),
				headerStyle.Render("NAME"),
				headerStyle.Render("DESCRIPTION"),
				headerStyle.Render("PRICE"),
				headerStyle.Render("CURRENCY")))
			s.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 80)) + "\n")

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
					s.WriteString(theme.SelectedRowStyle.Render(rowText) + "\n")
				} else {
					s.WriteString(rowText + "\n")
				}
			}
		}
		s.WriteString("\n")
		s.WriteString(theme.MutedStyle.Render("a: Add Product | Enter / u: Edit Product | d: Delete Product"))

	case modeAdd:
		s.WriteString(theme.TitleStyle.Render("ADD NEW PRODUCT"))
		s.WriteString("\n\n")

		if p.validationError != "" {
			s.WriteString(theme.ErrorStyle.Render("⚠️  "+p.validationError) + "\n\n")
		}

		for i, input := range p.inputs {
			var labelStyle lipgloss.Style
			if p.focusIndex == i {
				labelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.ColorPrimaryBlue))
			} else {
				labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
			}
			s.WriteString(fmt.Sprintf("%-15s %s\n", labelStyle.Render(getLabel(i)+":"), input.View()))
		}
		s.WriteString("\n")
		s.WriteString(theme.MutedStyle.Render("TAB: Next Field | Shift+TAB: Previous Field | Enter on last field to SAVE | ESC: Cancel"))

	case modeUpdate:
		s.WriteString(theme.TitleStyle.Render(fmt.Sprintf("EDIT PRODUCT: %s", p.selectedID)))
		s.WriteString("\n\n")

		if p.validationError != "" {
			s.WriteString(theme.ErrorStyle.Render("⚠️  "+p.validationError) + "\n\n")
		}

		for i, input := range p.inputs {
			var labelStyle lipgloss.Style
			if p.focusIndex == i {
				labelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.ColorPrimaryBlue))
			} else {
				labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
			}
			if i == 0 {
				s.WriteString(fmt.Sprintf("%-15s %s (Read-only)\n", labelStyle.Render(getLabel(i)+":"), theme.MutedStyle.Render(input.Value())))
			} else {
				s.WriteString(fmt.Sprintf("%-15s %s\n", labelStyle.Render(getLabel(i)+":"), input.View()))
			}
		}
		s.WriteString("\n")
		s.WriteString(theme.MutedStyle.Render("TAB: Next Field | Shift+TAB: Previous Field | Enter on last field to SAVE | ESC: Cancel"))
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
