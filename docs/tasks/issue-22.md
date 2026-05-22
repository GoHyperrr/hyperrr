# Tasks for #22: Technical Debt & Architectural Refinement

Parent issue: #22
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Implement Workflow Registry & Store
**Type**: WRITE
**Output**: `internal/workflow/registry.go` and `internal/workflow/model.go`.
**Depends on**: none

Create a central registry where modules can register their DAGs by name. Implement a database-backed store for these definitions to avoid defining workflows inline in resolvers.

### 2. Implement Transactional Outbox
**Type**: WRITE
**Output**: `pkg/db/outbox.go` and updated `pkg/eventbus`.
**Depends on**: 1

Ensure all events are persisted in an `outbox` table within the same transaction as the business state change. A background worker should then publish these events to the Event Fabric.

### 3. ML Brain v2: Context-Aware Segmentation
**Type**: WRITE
**Output**: `commerce/customer/ml_brain.go`.
**Depends on**: 2

Replace the mock persona logic. The new handler should query the `Context Engine` to analyze the customer's full lineage graph (orders, returns, support calls) before assigning a persona.

### 4. JWT Refresh & Token Blacklisting
**Type**: WRITE
**Output**: `internal/auth/refresh.go` and `internal/auth/blacklist.go`.
**Depends on**: 2

Implement token refresh rotations. Add a database table to track revoked tokens, ensuring sessions can be terminated immediately (e.g., on password change).

### 5. Dependency Injection Refactoring
**Type**: REFACTOR
**Output**: Updated `pkg/registry` and module `Init` methods.
**Depends on**: 4

Standardize how common utilities (Logger, Config, DB) are injected into modules to reduce boilerplate and improve testability.
