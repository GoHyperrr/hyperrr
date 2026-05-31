package product

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

func TestProductTUI(t *testing.T) {
	// 1. Spin up mock GraphQL server
	var productsResponse = []*Product{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "listProducts") {
			resp := map[string]any{
				"data": map[string]any{
					"listProducts": productsResponse,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else if strings.Contains(req.Query, "createProduct") {
			inputRaw := req.Variables["input"].(map[string]any)
			newProd := &Product{
				ID:          inputRaw["id"].(string),
				Name:        inputRaw["name"].(string),
				Description: inputRaw["description"].(string),
				Price:       inputRaw["price"].(float64),
				Currency:    inputRaw["currency"].(string),
			}
			productsResponse = append(productsResponse, newProd)

			resp := map[string]any{
				"data": map[string]any{
					"createProduct": map[string]any{
						"id": newProd.ID,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else if strings.Contains(req.Query, "updateProduct") {
			id := req.Variables["id"].(string)
			inputRaw := req.Variables["input"].(map[string]any)
			for _, p := range productsResponse {
				if p.ID == id {
					p.Name = inputRaw["name"].(string)
					p.Description = inputRaw["description"].(string)
					p.Price = inputRaw["price"].(float64)
					p.Currency = inputRaw["currency"].(string)
				}
			}
			resp := map[string]any{
				"data": map[string]any{
					"updateProduct": map[string]any{
						"id": id,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else if strings.Contains(req.Query, "deleteProduct") {
			id := req.Variables["id"].(string)
			newProds := []*Product{}
			for _, p := range productsResponse {
				if p.ID != id {
					newProds = append(newProds, p)
				}
			}
			productsResponse = newProds
			resp := map[string]any{
				"data": map[string]any{
					"deleteProduct": true,
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
	resPage, cmdVal := p.Update(tea.KeyPressMsg{Text: "a", Code: 'a'})
	p = resPage.(*productPage)
	if p.mode != modeAdd {
		t.Errorf("expected mode modeAdd (ADD), got %v", p.mode)
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

	// Submit
	p.focusIndex = 4
	resPage, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	p = resPage.(*productPage)

	// Verify mode returns to List
	if p.mode != modeList {
		t.Errorf("expected mode modeList (LIST) after submit, got %v", p.mode)
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
	resPage, _ = p.Update(tea.KeyPressMsg{Text: "u", Code: 'u'})
	p = resPage.(*productPage)
	if p.mode != modeUpdate {
		t.Errorf("expected mode modeUpdate (UPDATE), got %v", p.mode)
	}
	if p.inputs[0].Value() != "prod_tui_123" {
		t.Errorf("expected pre-populated ID, got %s", p.inputs[0].Value())
	}

	// Edit price
	p.inputs[3].SetValue("30.00")
	p.focusIndex = 4
	resPage, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	p.mode = modeAdd
	addViewStr := p.View()
	if !strings.Contains(addViewStr, "ADD NEW PRODUCT") {
		t.Errorf("unexpected add view rendering: %s", addViewStr)
	}

	// Verify update view rendering
	p.mode = modeUpdate
	updateViewStr := p.View()
	if !strings.Contains(updateViewStr, "EDIT PRODUCT") {
		t.Errorf("unexpected edit view rendering: %s", updateViewStr)
	}
}
