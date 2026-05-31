package order

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

func TestOrderTUI(t *testing.T) {
	// Pre-populate test order data
	testOrder := &tuiOrder{
		ID:         "ord_1",
		CustomerID: "cust_123",
		Status:     "PAID",
		TotalPrice: 99.90,
		Items: []tuiOrderItem{
			{
				ProductID: "prod_1",
				Quantity:  2,
				UnitPrice: 49.95,
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "listOrders") {
			resp := map[string]any{
				"data": map[string]any{
					"listOrders": []*tuiOrder{testOrder},
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
	p.Update(tea.KeyPressMsg{Text: "j", Code: 'j'})
	if p.activeRow != 0 { // wrapping scroll on 1 item
		t.Errorf("expected active row index 0, got %d", p.activeRow)
	}

	// Verify list view rendering
	viewStr := p.View()
	if !strings.Contains(viewStr, "ORDER REGISTRATION LOGS") || !strings.Contains(viewStr, "ord_1") {
		t.Errorf("unexpected list view rendering: %s", viewStr)
	}

	// Test enter to open details
	p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	p.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if p.detailMode {
		t.Error("expected detail mode to be false after hitting ESC")
	}
}
