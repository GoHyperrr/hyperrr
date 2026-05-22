# Tasks for #20: Commerce Plugin: Search (Semantic & Vector)

Parent issue: #20
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Search Module & Model
**Type**: WRITE
**Output**: `commerce/search/model.go` and `commerce/search/module.go`.
**Depends on**: none

Implement a `SearchQuery` model to track search history (optional audit). Build the OS plugin.

### 2. Implement Mock Vector Search Engine
**Type**: WRITE
**Output**: `commerce/search/engine.go`.
**Depends on**: 1

Implement a mock "Semantic Search" engine that simulates vector embeddings. It should allow searching the `Product` repository via keywords that represent "embeddings" in a simplified way for testing.

### 3. Implement Search Workflow Handlers
**Type**: WRITE
**Output**: `commerce/search/handlers.go`.
**Depends on**: 2

Implement `search.product_catalog` task handler. This handler will perform the search and emit a `search.performed` event to the Event Fabric.

### 4. Define Search GraphQL Schema
**Type**: WRITE
**Output**: `commerce/search/search.graphqls`.
**Depends on**: 2

Expose `searchProducts(query: String!, limit: Int): [Product!]!` via GraphQL.

### 5. Exhaustive Testing
**Type**: TEST
**Output**: `commerce/search/search_test.go`.
**Depends on**: 4

Verify search results return relevant products. Ensure 90%+ logic coverage.
