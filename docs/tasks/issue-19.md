# Tasks for #19: Commerce Plugin: Notifications (Omnichannel)

Parent issue: #19
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define Notification Domain Model & Repository
**Type**: WRITE
**Output**: `commerce/notification/model.go` and `commerce/notification/repository.go`.
**Depends on**: none

Implement a `Notification` model to track messages sent to users (ID, Recipient, Channel, Subject, Body, Status: PENDING, SENT, FAILED).

### 2. Implement Notification Provider Interface
**Type**: WRITE
**Output**: `commerce/notification/provider.go`.
**Depends on**: none

Define an interface for sending messages (e.g., `Send(ctx, notification) error`). Implement a mock provider for local testing and development.

### 3. Implement Notification Module & Handlers
**Type**: WRITE
**Output**: `commerce/notification/module.go` and `commerce/notification/handlers.go`.
**Depends on**: 1, 2

Build the OS plugin. Implement workflow task handlers for sending notifications (`notification.send`).

### 4. Wire Event Fabric Triggers
**Type**: WRITE
**Output**: `commerce/notification/module.go` (Init method).
**Depends on**: 3

Subscribe to core OS events:
- `identity.user_created`: Send a welcome email.
- `workflow.completed` (where name == `fulfillment.v1`): Send an order confirmation.
These subscriptions should trigger the `notification.send` workflow asynchronously.

### 5. Define Notification GraphQL Schema
**Type**: WRITE
**Output**: `commerce/notification/notification.graphqls`.
**Depends on**: 3

Expose the notification domain via GraphQL. Include queries for `listNotifications` to allow users/support to see their message history.

### 6. Exhaustive Testing
**Type**: TEST
**Output**: `commerce/notification/notification_test.go`.
**Depends on**: 5

Verify the full flow: Emit an event -> Event Bus triggers subscription -> Workflow runs -> Provider sends message -> DB status updates to SENT. Ensure 90%+ logic coverage.
