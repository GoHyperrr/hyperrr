# Distributed Transactions and Saga Compensations

In a modular microservices architecture, maintaining database consistency across domains is a significant challenge. Traditional database-level transactions (such as Two-Phase Commit / 2PC) do not scale well, block resources, and introduce tight coupling.

Hyperrr solves this using the **Saga Pattern** inside its Workflow Runner, guaranteeing eventual consistency across multiple pluggable modules without database-level locks.

---

## 1. The Saga Pattern Overview

A Saga is a sequence of local transactions. For every step in a workflow, there is a corresponding **compensation step** (a rollback handler) designed to undo the changes made by the forward step if subsequent steps fail.

```
[Forward Pass]
Step 1: Reserve Inventory (Success)  --->  Step 2: Process Payment (Failed)
                                                    |
                                                    v
[Backward Pass (Rollback)]                          |
Step 1 Compensation: Release Inventory  <-----------+
```

---

## 2. Forward and Backward Pass Execution

The `workflow.Runner` executes workflows in two distinct phases:

### 2.1 The Forward Pass
As steps are executed in parallel or sequence, the runner records a running **history log** of completed steps.
- Only steps that completed successfully are added to the `history` slice in the chronological order of execution.
- If a step fails and exhausts its retry limits, the runner terminates the forward pass immediately.

### 2.2 The Backward Pass (Compensation)
Once a failure is caught, the runner triggers the compensation flow by calling the private `compensate()` method:
1. It loops through the `history` slice in **reverse chronological order** (last completed step is rolled back first):
   ```go
   for i := len(history) - 1; i >= 0; i-- {
       step := history[i]
       if step.Saga == nil || step.Saga.Uses == "" {
           continue
       }
       // Retrieve registered compensation handler and run it
   }
   ```
2. The compensation handler is invoked with the accumulated `results` map as its input context.

---

## 3. Context Sharing and Rollback Inputs

For a compensation task to successfully undo changes, it must know exactly what was created or modified during the forward step. Hyperrr enables this by passing the **complete execution context** to the saga handler.

### Example Context Structure
When `finance.refund_payment` runs, the input map contains the results of all previous steps:

```json
{
  "input": {
    "customer_id": "cust_123",
    "amount": 250.00
  },
  "hotel.reserve_room": {
    "booking": {
      "id": "book_987",
      "status": "PENDING"
    }
  },
  "finance.charge_card": {
    "transaction_id": "tx_4455",
    "status": "SUCCESS"
  },
  "_workflow_id": "wf_exec_0001"
}
```

The compensation handler reads values directly from previous steps:
```go
func (m *Module) RefundPayment(ctx context.Context, input any) (any, error) {
	data := input.(map[string]any)
	
	// Access charge card step output to get the transaction ID
	chargeStep := data["finance.charge_card"].(map[string]any)
	txID := chargeStep["transaction_id"].(string)

	// Issue refund against this transaction ID
	return m.paymentGateway.Refund(txID)
}
```

---

## 4. Critical Rollbacks and System Alerts

Not all compensation steps can be assumed to succeed automatically. For example, if a refund API call fails due to a network outage or card issuer error, the system must handle it.

```go
type Saga struct {
	Uses       string `yaml:"uses" json:"uses"`
	IsCritical bool   `yaml:"is_critical,omitempty" json:"is_critical,omitempty"`
}
```

### Saga Error Options:
*   **Non-Critical Rollbacks (`IsCritical: false`)**: If the compensation handler returns an error, the system logs the failure and continues to execute the remaining compensation steps.
*   **Critical Rollbacks (`IsCritical: true`)**: If a critical compensation handler fails, it indicates a severe inconsistency (e.g. money charged but room booking failed, and refund failed). The engine logs a prominent `CRITICAL COMPENSATION FAILED` alert, publishes a failure alert event on the `EventBus` to notify monitoring systems, and requires manual operator intervention.
