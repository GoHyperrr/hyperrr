package customer

import (
	"context"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "github.com/charmbracelet/bubbletea"
)

func TestCustomerTUI(t *testing.T) {
	// Initialize in-memory database
	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    ":memory:",
	}
	database, err := db.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to memory db: %v", err)
	}

	// Auto-migrate Customer schema
	err = database.AutoMigrate(&Customer{})
	if err != nil {
		t.Fatalf("failed to auto-migrate customer: %v", err)
	}

	// Pre-populate test customer
	testCust := &Customer{
		ID:      "cust_1",
		UserID:  "usr_1",
		Name:    "Alice",
		Email:   "alice@example.com",
		Persona: "Sargasso",
	}
	err = database.Save(testCust).Error
	if err != nil {
		t.Fatalf("failed to save test customer: %v", err)
	}

	deps := &registry.Dependencies{
		DB: database,
	}

	ctx := context.Background()
	p := &customerPage{}

	// Verify Title
	if p.Title() != "Customers" {
		t.Errorf("expected title 'Customers', got %s", p.Title())
	}

	// Initialize Page
	p.Init(ctx, deps)

	// Verify item is loaded
	if len(p.customers) != 1 {
		t.Fatalf("expected 1 customer loaded, got %d", len(p.customers))
	}

	loaded := p.customers[0]
	if loaded.ID != "cust_1" || loaded.Email != "alice@example.com" {
		t.Errorf("incorrect customer loaded: %+v", loaded)
	}

	// Test navigation keypresses
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if p.activeRow != 0 { // wrapping scroll on 1 item
		t.Errorf("expected active row index 0, got %d", p.activeRow)
	}

	// Verify rendering view outputs customer name
	viewStr := p.View()
	if !strings.Contains(viewStr, "CUSTOMER REGISTRATION LOGS") || !strings.Contains(viewStr, "alice@example.com") {
		t.Errorf("unexpected view rendering: %s", viewStr)
	}
}
