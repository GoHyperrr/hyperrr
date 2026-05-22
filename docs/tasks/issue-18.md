# Tasks for #18: Commerce Plugin: Marketing & Loyalty

Parent issue: #18
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Marketing Domain Model & Repository
**Type**: WRITE
**Output**: `commerce/marketing/model.go` and `commerce/marketing/repository.go`.
**Depends on**: none

Implement models for `Coupon` (ID, Code, DiscountPercentage, Active) and `LoyaltyPoints` (ID, CustomerID, Balance).

### 2. Implement Marketing Module as a Plugin
**Type**: WRITE
**Output**: `commerce/marketing/module.go` implementing the `registry.Module` interface.
**Depends on**: 1

Register the `commerce.marketing` module. Wire up the repository and register it in `internal/app`.

### 3. Implement Marketing Workflow Handlers
**Type**: WRITE
**Output**: `commerce/marketing/handlers.go`.
**Depends on**: 2

Implement task handlers:
- `marketing.validate_coupon`: Checks if a coupon code is valid.
- `marketing.apply_discount`: Calculates the discount amount based on a coupon.
- `marketing.add_loyalty_points`: Increments a customer's loyalty point balance after a successful order.

### 4. Define Marketing GraphQL Schema
**Type**: WRITE
**Output**: `commerce/marketing/marketing.graphqls`.
**Depends on**: 2

Expose the domain via GraphQL. Include queries for `getCoupon`, `getLoyaltyBalance` and mutation `applyCouponToCart`.

### 5. Wire Order Saga to Marketing
**Type**: WRITE
**Output**: `api/graph/order.resolvers.go` and `commerce/order/order_test.go` updated.
**Depends on**: 3

Update the fulfillment saga to include `marketing.add_loyalty_points` as the final successful step.

### 6. Exhaustive Testing
**Type**: TEST
**Output**: `commerce/marketing/marketing_test.go`.
**Depends on**: 4

Verify coupon validation, discount application, and loyalty point accrual. Re-achieve 90%+ project logic coverage.
