package product

import (
	"context"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "github.com/charmbracelet/bubbletea"
)

func TestProductTUI(t *testing.T) {
	// Initialize in-memory database
	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    ":memory:",
	}
	database, err := db.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to memory db: %v", err)
	}

	// Auto-migrate Product schema
	err = database.AutoMigrate(&Product{})
	if err != nil {
		t.Fatalf("failed to auto-migrate product: %v", err)
	}

	deps := &registry.Dependencies{
		DB: database,
	}

	ctx := context.Background()
	p := &productPage{}

	// Verify Title
	if p.Title() != "Products" {
		t.Errorf("expected title 'Products', got %s", p.Title())
	}

	// Initialize Page
	p.Init(ctx, deps)

	// Ensure list starts empty
	if len(p.products) != 0 {
		t.Errorf("expected 0 initial products, got %d", len(p.products))
	}

	// 1. Simulate key 'a' to enter ADD mode
	resPage, cmdVal := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	p = resPage.(*productPage)
	if p.mode != 1 {
		t.Errorf("expected mode 1 (ADD), got %d", p.mode)
	}
	if cmdVal == nil {
		t.Error("expected non-nil blinking cmd on entering ADD mode")
	}

	// 2. Input product values
	p.inputs[0].SetValue("prod_tui_123") // ID
	p.inputs[1].SetValue("TUI Product")  // Name
	p.inputs[2].SetValue("TUI Desc")     // Description
	p.inputs[3].SetValue("25.50")        // Price
	p.inputs[4].SetValue("USD")          // Currency

	// Submit (focusIndex is 4, sending Tab/Enter will submit since it's the last index)
	p.focusIndex = 4
	resPage, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p = resPage.(*productPage)

	// Verify mode returns to List
	if p.mode != 0 {
		t.Errorf("expected mode 0 (LIST) after submit, got %d", p.mode)
	}

	// Verify item is saved to database and loaded
	if len(p.products) != 1 {
		t.Fatalf("expected 1 product loaded, got %d", len(p.products))
	}

	saved := p.products[0]
	if saved.ID != "prod_tui_123" || saved.Name != "TUI Product" || saved.Price != 25.50 {
		t.Errorf("incorrect product values saved: %+v", saved)
	}

	// 3. Simulate key 'u' on the selected row to enter UPDATE mode
	p.activeRow = 0
	resPage, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	p = resPage.(*productPage)
	if p.mode != 2 {
		t.Errorf("expected mode 2 (UPDATE), got %d", p.mode)
	}
	if p.inputs[0].Value() != "prod_tui_123" {
		t.Errorf("expected pre-populated ID, got %s", p.inputs[0].Value())
	}

	// Edit price
	p.inputs[3].SetValue("30.00")
	p.focusIndex = 4 // Focus currency/last field
	resPage, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p = resPage.(*productPage)

	// Verify price is updated
	if len(p.products) != 1 {
		t.Fatalf("expected 1 product after update, got %d", len(p.products))
	}
	updated := p.products[0]
	if updated.Price != 30.00 {
		t.Errorf("expected updated price 30.00, got %f", updated.Price)
	}

	// Verify view rendering doesn't panic
	listViewStr := p.View()
	if !strings.Contains(listViewStr, "PRODUCT CATALOG") || !strings.Contains(listViewStr, "TUI Product") {
		t.Errorf("unexpected list rendering: %s", listViewStr)
	}

	// Verify add view rendering
	p.mode = 1
	addViewStr := p.View()
	if !strings.Contains(addViewStr, "ADD NEW PRODUCT") {
		t.Errorf("unexpected add view rendering: %s", addViewStr)
	}

	// Verify update view rendering
	p.mode = 2
	updateViewStr := p.View()
	if !strings.Contains(updateViewStr, "EDIT PRODUCT") {
		t.Errorf("unexpected edit view rendering: %s", updateViewStr)
	}
}
