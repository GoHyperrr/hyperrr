# Tasks for #7: Context Engine: Execution Lineage GraphQL API

Parent issue: #7
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Initialize Context Engine Projection Logic
**Type**: WRITE
**Output**: `internal/context/projection.go` subscribing to workflow events.
**Depends on**: Issue 3 completion

Implement a background service in the Context Engine that subscribes to all `workflow.*` events. It must build a local, queryable state (e.g., in-memory or SQLite) that reconstructs the execution lineage of every workflow instance.

### 2. Configure `gqlgen` and Define Schema
**Type**: CONFIG/WRITE
**Output**: `gqlgen.yml`, `graph/schema.graphqls`, and generated boilerplate.
**Depends on**: 1

Initialize `gqlgen` in the `internal/context` directory. Define the GraphQL schema focusing on `WorkflowLineage`, `StepHistory`, and `EventNode`. Ensure the schema supports traversing the "causality chain" (e.g., which event triggered which workflow).

### 3. Implement GraphQL Resolvers for Lineage
**Type**: WRITE
**Output**: `graph/schema.resolvers.go` implementing the data retrieval logic.
**Depends on**: 2

Implement the resolvers to fetch lineage data from the projected state. The primary query should be `getWorkflowLineage(id: ID!)`, returning the full DAG of steps and their associated events/artifacts.

### 4. Implement Entity Correlation & Graph Traversal
**Type**: WRITE
**Output**: Logic to correlate disparate workflows via shared IDs (e.g., `order_id`).
**Depends on**: 3

Enhance the projection and resolvers to support cross-workflow correlation. If multiple workflows share a common metadata key (like an Order ID), the Context Engine should allow the AI or Operator to traverse from one workflow's execution graph to another.

### 5. Exhaustive Testing for Context API & Projections
**Type**: TEST
**Output**: `internal/context/graph_test.go` with >95% coverage.
**Depends on**: 4

Write comprehensive tests for the GraphQL API. Mock the `EventBus` to feed in complex, multi-workflow event streams, then execute GraphQL queries to assert that the returned lineage graph and entity correlations are 100% accurate.
