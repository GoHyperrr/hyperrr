# Workflows and DAG Execution Engine

Hyperrr utilizes a custom, lightweight workflow engine built in Go (`internal/workflow`) that models business logic as a **Directed Acyclic Graph (DAG)** of execution steps. This design makes it easy to structure complex business flows (e.g. multi-step order checkout, travel bookings, inventory reserves) and run independent parts in parallel.

---

## 1. Declarative Workflow Definition

A workflow is defined in pure Go structs, mapping directly to JSON schemas for autonomous AI execution:

```go
type Workflow struct {
	Name        string         // Unique name of the workflow (e.g., "fulfillment.v1")
	Version     string         // SemVer versioning (e.g., "1.0.0")
	Description string         // Human and AI-readable description of what the workflow does
	ExposeToAI  bool           // If true, registers automatically as a tool on the MCP server
	InputSchema map[string]any // JSON Schema defining required/optional inputs
	Steps       []Step         // Collection of steps forming the DAG
}
```

### Structuring a Step
Each step specifies a task handler and dependency parameters:

```go
type Step struct {
	ID        string    // Unique identifier of the step (e.g., "charge_card")
	Uses      string    // The registered TaskHandler name (e.g., "finance.charge_card")
	DependsOn []string  // Steps that must successfully finish before this step starts
	Retry     *RetryOpt // Optional retry configuration for transient errors
	Saga      *Saga     // Optional rollback step executed during transactional failures
}
```

---

## 2. Dynamic DAG Resolver and Parallel Execution

The Workflow Runner (`workflow.Runner`) executes steps in parallel where dependencies allow. It achieves this without pre-computing a static topological sort, resolving ready steps dynamically as upstream steps complete.

### Step Evaluation Loop
The execution loop follows these steps:
1. **Identify Ready Steps**: The runner iterates through all steps. A step is ready if it hasn't launched yet, and all steps in its `DependsOn` array are marked completed.
2. **Parallel Goroutines**: Every ready step is launched in its own goroutine:
   ```go
   go r.runStep(ctx, id, step, results, stepFinished)
   ```
3. **Synchronization**: A central `stepFinished` channel coordinates execution:
   - When a step completes, it writes its status (success or failure) to `stepFinished`.
   - The main loop blocks on `stepFinished`, updates the `completed` step map upon success, and loops to evaluate if new downstream steps have become ready.
4. **State Machine Context**: A local `sync.Mutex` protects the execution state (`completed`, `launched`, and the shared `results` map).

---

## 3. Resumability and State Checkpoint Save Points

A primary feature of Hyperrr's workflow engine is **durability**. If the application crashes, a power failure occurs, or a node restarts, the workflow runner can resume executions from the exact step they left off.

### Checkpointing Steps
At critical transitions, the runner saves checkpoints using the `StateStore` driver:
- **`InitializeExecution(ctx, id, inputBytes)`**: Initiates state in the store with raw inputs.
- **`SaveState(ctx, id, stepID, outputBytes)`**: Stores step outcomes, which are merged back into the execution context during recovery.
- **State Checkpoints**: Every step transition writes its status (e.g. `StateRunning`, `StateCompleted`, `StateFailed`) to the database.

### Auto-Recovery Loop
At startup, the boot supervisor queries the `StateStore` for stalled executions:
```go
stalled, _ := store.ListExecutions(ctx, workflow.StateRunning)
for _, execID := range stalled {
    states, _ := store.GetState(ctx, execID)
    wfName := states["__wf_name"]
    wf, _ := registryStore.Get(wfName)
    
    // Resume execution
    go runner.ResumeExecution(ctx, execID, wf)
}
```
`ResumeExecution` reads all completed step outputs from the store, pre-populates the results context, sets the `completed` and `launched` maps, and restarts the execution loop from the first uncompleted steps.

---

## 4. Error Tolerances and Retries

For transient issues (e.g. network requests to third-party APIs), steps can declare exponential retries:

```go
type RetryOpt struct {
	MaxAttempts int           // E.g., 3 retries
	InitialDelay time.Duration // E.g., 100ms
	MaxDelay    time.Duration // E.g., 2s
	Backoff     float64       // E.g., 2.0 (doubles delay each time)
}
```

If a step fails and its maximum attempts are not exhausted, the runner calculates the backoff delay:
$$\text{Delay} = \min(\text{InitialDelay} \times \text{Backoff}^{\text{Attempt}}, \text{MaxDelay})$$
It then schedules a retry timer before re-executing the step handler. If all attempts fail, the runner triggers the Saga rollback transaction flow.
