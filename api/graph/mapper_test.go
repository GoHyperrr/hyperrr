package graph

import (
	"testing"
	"github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/commerce/cart"
	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/commerce/finance"
	"github.com/GoHyperrr/hyperrr/commerce/notification"
	"github.com/GoHyperrr/hyperrr/commerce/fulfillment"
	"github.com/GoHyperrr/hyperrr/commerce/support"
	"github.com/GoHyperrr/hyperrr/commerce/marketing"
)

func TestMappers(t *testing.T) {
	t.Run("Lineage", func(t *testing.T) {
		l := &context.Lineage{ID: "l1"}
		mapToModel(l)
	})

	t.Run("Cart", func(t *testing.T) {
		c := &cart.Cart{ID: "c1"}
		mapCartToModel(c)
	})

	t.Run("Order", func(t *testing.T) {
		o := &order.Order{ID: "o1"}
		mapOrderToModel(o)
	})

	t.Run("Payment", func(t *testing.T) {
		p := &finance.Payment{ID: "p1"}
		mapPaymentToModel(p)
	})

	t.Run("Notification", func(t *testing.T) {
		n := &notification.Notification{ID: "n1"}
		mapNotificationToModel(n)
	})

	t.Run("Fulfillment", func(t *testing.T) {
		inv := &fulfillment.Inventory{ID: "i1"}
		mapInventoryToModel(inv)
		ship := &fulfillment.Shipment{ID: "s1"}
		mapShipmentToModel(ship)
	})

	t.Run("Support", func(t *testing.T) {
		tick := &support.Ticket{ID: "t1"}
		mapTicketToModel(tick)
	})

	t.Run("Marketing", func(t *testing.T) {
		coup := &marketing.Coupon{ID: "cp1"}
		mapCouponToModel(coup)
	})

	t.Run("Product", func(t *testing.T) {
		p := &product.Product{ID: "p1"}
		mapProductToModel(p)
	})
}
