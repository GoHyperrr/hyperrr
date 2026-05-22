# Tasks for #16: Commerce Plugin: Fulfillment (Logistics & Tracking)

Parent issue: #16
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Fulfillment Domain Model & Repository
**Type**: WRITE
**Output**: `commerce/fulfillment/model.go` and `commerce/fulfillment/repository.go`.
**Depends on**: none

Implement models for `Inventory` (ID, ProductID, AvailableQuantity) and `Shipment` (ID, OrderID, Status: PENDING, SHIPPED, DELIVERED, TrackingNumber).

### 2. Implement Fulfillment Module as a Plugin
**Type**: WRITE
**Output**: `commerce/fulfillment/module.go` implementing the `registry.Module` interface.
**Depends on**: 1

Register the `commerce.fulfillment` module. Wire up the repository and register it in `internal/app`.

### 3. Implement Fulfillment Workflow Handlers
**Type**: WRITE
**Output**: `commerce/fulfillment/handlers.go`.
**Depends on**: 2

Implement task handlers:
- `fulfillment.reserve_inventory`: Checks and decrements inventory for order items.
- `fulfillment.release_inventory`: Compensates `reserve_inventory` by incrementing inventory back.
- `fulfillment.create_shipment`: Creates a `Shipment` record in `PENDING` state.

### 4. Define Fulfillment GraphQL Schema
**Type**: WRITE
**Output**: `commerce/fulfillment/fulfillment.graphqls`.
**Depends on**: 2

Expose the domain via GraphQL. Include queries for `getInventory`, `getShipment` and mutation to update shipping status `updateShipmentStatus`.

### 5. Wire Order Saga to Fulfillment
**Type**: WRITE
**Output**: `api/graph/order.resolvers.go` and `commerce/order/order_test.go` updated.
**Depends on**: 3

Update the checkout flow. The order fulfillment saga should now include `fulfillment.reserve_inventory` (before payment) and `fulfillment.create_shipment` (after payment).

### 6. Exhaustive Testing for Fulfillment Logic
**Type**: TEST
**Output**: `commerce/fulfillment/fulfillment_test.go`.
**Depends on**: 5

Verify the full inventory reservation, payment, and shipment creation logic. Verify compensation logic (release inventory if payment fails).
