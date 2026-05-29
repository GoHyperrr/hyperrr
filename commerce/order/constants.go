package order

// Order statuses.
const (
	StatusPending   = "PENDING"
	StatusPaid      = "PAID"
	StatusFulfilled = "FULFILLED"
	StatusCancelled = "CANCELLED"
)

// Event types.
const (
	EventOrderCreated = "order.created"
	EventOrderPaid    = "order.paid"
)

// Workflow task names.
const (
	TaskCreateOrder   = "order.create"
	TaskFinalizeOrder = "order.finalize"
)
