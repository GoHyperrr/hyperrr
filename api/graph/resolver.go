package graph

import (
	"github.com/GoHyperrr/commerce/product"
	"github.com/GoHyperrr/commerce/customer"
	"github.com/GoHyperrr/commerce/cart"
	"github.com/GoHyperrr/commerce/order"
	"github.com/GoHyperrr/commerce/finance"
	"github.com/GoHyperrr/commerce/notification"
	"github.com/GoHyperrr/commerce/fulfillment"
	"github.com/GoHyperrr/commerce/support"
	"github.com/GoHyperrr/commerce/marketing"
	"github.com/GoHyperrr/commerce/search"
	"github.com/GoHyperrr/commerce/analytics"
	"github.com/GoHyperrr/hyperrr/modules/auth"
	"github.com/GoHyperrr/hyperrr/pkg/ctxengine"
	"github.com/GoHyperrr/hyperrr/modules/identity"
	"github.com/GoHyperrr/hyperrr/pkg/workflow"
)

type Resolver struct {
	Projector          *ctxengine.Projector
	ProductModule      *product.Module
	CustomerModule     *customer.Module
	CartModule         *cart.Module
	OrderModule        *order.Module
	FinanceModule      *finance.Module
	NotificationModule *notification.Module
	FulfillmentModule  *fulfillment.Module
	SupportModule      *support.Module
	MarketingModule    *marketing.Module
	SearchModule       *search.Module
	AnalyticsModule    *analytics.Module
	IdentityModule     *identity.Module
	AuthModule         *auth.Module
	Runner             *workflow.Runner
	Registry           *workflow.Registry
}
