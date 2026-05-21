# Tasks for #11: Commerce Plugin: Customer & ML Segmentation

Parent issue: #11
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Customer Domain Model & Repository ✅
**Status**: Completed  
**Output**: `commerce/customer/model.go` and `commerce/customer/repository.go`.  
**Implementation**: Defined `Customer` and `Address` models with GORM. The `Customer` model includes a `Persona` field for ML insights.

### 2. Implement Customer Module as a Plugin ✅
**Status**: Completed  
**Output**: `commerce/customer/module.go`.  
**Implementation**: Registered `commerce.customer` with the OS. Wired the repository and models for automatic initialization.

### 3. Define Customer GraphQL Schema ✅
**Status**: Completed  
**Output**: `commerce/customer/customer.graphqls`.  
**Implementation**: Added Customer and Address types to the pluggable schema. Implemented the `getCustomer` resolver in `api/graph/customer.resolvers.go`.

### 4. Implement ML Persona Segmentation Workflow ✅
**Status**: Completed  
**Output**: `commerce/customer/workflows/segmentation.yaml` and handlers in `handlers.go`.  
**Implementation**: Developed a 2-step workflow (`calculate_persona`, `update_persona`). The calculation step uses mock ML logic to assign personas based on transaction value.

### 5. Wire Event-Triggered Segmentation ✅
**Status**: Completed  
**Implementation**: Updated the module's `Init` method to subscribe to `order.completed` events. The module now automatically triggers the segmentation workflow asynchronously using the injected OS Runner.

### 6. Exhaustive Testing for Customer & ML Logic ✅
**Status**: Completed  
**Output**: `commerce/customer/customer_test.go`.  
**Implementation**: Verified the full end-to-end flow: from order event arrival to autonomous persona update in the database.
