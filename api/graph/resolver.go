package graph

import (
	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/commerce/cart"
	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/identity"
	"github.com/GoHyperrr/hyperrr/internal/workflow"
)

type Resolver struct {
	Projector      *context.Projector
	ProductModule  *product.Module
	CustomerModule *customer.Module
	CartModule     *cart.Module
	OrderModule    *order.Module
	IdentityModule *identity.Module
	Runner         *workflow.Runner
}
