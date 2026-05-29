# PRD: Pluggable Stateful Workflow Engine (Medusa Architecture)

## Problem Statement

Currently, the Hyperrr OS executes workflows synchronously in-memory. If the server crashes or restarts mid-workflow, all execution state (which steps have completed, which are pending, and which require saga compensations) is permanently lost. 

While the system does write observability logs to the primary SQL database (via `internal/context`), treating the primary relational database as a high-throughput workflow state tracker introduces massive write-contention and scales poorly. 

We need the system to behave like a standard, reliable Modular Monolith: workflows should execute rapidly in the request goroutine, but they must be able to resume or roll back if a hardware failure interrupts them, without spamming the primary business database.

## Solution

We will introduce a **Pluggable Workflow State Engine**. 

Workflows will continue to execute synchronously (providing excellent Developer Experience and low latency). However, before and after every step, the engine will "checkpoint" the state to a high-speed Key-Value (KV) store.

If the server crashes, an operator or a boot script can call a new `ResumeExecution(workflowID)` method. The engine will query the KV store, determine exactly which steps were completed, and resume execution from the point of failure.

To prevent the KV store from growing infinitely, execution states will have a configurable Time-To-Live (TTL) and auto-expire after a set period (e.g., 24 hours), serving only as an ephemeral crash-recovery buffer.

## User Stories

1.  **As a Platform Operator**, I want workflow states to be checkpointed to a durable, high-speed store, so that a server crash does not result in permanently abandoned workflows.
2.  **As a Platform Operator**, I want to be able to call `ResumeExecution(workflowID)` after a crash, so that interrupted business processes (like order fulfillment) can complete successfully.
3.  **As a System Architect**, I want the checkpointing store to be pluggable, so that I can use NATS JetStream KV as the production default, swap to Redis if required by enterprise policies, or use In-Memory for rapid local testing.
4.  **As a Database Administrator**, I want workflow execution state kept out of the primary SQL database, so that business domain queries are not impacted by massive orchestration write-loads.
5.  **As a Developer**, I want old workflow execution states to automatically expire (TTL), so that I don't have to write custom sweeper jobs to prevent memory/storage exhaustion.
6.  **As a Developer**, I want the workflow definition DX to remain exactly as it is (synchronous function calls), so that I don't have to learn a complex distributed worker paradigm.

## Implementation Decisions

*   **Architecture Pattern**: MedusaJS-style Stateful Monolith. Workflows execute in the calling goroutine but checkpoint to an external engine.
*   **Default Production Engine**: **NATS JetStream KV**. Chosen for its native Go synergy (ability to embed or run alongside) and extreme performance.
*   **Supported Engines**:
    *   `mem`: `InMemStore` (for `make test` and local dev).
    *   `nats`: `NATSStore` (Production default, using JetStream KV).
    *   `redis`: `RedisStore` (Enterprise alternative).
*   **State Interface**: We will define a `workflow.StateStore` interface in `internal/workflow` that mandates `SaveStepState`, `GetWorkflowState`, and `SetTTL`.
*   **Crash Recovery**: 
    *   The `Runner` will be upgraded with a `Resume(ctx, workflowID)` method.
    *   It will pull the DAG state from the `StateStore`, identify `PENDING` or `RUNNING` steps, and re-invoke them.
    *   Existing idempotency checks (`pkg/db/idempotency.go`) will guarantee that partially completed steps are not double-processed.
*   **Observability (`internal/context`)**: The Projector will remain for *optional* long-term auditing, but it will be decoupled from the core workflow engine's critical path.

## Module Design

### 1. `internal/workflow/store.go` (New)
*   **Responsibility**: Define the interface for checkpointing workflow states.
*   **Interface**: 
    ```go
    type StateStore interface {
        SaveState(ctx context.Context, execID string, stepID string, state string) error
        GetState(ctx context.Context, execID string) (map[string]string, error)
        SetTTL(ctx context.Context, execID string, ttl time.Duration) error
    }
    ```
*   **Tested**: Yes.

### 2. `internal/workflow/store_inmem.go` (New)
*   **Responsibility**: Thread-safe map implementation for local testing.
*   **Interface**: Implements `StateStore`.
*   **Tested**: Yes.

### 3. `internal/workflow/store_nats.go` (New)
*   **Responsibility**: NATS JetStream KV implementation for production crash recovery.
*   **Interface**: Implements `StateStore`. Uses JetStream KeyValue buckets with native TTL support.
*   **Tested**: Yes.

### 4. `internal/workflow/runner.go` (Refactor)
*   **Responsibility**: Execute DAGs. Intercept execution before and after every step to call `store.SaveState`. Provide the `Resume` capability.
*   **Interface**: Add `ResumeExecution(ctx context.Context, id string, wf *Workflow, input any) error`.
*   **Tested**: Yes.

## Testing Decisions

*   **Mocking Crashes**: The primary test will involve starting a workflow, letting it complete Step 1, then forcibly exiting the function (simulating a crash). We will then call `ResumeExecution` and verify that Step 1 is skipped and Step 2 executes successfully.
*   **Store Concurrency**: We must run high-parallelism tests against the `InMemStore` and `NATSStore` to ensure that checkpointing does not introduce race conditions during Parallel DAG execution.
*   **Idempotency Verification**: Ensure that if a step was *actually* finished but the server crashed before the "COMPLETED" state was written to the KV store, the `retryStep` mechanism safely bounces off the `pkg/db/idempotency.go` layer.

## Out of Scope

*   Implementing the `RedisStore` immediately (we will define the interface and implement `InMem` and `NATS` first; Redis can be added later as the interface will support it).
*   Distributed worker pools (this remains a monolith).
*   Building an automated "re-boot" sweeper (we will build the `Resume` API; how the application calls it on startup is an infrastructure deployment concern).

## Open Questions

*   **TTL Duration**: What is the default Time-To-Live for a workflow state in the KV store?
    *   *Suggested*: 48 hours. This provides ample time for a weekend crash to be investigated and resumed on Monday morning without permanently filling up NATS storage.
*   **Input Payload Storage**: Should the `StateStore` also persist the initial `input` payload of the workflow, so that `ResumeExecution` only requires the `workflowID`?
    *   *Suggested*: Yes. If the `Runner` has to resume a workflow after a server reboot, it needs the original input data. The `StateStore` must save the `input` map during `EventWorkflowStarted`.

## Further Notes

This architectural pivot keeps Hyperrr aligned with the "Majestic Monolith" philosophy. It provides the enterprise safety net of crash recovery (like Temporal or Cadence) but without the massive operational burden of deploying separate worker fleets and orchestrator clusters. NATS KV provides the perfect middle-ground of extreme performance and native Go integration.
