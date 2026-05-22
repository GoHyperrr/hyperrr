# Tasks for #17: Commerce Plugin: Support & AI Helpdesk

Parent issue: #17
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Support Ticket Domain Model & Repository
**Type**: WRITE
**Output**: `commerce/support/model.go` and `commerce/support/repository.go`.
**Depends on**: none

Implement models for `Ticket` (ID, CustomerID, Subject, Status: OPEN, RESOLVED, CLOSED) and `Message` (ID, TicketID, SenderType: HUMAN/AI, Content).

### 2. Implement Support Module as a Plugin
**Type**: WRITE
**Output**: `commerce/support/module.go` implementing the `registry.Module` interface.
**Depends on**: 1

Register the `commerce.support` module. Wire up the repository and register it in `internal/app`.

### 3. Implement AI Agent Support Handler
**Type**: WRITE
**Output**: `commerce/support/handlers.go`.
**Depends on**: 2

Implement workflow task handlers:
- `support.create_ticket`: Initializes a ticket.
- `support.dispatch_ai_response`: A mock handler that queries the `Context Engine` for the customer's last workflow execution status to generate a helpful automated response.

### 4. Define Support GraphQL Schema
**Type**: WRITE
**Output**: `commerce/support/support.graphqls`.
**Depends on**: 2

Expose the domain via GraphQL. Include queries for `getTicket`, `listCustomerTickets` and mutations `createTicket`, `addTicketMessage`.

### 5. Exhaustive Testing
**Type**: TEST
**Output**: `commerce/support/support_test.go`.
**Depends on**: 4

Verify ticket creation and the AI response flow. Ensure the AI response correctly "sees" other module state via the Context Engine.
