package scaffold

import "strings"

// ResolveModule takes a module ID or shorthand name and resolves it to a standard package URL and ID.
func ResolveModule(name string) (ModuleInfo, bool) {
	// Standardize name: lower case and trim whitespace
	name = strings.TrimSpace(strings.ToLower(name))
	
	// Direct mappings for shorthand names
	switch name {
	case "auth.apikey", "apikey":
		return ModuleInfo{ID: "auth.apikey", Resolve: "github.com/GoHyperrr/auth/apikey"}, true
	case "auth.emailpass", "emailpass":
		return ModuleInfo{ID: "auth.emailpass", Resolve: "github.com/GoHyperrr/auth/emailpass"}, true
	case "commerce.product", "product":
		return ModuleInfo{ID: "commerce.product", Resolve: "github.com/GoHyperrr/commerce/product"}, true
	case "commerce.store", "store":
		return ModuleInfo{ID: "commerce.store", Resolve: "github.com/GoHyperrr/commerce/store"}, true
	case "commerce.customer", "customer":
		return ModuleInfo{ID: "commerce.customer", Resolve: "github.com/GoHyperrr/commerce/customer"}, true
	case "commerce.cart", "cart":
		return ModuleInfo{ID: "commerce.cart", Resolve: "github.com/GoHyperrr/commerce/cart"}, true
	case "commerce.order", "order":
		return ModuleInfo{ID: "commerce.order", Resolve: "github.com/GoHyperrr/commerce/order"}, true
	case "commerce.payments", "payments":
		return ModuleInfo{ID: "commerce.payments", Resolve: "github.com/GoHyperrr/commerce/payments"}, true
	case "commerce.taxonomy", "taxonomy":
		return ModuleInfo{ID: "commerce.taxonomy", Resolve: "github.com/GoHyperrr/commerce/taxonomy"}, true
	case "commerce.seo", "seo":
		return ModuleInfo{ID: "commerce.seo", Resolve: "github.com/GoHyperrr/commerce/seo"}, true
	case "commerce.notification", "notification":
		return ModuleInfo{ID: "commerce.notification", Resolve: "github.com/GoHyperrr/notification"}, true
	}
	
	// If it contains a slash, treat it as a full repository URL
	if strings.Contains(name, "/") {
		// Try to deduce ID from the path (e.g. github.com/username/plugin-loyalty -> plugin.loyalty or just loyalty)
		parts := strings.Split(name, "/")
		id := parts[len(parts)-1]
		if strings.HasPrefix(id, "hyperrr-") {
			id = strings.TrimPrefix(id, "hyperrr-")
		}
		// If it's github.com/GoHyperrr/... we can keep the namespace prefix
		if strings.HasPrefix(name, "github.com/gohyperrr/") {
			if strings.HasPrefix(name, "github.com/gohyperrr/commerce/") {
				return ModuleInfo{ID: "commerce." + id, Resolve: name}, true
			}
			if strings.HasPrefix(name, "github.com/gohyperrr/auth/") {
				return ModuleInfo{ID: "auth." + id, Resolve: name}, true
			}
		}
		return ModuleInfo{ID: id, Resolve: name}, true
	}
	
	return ModuleInfo{}, false
}
