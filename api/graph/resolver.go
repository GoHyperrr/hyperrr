package graph

import (
	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/internal/context"
	"github.com/GoHyperrr/hyperrr/internal/identity"
)

type Resolver struct {
	Projector      *context.Projector
	ProductModule  *product.Module
	CustomerModule *customer.Module
	IdentityModule *identity.Module
}
