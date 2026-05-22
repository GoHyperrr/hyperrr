# Tasks for #23: Audit Findings: Code Review & 95% Coverage Mandate

Parent issue: #23
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Reach 95% Logic Coverage
**Type**: TEST
**Output**: 95%+ coverage in `coverage.out`.
**Depends on**: none

Add exhaustive unit and integration tests focusing on:
- `internal/context/projection.go`: All edge cases in event handling.
- `internal/workflow/runner.go`: Compensation and failure path logic.
- `api/graph`: Error scenarios for all resolvers.

### 2. Formalize Workflow State Machine
**Type**: REFACTOR
**Output**: Updated `internal/workflow/runner.go`.
**Depends on**: 1

Implement an internal state machine to manage workflow executions. Define valid transitions (e.g., `RUNNING` -> `WAITING_HUMAN`, `WAITING_HUMAN` -> `RUNNING`) to prevent race conditions during concurrent signals.

### 3. Implement Unified Constants & Enums
**Type**: REFACTOR
**Output**: `pkg/constants/constants.go`.
**Depends on**: 2

Replace all magic string literals for statuses and types across the entire codebase with typed constants.

### 4. Enable Soft-Delete Across Modules
**Type**: WRITE
**Output**: Updated models in Product, Customer, and Cart modules.
**Depends on**: 3

Update all GORM models to include `DeletedAt`. Refactor repository `Delete` methods to ensure they leverage GORM's soft-delete functionality to preserve audit trails.

### 5. Harden GraphQL Error Modeling
**Type**: WRITE
**Output**: `pkg/errors` package and updated resolvers.
**Depends on**: 4

Create a unified error handling strategy. Replace `panic` calls in resolvers with structured errors that provide clear feedback and error codes to API consumers.
