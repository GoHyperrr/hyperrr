# Issues: Stateful Modular Monolith Core Refactor

This document breaks down the transition from an ephemeral, DB-heavy core to a durable, high-speed stateful engine as defined in `docs/PRD_Pluggable_Workflow_Engine.md`.

---

## Issue 1: Core Deconstruction & DB Cleanup

**Type**: AFK
**Blocked by**: None

### Parent PRD

`docs/PRD_Pluggable_Workflow_Engine.md`

### What to build

Strip out the "distributed microservice" architecture and heavy relational database logging. We are moving away from the "Transactional Outbox in SQL" and "Execution Lineage in SQL" patterns to favor a high-speed KV-based workflow engine.

- Delete `pkg/db/outbox.go` and the `OutboxEvent` GORM model.
- Remove `LineageModel` and all relational DB logic from `internal/context`.
- Simplify `registry.Module` and `registry.Dependencies` to remove observability-persistence requirements.
- Clean up any lingering references to "outbox" in the commerce modules.

### How to verify

- **Automated**: `make test` passes after deleting the files (tests for outbox and old projector should be removed or mocked).
- **Manual**: Verify `internal/context` no longer creates tables in the SQL database.

### Acceptance criteria

- [ ] All `outbox_events` and `workflow_lineages` tables are removed from the migration list.
- [ ] Primary SQL database contains only business domain models (Orders, Customers, etc.).
- [ ] No compilation errors in any commerce module.

---

## Issue 2: Pluggable High-Speed StateStore (NATS, Redis, In-Memory)

**Type**: AFK
**Blocked by**: Issue 1

### Parent PRD

`docs/PRD_Pluggable_Workflow_Engine.md`

### What to build

Implement the single source of truth for workflow execution state. This must be pluggable via application configuration.

- Define the `StateStore` interface in `internal/workflow` (Get/Save state, Get/Save input, TTL).
- Implement `InMemStore` using thread-safe Go maps (for fast dev/tests).
- Implement `NATSStore` using NATS JetStream KV buckets (Production Default).
- Implement `RedisStore` using `go-redis` (Enterprise Alternative).
- Update `pkg/config/config.go` to support `WORKFLOW_STORE_TYPE` (`mem`, `nats`, `redis`).

### How to verify

- **Automated**: Create a shared test suite in `internal/workflow/store_test.go` that runs against all three implementations (using mocks/local-containers for NATS/Redis).

### Acceptance criteria

- [ ] `StateStore` correctly persists and retrieves simple key-value state for a `workflowID`.
- [ ] `NATSStore` correctly applies TTL to auto-expire old executions.
- [ ] The system boots with the store specified in the environment variables.

---

## Issue 3: Checkpointed Workflow Runner

**Type**: AFK
**Blocked by**: Issue 2

### Parent PRD

`docs/PRD_Pluggable_Workflow_Engine.md`

### What to build

Refactor the `Runner` to be state-aware. It must "checkpoint" its progress so it can be resumed from the exact point of failure.

- Update `Runner.Execute` to save the initial `input` to the `StateStore`.
- Intercept step execution:
    - Save state `RUNNING` for `stepID` before calling the handler.
    - Save the return value (result) and state `COMPLETED` for `stepID` after the handler returns.
- Ensure that if a step fails, the `StateStore` reflects the `FAILED` status and stores the error message.

### How to verify

- **Automated**: Run a workflow and verify that the `StateStore` contains the correct step transitions and result payloads after execution completes.

### Acceptance criteria

- [ ] The `Runner` writes to the `StateStore` at every critical state transition.
- [ ] Independent DAG branches correctly checkpoint their state without race conditions.

---

## Issue 4: The Resumption Engine (Crash Recovery)

**Type**: AFK
**Blocked by**: Issue 3

### Parent PRD

`docs/PRD_Pluggable_Workflow_Engine.md`

### What to build

Implement the Medusa-style crash recovery logic. This allows Hyperrr to finish workflows that were interrupted by a server restart.

- Add `ResumeExecution(ctx, workflowID)` to the `Runner`.
- Implementation Logic:
    1. Retrieve original `input` from the `StateStore`.
    2. Reconstruct the DAG based on the workflow definition.
    3. Query the `StateStore` for step states.
    4. Skip all `COMPLETED` steps (re-injecting their saved results into the results map).
    5. Re-invoke any steps that were `RUNNING`, `FAILED`, or `PENDING`.
- Ensure idempotency keys are respected so that a re-invoked step doesn't duplicate side effects.

### How to verify

- **Automated**: Start a 3-step workflow, mock-fail/restart after step 1, call `ResumeExecution`, and verify that only steps 2 and 3 execute.

### Acceptance criteria

- [ ] `ResumeExecution` successfully completes an interrupted workflow.
- [ ] The final system state is identical whether the workflow runs in one go or is interrupted and resumed.

---

## Issue 5: Standardized Workflow DX & Event Helpers

**Type**: AFK
**Blocked by**: Issue 4

### Parent PRD

`docs/PRD_Pluggable_Workflow_Engine.md`

### What to build

Polish the Developer Experience to make creating commerce workflows as easy as MedusaJS, while standardizing how events are emitted.

- Create a `workflow.Emit(ctx, eventType, payload)` helper.
- Standardize the `TaskHandler` context so it's easy to access the `workflowID` and `StateStore`.
- Update `internal/context` (the Projector) to subscribe to the `EventBus` and build its read-only dashboard data *only* from events, completely decoupled from the Workflow Engine's state.

### How to verify

- **Manual**: Refactor the `order.checkout` workflow to use the new helpers and verify it is cleaner and more readable.

### Acceptance criteria

- [ ] `internal/context` (Observability) is 100% decoupled from `internal/workflow` (Execution).
- [ ] Standardized event emission is used across all commerce modules.
