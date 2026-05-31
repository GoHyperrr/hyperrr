package customer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "charm.land/bubbletea/v2"
)

func TestCustomerTUI(t *testing.T) {
	// Pre-populate test customer data
	testCust := &Customer{
		ID:      "cust_1",
		UserID:  "usr_1",
		Name:    "Alice",
		Email:   "alice@example.com",
		Persona: "Sargasso",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "listCustomers") {
			resp := map[string]any{
				"data": map[string]any{
					"listCustomers": []*Customer{testCust},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	deps := &registry.Dependencies{
		Config:    &config.Config{},
		ServerURL: ts.URL,
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

	// Test navigation keypresses in v2
	p.Update(tea.KeyPressMsg{Text: "j", Code: 'j'})
	if p.activeRow != 0 { // wrapping scroll on 1 item
		t.Errorf("expected active row index 0, got %d", p.activeRow)
	}

	// Verify rendering view outputs customer name
	viewStr := p.View()
	if !strings.Contains(viewStr, "CUSTOMER REGISTRATION LOGS") || !strings.Contains(viewStr, "alice@example.com") {
		t.Errorf("unexpected view rendering: %s", viewStr)
	}
}
