package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/api/graph/model"
	"github.com/GoHyperrr/hyperrr/api/middleware"
	"github.com/GoHyperrr/commerce/cart"
	"github.com/GoHyperrr/commerce/support"
	"github.com/GoHyperrr/commerce/order"
	"github.com/GoHyperrr/commerce/customer"
	"github.com/GoHyperrr/commerce/fulfillment"
	"github.com/GoHyperrr/commerce/product"
	ctxEngine "github.com/GoHyperrr/hyperrr/pkg/ctxengine"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/google/uuid"
)

// --- From analytics.resolvers.go ---

func (r *queryResolver) GetSystemStats(ctx context.Context) (*model.SystemStats, error) {
	if r.Projector == nil {
		return nil, fmt.Errorf("analytics module not initialized with projector")
	}

	lineages := r.Projector.ListLineages()

	total := len(lineages)
	if total == 0 {
		return &model.SystemStats{}, nil
	}

	success := 0
	failed := 0
	var totalDuration time.Duration

	for _, l := range lineages {
		if l.GetState() == workflow.StateCompleted {
			success++
		} else if l.GetState() == workflow.StateFailed {
			failed++
		}
		if l.GetEndedAt() != nil && !l.GetEndedAt().IsZero() {
			totalDuration += l.GetEndedAt().Sub(l.GetStartedAt())
		}
	}

	return &model.SystemStats{
		TotalWorkflows:     total,
		SuccessRate:        (float64(success) / float64(total)) * 100,
		FailureRate:        (float64(failed) / float64(total)) * 100,
		AvgExecutionTimeMs: float64(totalDuration.Milliseconds()) / float64(total),
	}, nil
}

func (r *queryResolver) GetSalesStats(ctx context.Context) (*model.SalesStats, error) {
	if r.Projector == nil {
		return nil, fmt.Errorf("analytics module not initialized with projector")
	}

	lineages := r.Projector.ListLineages()

	var totalRevenue float64
	orderCount := 0
	orderIDs := make(map[string]bool)

	for _, l := range lineages {
		// Look for successful fulfillment workflows
		if l.GetName() == "fulfillment.v1" && l.GetState() == workflow.StateCompleted {
			// Extract revenue from lineage events
			conc, ok := l.(*ctxEngine.Lineage)
			if ok {
				for _, ev := range conc.Events {
					if ev.Type == "order.paid" {
						if p, ok := ev.Payload.(map[string]any); ok {
							if total, ok := p["total_price"].(float64); ok {
								totalRevenue += total
								orderCount++
								if id, ok := p["order_id"].(string); ok {
									orderIDs[id] = true
								}
								break
							}
						}
					}
				}
			}
		}
	}

	// Secondary check: Database reconciliation
	if r.OrderModule != nil {
		orders, err := r.OrderModule.Repo().List(ctx)
		if err == nil {
			for _, o := range orders {
				if (o.Status == order.OrderPaid || o.Status == order.OrderFulfilled) && !orderIDs[o.ID] {
					totalRevenue += o.TotalPrice
					orderCount++
					orderIDs[o.ID] = true
				}
			}
		}
	}

	avg := 0.0
	if orderCount > 0 {
		avg = totalRevenue / float64(orderCount)
	}

	return &model.SalesStats{
		TotalRevenue:  totalRevenue,
		OrderCount:    orderCount,
		AvgOrderValue: avg,
	}, nil
}

// --- From apikey.resolvers.go ---

func (r *mutationResolver) CreateAPIKey(ctx context.Context, name string, expiresAt *time.Time) (*model.GeneratedAPIKey, error) {
	actor, ok := middleware.ForContext(ctx)
	if !ok {
		return nil, fmt.Errorf("unauthorized")
	}

	if r.APIKeyModule == nil {
		return nil, fmt.Errorf("API Key module not loaded")
	}

	key, err := r.APIKeyModule.CreateAPIKey(ctx, actor.ID, name, expiresAt)
	if err != nil {
		return nil, err
	}

	return &model.GeneratedAPIKey{
		ID:        key.ID,
		Name:      key.Name,
		Key:       key.Key,
		ActorID:   key.ActorID,
		ExpiresAt: key.ExpiresAt,
		CreatedAt: key.CreatedAt,
	}, nil
}

func (r *mutationResolver) RevokeAPIKey(ctx context.Context, id string) (bool, error) {
	actor, ok := middleware.ForContext(ctx)
	if !ok {
		return false, fmt.Errorf("unauthorized")
	}

	if r.APIKeyModule == nil {
		return false, fmt.Errorf("API Key module not loaded")
	}

	return r.APIKeyModule.RevokeAPIKey(ctx, actor.ID, id)
}

func (r *queryResolver) ListAPIKeys(ctx context.Context) ([]*model.APIKeyInfo, error) {
	actor, ok := middleware.ForContext(ctx)
	if !ok {
		return nil, fmt.Errorf("unauthorized")
	}

	if r.APIKeyModule == nil {
		return nil, fmt.Errorf("API Key module not loaded")
	}

	keys, err := r.APIKeyModule.ListAPIKeys(ctx, actor.ID)
	if err != nil {
		return nil, err
	}

	res := make([]*model.APIKeyInfo, len(keys))
	for i, key := range keys {
		res[i] = &model.APIKeyInfo{
			ID:        key.ID,
			Name:      key.Name,
			ActorID:   key.ActorID,
			ExpiresAt: key.ExpiresAt,
			CreatedAt: key.CreatedAt,
		}
	}
	return res, nil
}

// --- From cart.resolvers.go ---

func (r *mutationResolver) AddItemToCart(ctx context.Context, cartID string, input model.AddItemInput) (*model.Cart, error) {
	wf, err := r.Registry.Get("cart.add")
	if err != nil {
		return nil, err
	}

	workflowInput := map[string]any{
		"cart_id":    cartID,
		"product_id": input.ProductID,
		"quantity":   input.Quantity,
		"price":      input.Price,
	}

	execID := "add_item_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	resRaw, ok := results["add"]
	if !ok {
		return nil, fmt.Errorf("failed to retrieve updated cart from workflow results")
	}

	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from add step")
	}

	var domainRes cart.Cart
	if err := decodeResult(resMap["cart"], &domainRes); err != nil {
		return nil, fmt.Errorf("invalid cart type in results: %w", err)
	}

	return mapCartToModel(&domainRes), nil
}

func (r *mutationResolver) RemoveItemFromCart(ctx context.Context, cartID string, itemID string) (*model.Cart, error) {
	wf, err := r.Registry.Get("cart.remove")
	if err != nil {
		return nil, err
	}

	workflowInput := map[string]any{
		"cart_id": cartID,
		"item_id": itemID,
	}

	execID := "remove_item_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	resRaw, ok := results["remove"]
	if !ok {
		return nil, fmt.Errorf("failed to retrieve updated cart from workflow results")
	}

	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from remove step")
	}

	var domainRes cart.Cart
	if err := decodeResult(resMap["cart"], &domainRes); err != nil {
		return nil, fmt.Errorf("invalid cart type in results: %w", err)
	}

	return mapCartToModel(&domainRes), nil
}

func (r *mutationResolver) CheckoutCart(ctx context.Context, cartID string) (bool, error) {
	wf, err := r.Registry.Get("cart.checkout")
	if err != nil {
		return false, err
	}

	workflowInput := map[string]any{
		"cart_id": cartID,
	}

	execID := "checkout_" + uuid.New().String()
	_, err = r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *queryResolver) GetCart(ctx context.Context, id string) (*model.Cart, error) {
	c, err := r.CartModule.Repo().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapCartToModel(c), nil
}

func (r *queryResolver) GetActiveCart(ctx context.Context, customerID string) (*model.Cart, error) {
	c, err := r.CartModule.Repo().GetActiveByCustomerID(ctx, customerID)
	if err != nil {
		// If not found, create a new one
		newCart := &cart.Cart{
			ID:         "cart_" + uuid.New().String(),
			CustomerID: customerID,
			Status:     cart.CartActive,
		}
		if err := r.CartModule.Repo().Save(ctx, newCart); err != nil {
			return nil, err
		}
		return mapCartToModel(newCart), nil
	}
	return mapCartToModel(c), nil
}

// --- From context.resolvers.go ---

func (r *queryResolver) GetWorkflowLineage(ctx context.Context, id string) (*model.WorkflowLineage, error) {
	lineage, err := r.Projector.GetLineage(id)
	if err != nil {
		return nil, err
	}

	return mapToModel(lineage), nil
}

func (r *queryResolver) ListLineages(ctx context.Context) ([]*model.WorkflowLineage, error) {
	lineages := r.Projector.ListLineages()
	res := make([]*model.WorkflowLineage, 0, len(lineages))
	for _, l := range lineages {
		conc, ok := l.(*ctxEngine.Lineage)
		if ok {
			res = append(res, mapToModel(conc))
		}
	}
	return res, nil
}

func (r *workflowLineageResolver) Events(ctx context.Context, obj *model.WorkflowLineage) ([]*model.Event, error) {
	lineage, err := r.Projector.GetLineage(obj.ID)
	if err != nil {
		return nil, err
	}
	res := mapToModel(lineage)
	return res.Events, nil
}

func (r *workflowLineageResolver) RelatedLineages(ctx context.Context, obj *model.WorkflowLineage) ([]*model.WorkflowLineage, error) {
	lineages, err := r.Projector.GetRelatedLineages(ctx, obj.ID)
	if err != nil {
		return nil, err
	}

	res := make([]*model.WorkflowLineage, 0, len(lineages))
	for _, l := range lineages {
		res = append(res, mapToModel(l))
	}
	return res, nil
}


// --- From customer.resolvers.go ---

func (r *mutationResolver) UpdateCustomer(ctx context.Context, id string, input model.UpdateCustomerInput) (*model.Customer, error) {
	wf := &workflow.Workflow{
		Name: "customer.update",
		Steps: []workflow.Step{
			{ID: "customer.update_details", Uses: "customer.update_details"},
		},
	}

	workflowInput := map[string]any{
		"id": id,
	}
	if input.Name != nil {
		workflowInput["name"] = *input.Name
	}
	if input.Email != nil {
		workflowInput["email"] = *input.Email
	}

	execID := "update_cust_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	resRaw, ok := results["customer.update_details"]
	if !ok {
		return nil, fmt.Errorf("failed to retrieve updated customer from workflow results")
	}

	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from update step")
	}

	var domainRes customer.Customer
	if err := decodeResult(resMap["customer"], &domainRes); err != nil {
		return nil, fmt.Errorf("invalid customer type in results: %w", err)
	}

	res := &model.Customer{
		ID:      domainRes.ID,
		UserID:  domainRes.UserID,
		Name:    domainRes.Name,
		Email:   domainRes.Email,
		Persona: &domainRes.Persona,
	}

	for _, a := range domainRes.Addresses {
		res.Addresses = append(res.Addresses, &model.Address{
			ID:      a.ID,
			Line1:   a.Line1,
			Line2:   &a.Line2,
			City:    a.City,
			State:   a.State,
			Zip:     a.Zip,
			Country: a.Country,
		})
	}

	return res, nil
}

func (r *queryResolver) GetCustomer(ctx context.Context, id string) (*model.Customer, error) {
	c, err := r.CustomerModule.Repo().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	res := &model.Customer{
		ID:      c.ID,
		UserID:  c.UserID,
		Name:    c.Name,
		Email:   c.Email,
		Persona: &c.Persona,
	}

	for _, a := range c.Addresses {
		res.Addresses = append(res.Addresses, &model.Address{
			ID:      a.ID,
			Line1:   a.Line1,
			Line2:   &a.Line2,
			City:    a.City,
			State:   a.State,
			Zip:     a.Zip,
			Country: a.Country,
		})
	}

	return res, nil
}

func (r *queryResolver) ListCustomers(ctx context.Context) ([]*model.Customer, error) {
	list, err := r.CustomerModule.Repo().List(ctx)
	if err != nil {
		return nil, err
	}

	var res []*model.Customer
	for _, c := range list {
		cust := &model.Customer{
			ID:      c.ID,
			UserID:  c.UserID,
			Name:    c.Name,
			Email:   c.Email,
			Persona: &c.Persona,
		}
		for _, a := range c.Addresses {
			addr := a
			cust.Addresses = append(cust.Addresses, &model.Address{
				ID:      addr.ID,
				Line1:   addr.Line1,
				Line2:   &addr.Line2,
				City:    addr.City,
				State:   addr.State,
				Zip:     addr.Zip,
				Country: addr.Country,
			})
		}
		res = append(res, cust)
	}

	return res, nil
}

// --- From emailpass.resolvers.go ---

func (r *mutationResolver) Register(ctx context.Context, email string, password string, name string) (*model.AuthResponse, error) {
	actor, err := r.EmailPassModule.Register(ctx, email, password, name)
	if err != nil {
		return nil, err
	}

	token, err := r.EmailPassModule.Store().GenerateToken(*actor)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &model.AuthResponse{
		Token: token,
		Actor: &model.Actor{
			ID:   actor.ID,
			Type: string(actor.Type),
			Name: actor.Name,
		},
	}, nil
}

func (r *mutationResolver) Login(ctx context.Context, email string, password string) (*model.AuthResponse, error) {
	actor, err := r.EmailPassModule.Login(ctx, email, password)
	if err != nil {
		return nil, err
	}

	token, err := r.EmailPassModule.Store().GenerateToken(*actor)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &model.AuthResponse{
		Token: token,
		Actor: &model.Actor{
			ID:   actor.ID,
			Type: string(actor.Type),
			Name: actor.Name,
		},
	}, nil
}

func (r *queryResolver) Me(ctx context.Context) (*model.Actor, error) {
	actor, ok := middleware.ForContext(ctx)
	if !ok {
		return nil, fmt.Errorf("unauthorized")
	}

	return &model.Actor{
		ID:   actor.ID,
		Type: string(actor.Type),
		Name: actor.Name,
	}, nil
}

// --- From finance.resolvers.go ---

func (r *queryResolver) GetPayment(ctx context.Context, id string) (*model.Payment, error) {
	p, err := r.FinanceModule.Repo().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapPaymentToModel(p), nil
}

func (r *queryResolver) ListOrderPayments(ctx context.Context, orderID string) ([]*model.Payment, error) {
	payments, err := r.FinanceModule.Repo().ListByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	res := make([]*model.Payment, 0, len(payments))
	for _, p := range payments {
		res = append(res, mapPaymentToModel(p))
	}
	return res, nil
}

// --- From fulfillment.resolvers.go ---

func (r *mutationResolver) UpdateShipmentStatus(ctx context.Context, shipmentID string, trackingNumber *string, carrier *string) (*model.Shipment, error) {
	wf, err := r.Registry.Get("fulfillment.ship_order")
	if err != nil {
		return nil, err
	}

	workflowInput := map[string]any{
		"shipment_id": shipmentID,
	}
	if trackingNumber != nil {
		workflowInput["tracking_number"] = *trackingNumber
	}
	if carrier != nil {
		workflowInput["carrier"] = *carrier
	}

	execID := "ship_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	resRaw, ok := results["ship"]
	if !ok {
		return nil, fmt.Errorf("failed to retrieve updated shipment from workflow results")
	}

	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from ship step")
	}

	domainRes, ok := resMap["shipment"].(*fulfillment.Shipment)
	if !ok {
		return nil, fmt.Errorf("invalid shipment type in results")
	}

	return mapShipmentToModel(domainRes), nil
}

func (r *queryResolver) GetInventory(ctx context.Context, productID string) (*model.Inventory, error) {
	inv, err := r.FulfillmentModule.Repo().GetInventoryByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}
	return mapInventoryToModel(inv), nil
}

func (r *queryResolver) GetShipment(ctx context.Context, id string) (*model.Shipment, error) {
	s, err := r.FulfillmentModule.Repo().GetShipment(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapShipmentToModel(s), nil
}

func (r *queryResolver) GetShipmentByOrder(ctx context.Context, orderID string) (*model.Shipment, error) {
	s, err := r.FulfillmentModule.Repo().GetShipmentByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return mapShipmentToModel(s), nil
}

// --- From marketing.resolvers.go ---

func (r *mutationResolver) ApplyCouponToCart(ctx context.Context, cartID string, couponCode string) (*model.Cart, error) {
	wf, err := r.Registry.Get("marketing.apply_coupon")
	if err != nil {
		return nil, err
	}

	// In a real system, cart.add_item would be specialized to handle discounts.
	// For now we'll just validate and then manually add a discount item to the cart.
	workflowInput := map[string]any{
		"cart_id":     cartID,
		"coupon_code": couponCode,
		"product_id":  "DISCOUNT_PROMO",
		"quantity":    1.0,
		"price":       0.0, // Simplified
	}

	execID := "promo_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	// The cart.add_item handler returns the updated cart under step ID "apply"
	c, ok := results["apply"].(*cart.Cart)
	if !ok {
		// If apply didn't run because validate returned something else, just fetch the cart
		c, err = r.CartModule.Repo().GetByID(ctx, cartID)
		if err != nil {
			return nil, err
		}
	}

	return mapCartToModel(c), nil
}

func (r *queryResolver) GetCoupon(ctx context.Context, code string) (*model.Coupon, error) {
	c, err := r.MarketingModule.Repo().GetCouponByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return mapCouponToModel(c), nil
}

func (r *queryResolver) GetLoyaltyBalance(ctx context.Context, customerID string) (int, error) {
	lp, err := r.MarketingModule.Repo().GetLoyaltyPointsByCustomerID(ctx, customerID)
	if err != nil {
		return 0, nil // Default balance
	}
	return lp.Balance, nil
}

// --- From notification.resolvers.go ---

func (r *queryResolver) ListNotifications(ctx context.Context, recipient *string) ([]*model.Notification, error) {
	recip := ""
	if recipient != nil {
		recip = *recipient
	}

	notifs, err := r.NotificationModule.Repo().List(ctx, recip)
	if err != nil {
		return nil, err
	}

	res := make([]*model.Notification, 0, len(notifs))
	for _, n := range notifs {
		res = append(res, mapNotificationToModel(n))
	}

	return res, nil
}

// --- From order.resolvers.go ---

func (r *mutationResolver) CreateOrderFromCart(ctx context.Context, cartID string) (*model.Order, error) {
	c, err := r.CartModule.Repo().GetByID(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("cart not found: %w", err)
	}

	if len(c.Items) == 0 {
		return nil, fmt.Errorf("cart is empty")
	}

	// Prepare workflow input
	items := make([]any, 0, len(c.Items))
	for _, item := range c.Items {
		items = append(items, map[string]any{
			"product_id": item.ProductID,
			"quantity":   float64(item.Quantity), // json/map expects float64 for numbers
			"price":      item.Price,
		})
	}

	workflowInput := map[string]any{
		"customer_id": c.CustomerID,
		"cart_id":     c.ID,
		"items":       items,
	}

	wf, err := r.Registry.Get("fulfillment.v1")
	if err != nil {
		return nil, err
	}

	execID := "fulfill_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	oRaw, ok := results["order.finalize"]
	if !ok {
		oRaw, ok = results["order.create"]
	}
	if !ok {
		return nil, fmt.Errorf("failed to retrieve order from workflow results")
	}

	resMap, ok := oRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from order step")
	}

	var o order.Order
	if err := decodeResult(resMap["order"], &o); err != nil {
		return nil, fmt.Errorf("invalid order type in results: %w", err)
	}

	return mapOrderToModel(&o), nil
}

func (r *queryResolver) GetOrder(ctx context.Context, id string) (*model.Order, error) {
	o, err := r.OrderModule.Repo().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapOrderToModel(o), nil
}

func (r *queryResolver) ListOrders(ctx context.Context) ([]*model.Order, error) {
	orders, err := r.OrderModule.Repo().List(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]*model.Order, 0, len(orders))
	for _, o := range orders {
		res = append(res, mapOrderToModel(o))
	}
	return res, nil
}

func (r *queryResolver) ListCustomerOrders(ctx context.Context, customerID string) ([]*model.Order, error) {
	orders, err := r.OrderModule.Repo().ListByCustomerID(ctx, customerID)
	if err != nil {
		return nil, err
	}

	res := make([]*model.Order, 0, len(orders))
	for _, o := range orders {
		res = append(res, mapOrderToModel(o))
	}
	return res, nil
}

// --- From product.resolvers.go ---

func (r *mutationResolver) CreateProduct(ctx context.Context, input model.CreateProductInput) (*model.Product, error) {
	wf, err := r.Registry.Get("product.create")
	if err != nil {
		return nil, err
	}

	desc := ""
	if input.Description != nil {
		desc = *input.Description
	}

	workflowInput := map[string]any{
		"id":          input.ID,
		"name":        input.Name,
		"description": desc,
		"price":       input.Price,
	}

	execID := "create_prod_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	resRaw, ok := results["persist"]
	if !ok {
		return nil, fmt.Errorf("failed to retrieve created product from workflow results")
	}

	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from persist step")
	}

	var domainRes product.Product
	if err := decodeResult(resMap["product"], &domainRes); err != nil {
		return nil, fmt.Errorf("invalid product type in results: %w", err)
	}

	return mapProductToModel(&domainRes), nil
}

func (r *mutationResolver) UpdateProduct(ctx context.Context, id string, input model.UpdateProductInput) (*model.Product, error) {
	wf, err := r.Registry.Get("product.update")
	if err != nil {
		return nil, err
	}

	workflowInput := map[string]any{
		"id": id,
	}
	if input.Name != nil {
		workflowInput["name"] = *input.Name
	}
	if input.Description != nil {
		workflowInput["description"] = *input.Description
	}
	if input.Price != nil {
		workflowInput["price"] = *input.Price
	}

	execID := "update_prod_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	resRaw, ok := results["update"]
	if !ok {
		return nil, fmt.Errorf("failed to retrieve updated product from workflow results")
	}

	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from update step")
	}

	var domainRes product.Product
	if err := decodeResult(resMap["product"], &domainRes); err != nil {
		return nil, fmt.Errorf("invalid product type in results: %w", err)
	}

	return mapProductToModel(&domainRes), nil
}

func (r *mutationResolver) DeleteProduct(ctx context.Context, id string) (bool, error) {
	p, err := r.ProductModule.Repo().GetByID(ctx, id)
	if err != nil {
		return false, fmt.Errorf("product not found")
	}

	err = r.ProductModule.Repo().Delete(ctx, p.ID)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *queryResolver) GetProduct(ctx context.Context, id string) (*model.Product, error) {
	p, err := r.ProductModule.Repo().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return mapProductToModel(p), nil
}

func (r *queryResolver) ListProducts(ctx context.Context) ([]*model.Product, error) {
	products, err := r.ProductModule.Repo().List(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]*model.Product, 0, len(products))
	for _, p := range products {
		res = append(res, mapProductToModel(p))
	}
	return res, nil
}

// --- From schema.resolvers.go ---

func (r *mutationResolver) Health(ctx context.Context) (string, error) {
	return "", fmt.Errorf("not implemented: Health - _health")
}

func (r *queryResolver) Health(ctx context.Context) (string, error) {
	return "OK", nil
}


// --- From search.resolvers.go ---

func (r *queryResolver) SearchProducts(ctx context.Context, query string, limit *int) ([]*model.Product, error) {
	wf, err := r.Registry.Get("search.products")
	if err != nil {
		return nil, err
	}

	lim := 10.0
	if limit != nil {
		lim = float64(*limit)
	}

	workflowInput := map[string]any{
		"query": query,
		"limit": lim,
	}

	execID := "search_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	var prodsRaw []*product.Product
	if err := decodeResult(results["search"], &prodsRaw); err != nil {
		return nil, fmt.Errorf("failed to retrieve search results from workflow: %w", err)
	}

	res := make([]*model.Product, 0, len(prodsRaw))
	for _, p := range prodsRaw {
		res = append(res, mapProductToModel(p))
	}
	return res, nil
}

// --- From support.resolvers.go ---

func (r *mutationResolver) CreateTicket(ctx context.Context, customerID string, subject string, message string) (*model.Ticket, error) {
	wf, err := r.Registry.Get("support.create")
	if err != nil {
		return nil, err
	}

	workflowInput := map[string]any{
		"customer_id": customerID,
		"subject":     subject,
		"message":     message,
	}

	execID := "tkt_wf_" + uuid.New().String()
	results, err := r.Runner.Execute(ctx, execID, wf, workflowInput)
	if err != nil {
		return nil, err
	}

	// CreateTicket handler returns map[string]any{"ticket": *support.Ticket} under step ID "ticket"
	resRaw, ok := results["ticket"]
	if !ok {
		return nil, fmt.Errorf("missing ticket from workflow results")
	}
	resMap, ok := resRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid result format from ticket step")
	}
	t, ok := resMap["ticket"].(*support.Ticket)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve ticket from results")
	}

	return mapTicketToModel(t), nil
}

func (r *mutationResolver) AddTicketMessage(ctx context.Context, ticketID string, sender string, content string) (*model.Message, error) {
	msg := &support.Message{
		ID:        "msg_" + uuid.New().String(),
		TicketID:  ticketID,
		Sender:    support.SenderType(sender),
		Content:   content,
		CreatedAt: time.Now(),
	}

	if err := r.SupportModule.Repo().SaveMessage(ctx, msg); err != nil {
		return nil, err
	}

	return mapMessageToModel(msg), nil
}

func (r *queryResolver) GetTicket(ctx context.Context, id string) (*model.Ticket, error) {
	t, err := r.SupportModule.Repo().GetTicketByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapTicketToModel(t), nil
}

func (r *queryResolver) ListCustomerTickets(ctx context.Context, customerID string) ([]*model.Ticket, error) {
	tickets, err := r.SupportModule.Repo().ListTicketsByCustomerID(ctx, customerID)
	if err != nil {
		return nil, err
	}

	res := make([]*model.Ticket, 0, len(tickets))
	for _, t := range tickets {
		res = append(res, mapTicketToModel(t))
	}
	return res, nil
}

