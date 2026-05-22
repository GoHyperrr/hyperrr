# Tasks for #22: Technical Debt & Architectural Refinement

Parent issue: #22
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Implement Workflow Registry & Store ✅
**Status**: Completed  
**Implementation**: Created `internal/workflow/registry.go`. Modules now register DAGs in `Init`. Resolvers retrieve workflows by name.

### 2. Implement Transactional Outbox ✅
**Status**: Completed  
**Implementation**: Created `pkg/db/outbox.go` and `OutboxEvent` model. Added `SaveToOutbox` helper to DB struct.

### 3. ML Brain v2: Context-Aware Segmentation ✅
**Status**: Completed  
**Implementation**: Created `commerce/customer/ml_brain.go`. Upgraded `customer.calculate_persona` to analyze real execution lineages from the Context Engine.

### 4. JWT Refresh & Token Blacklisting ✅
**Status**: Completed  
**Implementation**: Created `internal/auth/store.go` with `Blacklist` support. Updated `ValidateToken` to enforce revocation checks.

### 5. Dependency Injection Refactoring ✅
**Status**: Completed  
**Implementation**: Standardized `registry.Dependencies` to include `Registry` and `Runner`.

### 6. Fix Goroutine Leak in Workflow Runner ✅
**Status**: Completed  
**Implementation**: Updated `Runner.ResumeWorkflow` to use `select` with a 5s timeout.

### 7. Secure JWT Secret Handling ✅
**Status**: Completed  
**Implementation**: Signing key is now injected from `config.Config`.

### 8. Enforce Safe Type Coercion in Handlers ✅
**Status**: Completed  
**Implementation**: Fixed all `commerce/*/handlers.go` to safely check types and handle `float64` vs `int` mismatches.

### 9. Secure Auth Middleware Error Handling ✅
**Status**: Completed  
**Implementation**: Middleware now returns 401 Unauthorized on invalid tokens.

### 10. Remove Resolver Panics ✅
**Status**: Completed  
**Implementation**: Replaced mock `panic` calls with proper `error` returns in `schema.resolvers.go`.
