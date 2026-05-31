package order

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/GoHyperrr/hyperrr/pkg/theme"
	tea "charm.land/bubbletea/v2"
)

type tuiOrder struct {
	ID         string         `json:"id"`
	CustomerID string         `json:"customerId"`
	Status     string         `json:"status"`
	TotalPrice float64        `json:"totalPrice"`
	Items      []tuiOrderItem `json:"items"`
}

type tuiOrderItem struct {
	ProductID string  `json:"productId"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
}

type ordersPage struct {
	serverURL  string
	orders     []*tuiOrder
	activeRow  int
	selectedID string
	detailMode bool
}

func (p *ordersPage) Title() string {
	return "Orders"
}

func (p *ordersPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	p.serverURL = deps.ServerURL
	p.loadOrders()
	return nil
}

func (p *ordersPage) loadOrders() {
	if p.serverURL != "" {
		var result struct {
			ListOrders []*tuiOrder `json:"listOrders"`
		}
		query := `
			query {
				listOrders {
					id
					customerId
					status
					totalPrice
					items {
						productId
						quantity
						unitPrice
					}
				}
			}
		`
		if err := registry.QueryGraphQL(p.serverURL, query, nil, &result); err == nil {
			p.orders = result.ListOrders
		}
	}
}

func (p *ordersPage) Update(msg any) (registry.TUIPage, any) {
	if msgKey, ok := msg.(tea.KeyPressMsg); ok {
		switch msgKey.String() {
		case "j", "down":
			if !p.detailMode && len(p.orders) > 0 {
				p.activeRow = (p.activeRow + 1) % len(p.orders)
			}
		case "k", "up":
			if !p.detailMode && len(p.orders) > 0 {
				p.activeRow = (p.activeRow - 1 + len(p.orders)) % len(p.orders)
			}
		case "enter":
			if !p.detailMode && len(p.orders) > 0 && p.activeRow < len(p.orders) {
				p.selectedID = p.orders[p.activeRow].ID
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

func (p *ordersPage) View() string {
	var s strings.Builder

	if p.detailMode {
		s.WriteString(theme.TitleStyle.Render(fmt.Sprintf("ORDER DETAILS: %s", p.selectedID)))
		s.WriteString("\n\n")

		var selectedOrder *tuiOrder
		for _, o := range p.orders {
			if o.ID == p.selectedID {
				selectedOrder = o
				break
			}
		}

		if selectedOrder == nil {
			s.WriteString("Order not found.\n")
		} else {
			s.WriteString(fmt.Sprintf("Customer ID:  %s\n", selectedOrder.CustomerID))
			s.WriteString(fmt.Sprintf("Status:       %s\n", selectedOrder.Status))
			s.WriteString(fmt.Sprintf("Total Price:  %.2f\n", selectedOrder.TotalPrice))
			s.WriteString("\n")
			s.WriteString(theme.TitleStyle.Render("LINE ITEMS"))
			s.WriteString("\n")
			headerStyle := theme.TableHeaderStyle
			s.WriteString(fmt.Sprintf("%-12s  %-12s  %-10s  %-10s\n",
				headerStyle.Render("PRODUCT ID"),
				headerStyle.Render("UNIT PRICE"),
				headerStyle.Render("QUANTITY"),
				headerStyle.Render("SUBTOTAL")))
			s.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 50)) + "\n")

			for _, item := range selectedOrder.Items {
				subtotal := float64(item.Quantity) * item.UnitPrice
				s.WriteString(fmt.Sprintf("%-12s  %12.2f  %10d  %10.2f\n",
					item.ProductID,
					item.UnitPrice,
					item.Quantity,
					subtotal))
			}
		}
		s.WriteString("\n")
		s.WriteString(theme.MutedStyle.Render("ESC / Backspace: Back to Orders List"))
	} else {
		s.WriteString(theme.TitleStyle.Render("ORDER REGISTRATION LOGS"))
		s.WriteString("\n\n")

		if len(p.orders) == 0 {
			s.WriteString(theme.MutedStyle.Render("No orders registered in database."))
			s.WriteString("\n")
		} else {
			headerStyle := theme.TableHeaderStyle
			s.WriteString(fmt.Sprintf("%-12s  %-15s  %-12s  %-12s\n",
				headerStyle.Render("ORDER ID"),
				headerStyle.Render("CUSTOMER ID"),
				headerStyle.Render("TOTAL PRICE"),
				headerStyle.Render("STATUS")))
			s.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 55)) + "\n")

			for i, o := range p.orders {
				rowText := fmt.Sprintf("%-12s  %-15s  %12.2f  %-12s",
					o.ID,
					o.CustomerID,
					o.TotalPrice,
					o.Status)

				if i == p.activeRow {
					s.WriteString(theme.SelectedRowStyle.Render(rowText) + "\n")
				} else {
					s.WriteString(rowText + "\n")
				}
			}
		}
		s.WriteString("\n")
		s.WriteString(theme.MutedStyle.Render("Enter: View Order Details"))
	}

	return s.String()
}
