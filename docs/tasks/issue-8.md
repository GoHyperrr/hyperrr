# Tasks for #8: Catalog: Product Lifecycle & Discoverable Modules

Parent issue: #8
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Implement Module Registry & Contract Interface
**Type**: WRITE
**Output**: `pkg/registry/registry.go` defining the `Module` interface and a global `Registry`.
**Depends on**: Issue 3 completion

Create a standardized interface that all modules must implement to be "discoverable." This should include methods like `GetWorkflows()`, `GetHandlers()`, and `GetSchema()`. This allows the Workflow Engine and AI Context Engine to dynamically discover and interact with module capabilities.

### 2. Define Catalog Domain Model & Repository
**Type**: WRITE
**Output**: `internal/catalog/model.go` and a repository implementation for product storage.
**Depends on**: 1

Implement the internal data structures for the Catalog. Ensure the model supports the metadata required for event-sourcing and auditability as defined in the PRD.

### 3. Implement Catalog Module as a Plugin
**Type**: WRITE
**Output**: `internal/catalog/module.go` implementing the `Registry.Module` interface.
**Depends on**: 2

Register the Catalog module with the core system. Expose its `product.create.v1` and `product.update.v1` workflows and the associated Go step-handlers (e.g., `ValidateProduct`, `PersistProduct`) to the Registry.

### 4. Wire Registry to Workflow Engine
**Type**: WRITE
**Output**: Workflow Engine updated to resolve `uses: catalog.create` from the Registry.
**Depends on**: 3 and Issue 3

Update the Workflow Engine's runner to look up step-handlers dynamically from the `ModuleRegistry` instead of using hardcoded maps. This fulfills the "discoverable" requirement and allows AI to reason over available system capabilities.

### 5. Exhaustive Testing for Module Discovery & Catalog Workflows
**Type**: TEST
**Output**: `internal/catalog/plugin_test.go` verifying discovery and execution with >95% coverage.
**Depends on**: 4

Write integration tests that assert the Catalog module correctly registers its capabilities and that the Workflow Engine can successfully discover and execute a `product.create` workflow end-to-end. Ensure 95%+ coverage for the entire module.
