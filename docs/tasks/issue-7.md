# Tasks for #7: Context Engine: Execution Lineage GraphQL API

Parent issue: #7
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Initialize Context Engine Projection Logic ✅
**Status**: Completed  
**Implementation**: Built a background `Projector` that subscribes to workflow events and maintains an in-memory lineage of every execution.

### 2. Configure `gqlgen` and Define Schema ✅
**Status**: Completed  
**Output**: `graph/schema.graphqls` and `gqlgen.yml`.  
**Implementation**: Configured `gqlgen` with explicit model generation (excluding internal business logic structs to avoid binding conflicts). Defined a schema for lineages, steps, and events.

### 3. Implement GraphQL Resolvers for Lineage ✅
**Status**: Completed  
**Implementation**: Developed resolvers in `schema.resolvers.go` that fetch projected state. Added a `mapper.go` to safely translate internal domain objects to GraphQL models.

### 4. Implement Entity Correlation & Graph Traversal ✅
**Status**: Completed  
**Implementation**: Enhanced the Projector to index events by metadata. Added `relatedLineages` field and resolver to allow traversing the causal graph of disparate workflows (e.g., all workflows related to a specific `order_id`).

### 5. Exhaustive Testing for Context API & Projections ✅
**Status**: Completed  
**Implementation**: Achieved high logic coverage by testing the Projector, Mapper, and Resolvers. Integrated cross-package coverage reporting in the `Makefile`.
