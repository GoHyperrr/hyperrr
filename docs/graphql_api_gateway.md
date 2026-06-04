# GraphQL API Gateway & Dynamic Resolver Architecture

Hyperrr implements a **Zero Core Pollution** GraphQL API gateway (`api/graph`). Module authors can define schemas and resolvers entirely within their module directories, and the build tool (`hyperrr build`) dynamically aggregates schemas and generates resolver delegation code at compile-time.

---

## 1. Zero Core Pollution Architecture Overview

In traditional GraphQL architectures, adding a field or query requires modifying core entry points. In Hyperrr, the core GraphQL gateway is a skeleton container. Individual modules are completely decoupled (residing in their own Go workspaces like `commerce` and `auth`), and the resolver container is auto-stitched:

```
                  +--------------------------------+
                  |    Module Schema & Code        |
                  |  (e.g., commerce/product)      |
                  +---------------+----------------+
                                  |
                                  | hyperrr build
                                  v
+------------------+     +------------------+     +-------------------+
|  schema_gen/     |     |   generated.go   |     | resolvers_impl.go |
| (Copied Schemas) |     |  (GQLGen Engine) |     | (Auto-Delegator)  |
+--------+---------+     +--------+---------+     +---------+---------+
         |                        |                         |
         +------------------------+-------------------------+
                                  |
                                  v
                  +--------------------------------+
                  |       Compiled Monolith        |
                  |         (bin/hyperrr)          |
                  +--------------------------------+
```

---

## 2. Implementing the `GraphQLProvider` Interface

Modules that expose GraphQL queries, mutations, or field resolvers must implement the `registry.GraphQLProvider` interface defined in [module.go](file:///D:/hyperrr-commerce-ai/hyperrr/pkg/registry/module.go):

```go
type GraphQLProvider interface {
	Module
	Queries() map[string]any
	Mutations() map[string]any
	FieldResolvers() map[string]any
}
```

### Methods Explanation
*   `Queries()`: Maps GraphQL Query resolver names (defined in your schema) to their respective handler functions on the module.
*   `Mutations()`: Maps GraphQL Mutation resolver names to their respective handler functions on the module.
*   `FieldResolvers()`: Maps nested model field resolvers (e.g. `WorkflowLineage.events`) using a dot-separated string syntax (e.g. `"WorkflowLineage.events": m.Events`).

### Example Implementation (`commerce/product/graphql.go`)
```go
package product

import (
	"context"
	"github.com/GoHyperrr/hyperrr/api/graph/model"
)

// Ensure Module implements registry.GraphQLProvider at compile time
var _ registry.GraphQLProvider = (*Module)(nil)

func (m *Module) Queries() map[string]any {
	return map[string]any{
		"getProduct":   m.GetProduct,
		"listProducts": m.ListProducts,
	}
}

func (m *Module) Mutations() map[string]any {
	return map[string]any{
		"createProduct": m.CreateProduct,
		"updateProduct": m.UpdateProduct,
		"deleteProduct": m.DeleteProduct,
	}
}

func (m *Module) FieldResolvers() map[string]any {
	return nil // No custom field resolvers for Product model
}
```

---

## 3. Dynamic Build & Code-Generation Pipeline

When you run `go run ./cmd/hyperrr build` (or `make build`), the build orchestrator executing [build.go](../cmd/hyperrr/build.go) performs the following steps:

### Step 1: Schema Aggregation
1. Cleans up the target `api/graph/schema_gen` cache directory.
2. Scans configured module paths (e.g. `../commerce`, `../auth`, `pkg`) for `.graphqls` files.
3. Copies all discovered schemas into module-separated subdirectories under `api/graph/schema_gen/` (e.g., `api/graph/schema_gen/commerce/product/product.graphqls`).

### Step 2: GQLGen Execution
Invokes GQLGen via `gqlgen generate` to parse the aggregated schemas and write the type-safe engine framework inside `api/graph/generated.go` and `api/graph/model/models_gen.go`.

### Step 3: Custom Delegation Codegen
Executes [codegen.go](../cmd/hyperrr/codegen.go):
1. **Module Scanning**: Leverages Go AST parsing to scan the packages, checking which ones implement `GraphQLProvider`.
2. **Interface Parsing**: Parses the `MutationResolver`, `QueryResolver`, and other custom type resolver interfaces generated inside GQLGen's `generated.go`.
3. **Case-Insensitive Mapping**: Statically matches GQLGen's resolver method names against the keys returned by `Queries()`, `Mutations()`, and `FieldResolvers()` case-insensitively.
4. **Writes Glue Code**:
    *   **`resolver.go`**: Contains the main `Resolver` struct with typed module fields and the `NewResolver` constructor.
    *   **`resolvers_impl.go`**: Implements the resolver methods, checking if the module is loaded and delegating executions directly:
        ```go
        func (r *mutationResolver) CreateProduct(ctx context.Context, input model.CreateProductInput) (*model.Product, error) {
        	if r.ProductModule == nil {
        		return nil, fmt.Errorf("module product not loaded")
        	}
        	return r.ProductModule.CreateProduct(ctx, input)
        }
        ```

---

## 4. Run-Time Injection

At startup, the bootstrapper in [app.go](../internal/app/app.go) retrieves the global list of active modules, instantiates the resolver dynamically, and serves the HTTP handler:

```go
resolver := graph.NewResolver(
	registry.List(),
	app.runner,
	app.registryStore,
	ctxMod.Projector(),
)
```

No hardcoded switch-cases or manual imports are required.

---

## 5. Security & Authentication Middleware

GraphQL requests pass through the security middleware ([auth.go](../api/middleware/auth.go)) which is dynamically registered by the `auth.emailpass` module (under the `"auth"` key in the registry). The middleware:

1. Validates the `Authorization: Bearer <JWT>` header using the secret configured locally inside the `auth.emailpass` module options.
2. Injects the verified `Actor` object directly into the Go `context.Context`.
3. Dispatches the request to the dynamic resolvers, enabling easy authorization checks via `ctx.Value`.
