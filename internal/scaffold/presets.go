package scaffold

type Preset struct {
	Name        string
	Description string
	Modules     []ModuleInfo
}

type ModuleInfo struct {
	ID      string
	Resolve string
}

var Presets = map[string]Preset{
	"commerce-full": {
		Name:        "commerce-full",
		Description: "Full e-commerce capabilities with authentication, cart, catalog, order processing, and payment integrations.",
		Modules: []ModuleInfo{
			{ID: "auth.apikey", Resolve: "github.com/GoHyperrr/auth/apikey"},
			{ID: "auth.emailpass", Resolve: "github.com/GoHyperrr/auth/emailpass"},
			{ID: "commerce.product", Resolve: "github.com/GoHyperrr/commerce/product"},
			{ID: "commerce.store", Resolve: "github.com/GoHyperrr/commerce/store"},
			{ID: "commerce.customer", Resolve: "github.com/GoHyperrr/commerce/customer"},
			{ID: "commerce.cart", Resolve: "github.com/GoHyperrr/commerce/cart"},
			{ID: "commerce.order", Resolve: "github.com/GoHyperrr/commerce/order"},
			{ID: "commerce.payments", Resolve: "github.com/GoHyperrr/commerce/payments"},
			{ID: "commerce.taxonomy", Resolve: "github.com/GoHyperrr/commerce/taxonomy"},
			{ID: "commerce.seo", Resolve: "github.com/GoHyperrr/commerce/seo"},
			{ID: "commerce.notification", Resolve: "github.com/GoHyperrr/notification"},
		},
	},
	"commerce-minimal": {
		Name:        "commerce-minimal",
		Description: "Minimal storefront setup with key modules: key authentication, products, cart, and orders.",
		Modules: []ModuleInfo{
			{ID: "auth.apikey", Resolve: "github.com/GoHyperrr/auth/apikey"},
			{ID: "commerce.product", Resolve: "github.com/GoHyperrr/commerce/product"},
			{ID: "commerce.cart", Resolve: "github.com/GoHyperrr/commerce/cart"},
			{ID: "commerce.order", Resolve: "github.com/GoHyperrr/commerce/order"},
		},
	},
	"auth-only": {
		Name:        "auth-only",
		Description: "API authentication foundation including key/token management and basic credentials.",
		Modules: []ModuleInfo{
			{ID: "auth.apikey", Resolve: "github.com/GoHyperrr/auth/apikey"},
			{ID: "auth.emailpass", Resolve: "github.com/GoHyperrr/auth/emailpass"},
		},
	},
	"bare": {
		Name:        "bare",
		Description: "A blank slate template with zero pre-loaded modules.",
		Modules:     []ModuleInfo{},
	},
}
