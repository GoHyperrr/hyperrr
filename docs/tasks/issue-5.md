# Tasks for #5: Policy-Driven Failure Orchestration & Sagas

Parent issue: #5
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Extend DSL for Policies ✅
**Status**: Completed  
**Output**: `internal/workflow/dsl.go`.  
**Implementation**: Added `Retry`, `Fallback`, and `Saga` structs to the Step definition.

### 2. Implement Retry Logic in Runner ✅
**Status**: Completed  
**Implementation**: Added retry loop with support for constant and exponential backoff strategies.

### 3. Implement Fallback Logic in Runner ✅
**Status**: Completed  
**Implementation**: Enabled the engine to transition to an alternative step if the primary step fails terminally.

### 4. Implement Compensation (Saga) Logic ✅
**Status**: Completed  
**Implementation**: Built a backward-traversing compensation engine. When a workflow fails, it automatically executes defined saga steps for all completed nodes in reverse order.

### 5. Update State Machine and Events for Failures & Sagas ✅
**Status**: Completed  
**Implementation**: Added `RETRYING`, `FAILED`, and `COMPENSATING` states. Runner now emits granular failure events.

### 6. Test Failure Orchestration (Retry & Fallback) ✅
**Status**: Completed  
**Output**: `internal/workflow/failure_test.go`.  
**Implementation**: Verified retries with backoff and successful fallback transitions.

### 7. Test Compensation (Saga) Logic ✅
**Status**: Completed  
**Output**: `internal/workflow/saga_test.go`.  
**Implementation**: Verified full and partial compensation paths, including reverse execution order.
