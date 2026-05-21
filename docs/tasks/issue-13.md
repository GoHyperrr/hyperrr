# Tasks for #13: Commerce Plugin: Cart & Checkout

Parent issue: #13
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Cart Domain Model & Repository ✅
**Status**: Completed  
**Implementation**: Defined `Cart` and `CartItem` models with GORM. The `Cart` includes a `Status` (ACTIVE, COMPLETED, ABANDONED) to track shopping lifecycles.

### 2. Implement Cart Module as a Plugin ✅
**Status**: Completed  
**Implementation**: Registered `commerce.cart` with the OS. Wired the repository and task handlers (`cart.add_item`, `cart.remove_item`, `cart.checkout`) for automatic discovery.

### 3. Define Cart GraphQL Schema ✅
**Status**: Completed  
**Implementation**: Added Cart and CartItem types to the pluggable schema. Exposed mutations for adding/removing items and checking out.

### 4. Implement Cart Workflows & Handlers ✅
**Status**: Completed  
**Implementation**: Developed declarative workflows for all state-mutating operations. The `AddItem` handler now manages quantity increments for existing products autonomously.

### 5. Wire Checkout to Order Intent ✅
**Status**: Completed  
**Implementation**: The `Checkout` workflow transitions the cart to `COMPLETED` and is ready to trigger downstream fulfillment sagas in the Orders module.

### 6. Exhaustive Testing for Cart Logic ✅
**Status**: Completed  
**Implementation**: Verified full end-to-end flow: from creating an active cart and adding items via GraphQL to finalizing the checkout through the workflow engine.
