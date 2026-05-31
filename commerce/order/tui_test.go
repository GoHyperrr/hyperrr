package order

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "github.com/charmbracelet/bubbletea"
)

func TestOrderTUI(t *testing.T) {
	// Initialize in-memory database
	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    ":memory:",
	}
	database, err := db.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to memory db: %v", err)
	}

	// Auto-migrate Order schema
	err = database.AutoMigrate(&Order{}, &OrderItem{})
	if err != nil {
		t.Fatalf("failed to auto-migrate order: %v", err)
	}

	// Pre-populate test order
	testOrder := &Order{
		ID:         "ord_1",
		CustomerID: "cust_123",
		Status:     OrderPaid,
		TotalPrice: 99.90,
		Items: []OrderItem{
			{
				ID:        "item_1",
				OrderID:   "ord_1",
				ProductID: "prod_1",
				Quantity:  2,
				UnitPrice: 49.95,
			},
		},
		CreatedAt: time.Now(),
	}
	err = database.Save(testOrder).Error
	if err != nil {
		t.Fatalf("failed to save test order: %v", err)
	}

	deps := &registry.Dependencies{
		DB: database,
	}

	ctx := context.Background()
	p := &ordersPage{}

	// Verify Title
	if p.Title() != "Orders" {
		t.Errorf("expected title 'Orders', got %s", p.Title())
	}

	// Initialize Page
	p.Init(ctx, deps)

	// Verify item is loaded
	if len(p.orders) != 1 {
		t.Fatalf("expected 1 order loaded, got %d", len(p.orders))
	}

	loaded := p.orders[0]
	if loaded.ID != "ord_1" || loaded.CustomerID != "cust_123" || loaded.TotalPrice != 99.90 {
		t.Errorf("incorrect order loaded: %+v", loaded)
	}

	// Test navigation keypresses
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if p.activeRow != 0 { // wrapping scroll on 1 item
		t.Errorf("expected active row index 0, got %d", p.activeRow)
	}

	// Verify list view rendering
	viewStr := p.View()
	if !strings.Contains(viewStr, "ORDER REGISTRATION LOGS") || !strings.Contains(viewStr, "ord_1") {
		t.Errorf("unexpected list view rendering: %s", viewStr)
	}

	// Test enter to open details
	p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.detailMode {
		t.Error("expected detail mode to be true after hitting Enter")
	}
	if p.selectedID != "ord_1" {
		t.Errorf("expected selected order ID 'ord_1', got %s", p.selectedID)
	}

	// Verify detail view rendering
	detailViewStr := p.View()
	if !strings.Contains(detailViewStr, "ORDER DETAILS: ord_1") || !strings.Contains(detailViewStr, "LINE ITEMS") {
		t.Errorf("unexpected detail view rendering: %s", detailViewStr)
	}

	// Test esc to return to list
	p.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if p.detailMode {
		t.Error("expected detail mode to be false after hitting ESC")
	}
}
