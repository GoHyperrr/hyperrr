package graph

import (
	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/internal/context"
)

type Resolver struct {
	Projector     *context.Projector
	ProductModule *product.Module
}
