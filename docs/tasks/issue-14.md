# Tasks for #14: Commerce Plugin: Orders & Fulfillment Sagas

Parent issue: #14
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Order Domain Model & Repository ✅
**Status**: Completed  
**Implementation**: Defined `Order` and `OrderItem` models with GORM. Statuses include `PENDING`, `PAID`, `FULFILLED`, `CANCELLED`.

### 2. Implement Order Module as a Plugin ✅
**Status**: Completed  
**Implementation**: Registered `commerce.order` with the OS. Wired the repository and task handlers for automatic discovery.

### 3. Implement Fulfillment Saga Handlers ✅
**Status**: Completed  
**Implementation**: Developed declarative task handlers for the fulfillment saga: `order.create`, `order.process_payment`, `order.finalize`, and `order.compensate_payment`.

### 4. Define Order GraphQL Schema ✅
**Status**: Completed  
**Implementation**: Added Order types and the `createOrderFromCart` mutation to the pluggable schema.

### 5. Wire Cart Checkout to Fulfillment Saga ✅
**Status**: Completed  
**Implementation**: Updated the API layer to trigger the `fulfillment.v1` saga when creating an order from a cart. The saga handles the transition from intent to a paid commerce transaction.

### 6. Exhaustive Testing for Fulfillment Sagas ✅
**Status**: Completed  
**Implementation**: Verified the full end-to-end fulfillment lifecycle, including success paths and saga compensation for payment failures. Reached 90%+ project logic coverage.
