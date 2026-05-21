# Implementation Issues: hyperrr — AI-Observable Commerce OS

## Status Legend
- ✅ **Completed**
- 🚧 **In Progress**
- ⏳ **Backlog**

---

## ✅ Issue 1: Project Scaffolding & Industry-Standard DX Tooling
**Status**: Completed
**Type**: HITL
**Blocked by**: None

### Parent PRD
`docs/PRD_CommerceOS.md`

### Achievements
- Initialized Go module `github.com/GoHyperrr/hyperrr`.
- Established standard directory structure (`/internal`, `/pkg`, `/cmd`, `/api`).
- Configured `golangci-lint` with strict industry rules.
- Implemented `Makefile` for automation (setup, lint, test, coverage, build).
- Built a custom Go-based coverage enforcement tool (`tools/coverage`) ensuring 93%+ coverage.
- Configured environment management with **Viper**.
- Established Git workflow with **Lefthook** pre-commit hooks and `CONTRIBUTING.md`.
- Implemented a centralized **Structured Logging** system (`pkg/logger`) using `slog`.

---

## ✅ Issue 2: Modular Database Abstraction & GORM Setup
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 1

### Achievements
- Integrated **GORM** for database-agnostic persistence.
- Implemented a pure-Go SQLite driver for portable development.
- Built a global `Registry` pattern for modular migrations (`pkg/db`).
- Established the "Soft Relationship" pattern (no explicit foreign keys) to ensure module isolation.
- Fully tested connection and migration logic.

---

## ✅ Issue 3: Event Fabric Interface & In-Memory Provider
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 1

### Achievements
- Defined the core `Event` and `EventBus` interfaces in `pkg/eventbus`.
- Implemented a thread-safe `InMemBus` for local async messaging.
- Ensured at-least-once delivery semantics logically via Go channels.

---

## ✅ Issue 4: Workflow Engine: DSL Parser & Basic Execution
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 3

### Achievements
- Defined a declarative YAML DSL for commerce workflows.
- Implemented a robust DAG parser with cycle detection and dependency validation.
- Built the `Runner` that executes steps in topological order.
- Integrated the runner with the `EventBus` to emit real-time lifecycle events.

---

## ✅ Issue 5: Policy-Driven Failure Orchestration & Sagas
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 4

### Achievements
- Extended the DSL and Runner to support **Retry Policies** (exponential backoff).
- Implemented **Fallback Strategies** to recover from terminal step failures.
- Built the **Saga Pattern** (Compensation) for reversible distributed transactions.
- Fully tested complex failure and compensation paths.

---

## ✅ Issue 6: Human-In-The-Loop: WAITING_HUMAN & TUI Shell
**Status**: Completed
**Type**: HITL
**Blocked by**: Issue 5

### Achievements
- Implemented the `WAITING_HUMAN` state in the Workflow Engine.
- Built the `ResumeWorkflow` API to allow manual intervention (retry/cancel).
- Developed a rich **TUI Mission Control** using `bubbletea` and `lipgloss` to visualize workflow states.
- Verified the end-to-end "Pause -> Intervene -> Resume" flow.

---

## ⏳ Issue 7: Context Engine: Execution Lineage GraphQL API
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 4

### Parent PRD
`docs/PRD_CommerceOS.md`

### What to build
Implement the Context Engine service which aggregates events into a queryable knowledge graph.
- Build a projection that listens to all workflow events and stores execution history.
- Expose a GraphQL API (`/internal/context`) using `gqlgen` to query the lineage of workflow executions.
- Include basic entity correlation (e.g., linking events by `order_id`).

---

## ⏳ Issue 8: Catalog: Product Lifecycle & Discoverable Modules
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 4

### What to build
Implement the `/internal/catalog` module using the discoverable plugin pattern.
- Implement the `Module` interface and register with the core system.
- Define `product.create.v1` and `product.update.v1` workflows.
- Ensure all mutations are driven by the Workflow Engine.

---

## ⏳ Issue 9: Inventory: Reservation & Multi-Warehouse Strategy
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 5

---

## ⏳ Issue 10: Orders: Fulfillment & Payment Orchestration (Saga)
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 5

---

## ⏳ Issue 11: Identity & CRM: ML-Driven Customer Segmentation
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 3

---

## ⏳ Issue 12: Search: Event-Driven Indexing (Postgres FTS)
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 3
