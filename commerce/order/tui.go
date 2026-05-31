package order

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"github.com/charmbracelet/lipgloss"
)

type ordersPage struct {
	db         *db.DB
	repo       *Repository
	orders     []*Order
	activeRow  int
	selectedID string
	detailMode bool
}

func (p *ordersPage) Title() string {
	return "Orders"
}

func (p *ordersPage) Init(ctx context.Context, deps *registry.Dependencies) any {
	p.db = deps.DB
	p.repo = NewRepository(deps.DB)
	p.loadOrders()
	return nil
}

func (p *ordersPage) loadOrders() {
	if p.repo != nil {
		orders, err := p.repo.List(context.Background())
		if err == nil {
			p.orders = orders
		}
	}
}

func (p *ordersPage) Update(msg any) (registry.TUIPage, any) {
	if msgKey, ok := msg.(interface{ String() string }); ok {
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
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render(fmt.Sprintf("ORDER DETAILS: %s", p.selectedID)))
		s.WriteString("\n\n")

		var selectedOrder *Order
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
			s.WriteString(fmt.Sprintf("Created At:   %s\n", selectedOrder.CreatedAt.Format("2006-01-02 15:04:05")))
			s.WriteString("\n")
			s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FAF87")).Render("LINE ITEMS"))
			s.WriteString("\n")
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FAF87"))
			s.WriteString(fmt.Sprintf("%-12s  %-12s  %-10s  %-10s\n",
				headerStyle.Render("PRODUCT ID"),
				headerStyle.Render("UNIT PRICE"),
				headerStyle.Render("QUANTITY"),
				headerStyle.Render("SUBTOTAL")))
			s.WriteString(strings.Repeat("─", 50) + "\n")

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
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("ESC / Backspace: Back to Orders List"))
	} else {
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render("ORDER REGISTRATION LOGS"))
		s.WriteString("\n\n")

		if len(p.orders) == 0 {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("No orders registered in database."))
			s.WriteString("\n")
		} else {
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FAF87"))
			s.WriteString(fmt.Sprintf("%-12s  %-15s  %-12s  %-12s  %-20s\n",
				headerStyle.Render("ORDER ID"),
				headerStyle.Render("CUSTOMER ID"),
				headerStyle.Render("TOTAL PRICE"),
				headerStyle.Render("STATUS"),
				headerStyle.Render("CREATED AT")))
			s.WriteString(strings.Repeat("─", 80) + "\n")

			for i, o := range p.orders {
				rowText := fmt.Sprintf("%-12s  %-15s  %12.2f  %-12s  %-20s",
					o.ID,
					o.CustomerID,
					o.TotalPrice,
					o.Status,
					o.CreatedAt.Format("2006-01-02 15:04:05"))

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
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render("Enter: View Order Details"))
	}

	return s.String()
}
