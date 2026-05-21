# Tasks for #11: Commerce Plugin: Customer & ML Segmentation

Parent issue: #11
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Customer Domain Model & Repository ✅
**Status**: Completed  
**Implementation**: Defined `Customer` and `Address` models with GORM. The `Customer` model includes a `Persona` field for ML insights.

### 2. Implement Customer Module as a Plugin ✅
**Status**: Completed  
**Implementation**: Registered `commerce.customer` with the OS. Wired the repository and registered task handlers (`calculate_persona`, `update_persona`, `update_details`).

### 3. Define Customer GraphQL Schema ✅
**Status**: Completed  
**Implementation**: Added Customer types and `updateCustomer` mutation to the pluggable schema.

### 4. Implement ML Persona Segmentation Workflow ✅
**Status**: Completed  
**Implementation**: Developed a 2-step background workflow that autonomously assigns customer personas based on transaction value.

### 5. Wire Event-Driven Consistency ✅
**Status**: Completed  
**Implementation**: Updated the module to listen for `identity.user_created` events to automatically create business profiles, and `order.completed` to trigger segmentation.

### 6. Exhaustive Testing ✅
**Status**: Completed  
**Implementation**: Verified the full end-to-end flow from event arrival and GraphQL mutation to workflow execution and DB persistence.
