# Tasks for #15: Commerce Plugin: Finance (Tax & Payments)

Parent issue: #15
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Finance Domain Model & Repository ✅
**Status**: Completed  
**Implementation**: Defined `Payment` model with GORM. Created repository with standard CRUD methods.

### 2. Implement Finance Module as a Plugin ✅
**Status**: Completed  
**Implementation**: Built `commerce/finance` as an OS plugin, wired it to `internal/app`.

### 3. Implement Finance Workflow Handlers ✅
**Status**: Completed  
**Implementation**: Built `finance.process_payment` and `finance.compensate_payment` to manage the lifecycle of a payment in standard or failure paths.

### 4. Define Finance GraphQL Schema ✅
**Status**: Completed  
**Implementation**: Created `commerce/finance/finance.graphqls` exposing `Payment` and queries. Regenerated API layer.

### 5. Wire Order Fulfillment Saga to Finance ✅
**Status**: Completed  
**Implementation**: Integrated Finance handlers into the `fulfillment.v1` saga within the Orders module, removing mock implementations.

### 6. Exhaustive Testing for Finance Logic ✅
**Status**: Completed  
**Implementation**: Added comprehensive integration tests in `commerce/finance/finance_test.go` checking normal and compensation saga flows.
