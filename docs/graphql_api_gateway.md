# GraphQL API Gateway and Security Middleware

Hyperrr uses a modular GraphQL gateway (`api/graph`) built using the `gqlgen` code-generation library. The gateway stitches schemas and resolvers from independent modules together, handles authentication via JWT middleware, and enforces actor security context across domains.

---

## 1. Modular Schema Co-Location

To keep domain logic decoupled, Hyperrr does not write a single large schema file. Instead, each module defines its own `.graphqls` schema file inside its own directory:
- Product schema: `commerce/product/product.graphqls`
- Order schema: `commerce/order/order.graphqls`
- System context schema: `internal/context/context.graphqls`

### The Compilation Configuration
At compile-time, `gqlgen` reads `api/gqlgen.yml`, aggregates all scattered schema files, and generates a single unified execution entry point (`api/graph/generated.go`):

```yaml
schema:
  - graph/*.graphqls
  - ../internal/**/*.graphqls
  - ../commerce/**/*.graphqls
```

---

## 2. The Unified Resolver Container

Although schemas are compile-time merged, resolution is dynamically bound at boot. The main `Resolver` struct (`api/graph/resolver.go`) contains pointers to all module instances:

```go
type Resolver struct {
	Projector      context.Projector
	ProductModule  *product.Module
	CustomerModule *customer.Module
	CartModule     *cart.Module
	OrderModule    *order.Module
	FinanceModule  *finance.Module
	...
}
```

During startup (`internal/app/app.go`), the bootstrapper queries the global `registry` list, retrieves the instantiated module pointers, and injects them into the GraphQL handler container.

---

## 3. JWT Security and Actor Middleware

Every query or mutation going through the gateway is interceptable by the security middleware (`api/middleware/auth.go`).

```
+---------+               +------------+               +-----------------+               +-----------+
| Client  |  -- token --> | Middleware |  -- query --> |  ActorResolver  |  -- actor --> | GraphQL   |
|         |               |  (Auth)    |               | (modules/ident) |               | Context   |
+---------+               +------------+               +-----------------+               +-----------+
```

### Context Injection Workflow
1. **Extraction**: The middleware parses the incoming `Authorization: Bearer <token>` HTTP header.
2. **Signature Validation**: It validates the token's cryptographic signature against the `JWT_SECRET` key.
3. **Actor Lookup**: If the signature is valid, it retrieves the `actor_id` from the JWT claims and calls the `ActorResolver` module:
   ```go
   actor, err := resolver.GetActorByID(ctx, actorID)
   ```
4. **Context Wrapping**: It wraps the resulting `identity.Actor` struct inside the request context:
   ```go
   ctx = context.WithValue(ctx, UserContextKey, actor)
   ```
5. **Resolver Enforcement**: Within any GraphQL resolver, the code retrieves the actor metadata to enforce authorization and access policies:
   ```go
   actor, ok := ctx.Value(UserContextKey).(*identity.Actor)
   ```

---

## 4. Safe Entity Type Mapping

Because dynamic workflow state persistence engines return generic `map[string]any` maps upon resumption, GraphQL resolvers require a safe mapping utility to convert un-typed maps back to concrete schema structs.

Hyperrr uses a custom decode mapper (`api/graph/mapper.go`):
```go
func decodeResult(src any, dest any) error {
	config := &mapstructure.DecoderConfig{
		TagName:          "json",
		Result:           dest,
		WeaklyTypedInput: true,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}
	return decoder.Decode(src)
}
```
* **Panic Prevention**: This decoder replaces unsafe direct type assertions (e.g. `src.(order.Order)`) that would cause runtime crashes if the data structure was serialized/deserialized through the NATS or Redis database.
* **Weak Typing Tolerance**: It handles conversions between numeric types (like GORM's `float64` and JSON-RPC's `int` formats) safely.
