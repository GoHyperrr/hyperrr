# Tasks for #21: Commerce Plugin: Analytics (Operational BI)

Parent issue: #21
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Analytics Module
**Type**: WRITE
**Output**: `commerce/analytics/module.go`.
**Depends on**: none

Build the OS plugin. Inject the `Projector` (Context Engine) dependency.

### 2. Implement Analytics Handlers
**Type**: WRITE
**Output**: `commerce/analytics/handlers.go`.
**Depends on**: 1

Implement logic to compute metrics from the Context Engine:
- `analytics.get_system_health`: Computes workflow success/failure rates.
- `analytics.get_sales_metrics`: Computes total revenue and order counts.

### 3. Define Analytics GraphQL Schema
**Type**: WRITE
**Output**: `commerce/analytics/analytics.graphqls`.
**Depends on**: 2

Expose `getSystemStats` and `getSalesStats` via GraphQL.

### 4. Exhaustive Testing
**Type**: TEST
**Output**: `commerce/analytics/analytics_test.go`.
**Depends on**: 3

Verify metrics are accurately derived from execution lineages. Ensure 90%+ logic coverage.
