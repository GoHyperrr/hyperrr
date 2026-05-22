# Implementation Issues: hyperrr — AI-Observable Commerce OS

## Status Legend
- ✅ **Completed**
- 🚧 **In Progress**
- ⏳ **Backlog**

---

## ✅ Issue 1: Project Scaffolding & Industry-Standard DX Tooling
**Status**: Completed

---

## ✅ Issue 2: Modular Database Abstraction & GORM Setup
**Status**: Completed

---

## ✅ Issue 3: Event Fabric Interface & In-Memory Provider
**Status**: Completed

---

## ✅ Issue 4: Workflow Engine: DSL Parser & Basic Execution
**Status**: Completed

---

## ✅ Issue 5: Policy-Driven Failure Orchestration & Sagas
**Status**: Completed

---

## ✅ Issue 6: Human-In-The-Loop: WAITING_HUMAN & TUI Shell
**Status**: Completed

---

## ✅ Issue 7: Context Engine: Execution Lineage GraphQL API
**Status**: Completed

---

## ✅ Issue 8: Plugin System & Module Registry
**Status**: Completed

---

## ✅ Issue 9: Core OS Extensions: Identity & Object Storage
**Status**: Completed

---

## ✅ Issue 10: Commerce Plugin: Product (PIM)
**Status**: Completed
**Type**: AFK

### Achievements
- Implemented the **Product Module** (`commerce/product`) with GORM persistence.
- Built a declarative **Product Creation & Update Workflow** ensuring all PIM changes are auditable.
- Exposed CRUD operations via the unified **GraphQL API** (`createProduct`, `updateProduct`, `deleteProduct`).
- Achieved high logic coverage for all product management logic.

---

## ✅ Issue 11: Commerce Plugin: Customer & ML Segmentation
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 9

### Achievements
- Implemented the **Customer Module** (`commerce/customer`) handling business-level profiles and addresses.
- Built a declarative **ML Persona Segmentation** workflow that autonomously calculates customer "Personas" (e.g., WHALE, GOLD).
- Integrated with the **Identity Module** via the **Event Fabric**: registering a new user identity automatically triggers background creation of a commerce customer profile.
- Exposed profile management via the unified **GraphQL API** (`updateCustomer`).
- Achieved 95%+ logic coverage for all customer and segmentation logic.

---

## ✅ Issue 12: OS-Level Authentication & JWT
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 9

### Achievements
- Implemented OS-level authentication using **JWT** and **bcrypt** for password hashing.
- Developed a dedicated `internal/auth` package for secure token management.
- Built a central **Auth Middleware** that injects the validated `Actor` into the request context for all API routes.
- Exposed `login` and `register` mutations via the unified GraphQL API.
- Leveraged the **Event Fabric** to ensure cross-module consistency: registering a new identity automatically triggers creation of a commerce-level Customer profile.
- Maintained 90%+ project logic coverage.

---

## ✅ Issue 13: Commerce Plugin: Cart & Checkout
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 11

### Achievements
- Implemented the **Cart Module** (`commerce/cart`) for managing shopping sessions and item persistence.
- Built declarative workflows for **Cart Operations** (`addItemToCart`, `removeItemFromCart`), ensuring every cart change is auditable.
- Developed the **Checkout Workflow** that transitions a cart to a COMPLETED status and prepares the system for order fulfillment.
- Exposed the Cart domain via the unified **GraphQL API** with automatic schema discovery.
- Achieved high logic coverage for all cart and checkout business logic.

---

## ✅ Issue 14: Commerce Plugin: Orders & Fulfillment Sagas
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 13

### Achievements
- Implemented the **Order Module** (`commerce/order`) for managing finalized commerce transactions.
- Built a multi-step **Fulfillment Saga** (`order.create` -> `order.process_payment` -> `order.finalize`) with explicit **Saga Compensation** (`order.compensate_payment`) to ensure distributed consistency.
- Wired the **Cart Module** to trigger the fulfillment saga upon checkout, transitioning from temporary intent to a paid order.
- Exposed the Order domain via the unified **GraphQL API** (`getOrder`, `listOrders`, `createOrderFromCart`).
- Achieved high logic coverage for all order and fulfillment saga logic.

---

## ✅ Issue 15: Commerce Plugin: Finance (Tax & Payments)
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 14

### Achievements
- Implemented the **Finance Module** (`commerce/finance`) with GORM persistence.
- Added workflow task handlers `finance.process_payment` and `finance.compensate_payment`.
- Replaced mock logic in the Fulfillment Saga (`commerce/order`) with real finance module workflows.
- Exposed the domain via GraphQL API (`getPayment`, `listOrderPayments`).
- Tested the cross-module saga coordination successfully.

---

## ⏳ Issue 16: Commerce Plugin: Fulfillment (Logistics & Tracking)
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 14

---

## ⏳ Issue 17: Commerce Plugin: Support & AI Helpdesk
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 14

---

## ⏳ Issue 18: Commerce Plugin: Marketing & Loyalty
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 14

---

## ✅ Issue 19: Commerce Plugin: Notifications (Omnichannel)
**Status**: Completed
**Type**: AFK
**Blocked by**: Issue 1

### Achievements
- Implemented the **Notification Module** (`commerce/notification`) for omnichannel message tracking.
- Created `Provider` interface and a mock implementation.
- Hooked into Event Fabric (`identity.user_created`, `workflow.completed`) to autonomously trigger `notification.send` workflows.
- Exposed `listNotifications` via GraphQL.
- Achieved robust logic coverage.

---

## ⏳ Issue 20: Commerce Plugin: Search (Semantic & Vector)
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 10

---

## ⏳ Issue 21: Commerce Plugin: Analytics (Operational BI)
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 1

---

## ⏳ Issue 22: Technical Debt & Architectural Refinement
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 13

### What to build
Harden the OS foundations and replace MVP shortcuts with production-grade logic.
- **Workflow Registry & Store**: Move from inline DAG definitions in resolvers to a central `WorkflowRegistry`. Implement persistence for workflow definitions (YAML/JSON) in the database.
- **Transactional Outbox Pattern**: Ensure that Event Fabric publications are atomic with database commits. Prevent "ghost events" if a transaction fails after an event is already sent.
- **ML Brain v2**: Replace the simple `if/else` persona logic in `commerce/customer` with a real `AI_Agent` actor that observes the Context Engine's lineage graph to make segmentation decisions.
- **JWT Refresh & Rotation**: Implement a secure refresh token flow. Add a "Token Blacklist" to `internal/auth` to support immediate revocation of sessions.
- **Dependency Injection Cleanup**: Standardize how modules receive and share utilities like the Logger and Config to reduce boilerplate in `Init`.

---

## ⏳ Issue 23: Audit Findings: Code Review & 95% Coverage Mandate
**Status**: Backlog
**Type**: AFK
**Blocked by**: Issue 22

### What to build
Address critical findings from the Issue 0-13 audit and reach the project's quality bar.
- **Logic Coverage (95%)**: Close the gap on the "Top 5 Uncovered Ranges" identified in `internal/context` (Projection logic) and `internal/workflow` (Compensation paths).
- **Formal Workflow State Machine**: Introduce a strict `Status` transition table in `internal/workflow/runner.go` to prevent race conditions during human intervention (`WAITING_HUMAN` -> `RUNNING`).
- **Unified Constants & Enums**: Eliminate magic strings by centralizing all system-wide statuses (Cart, Workflow, Actor) into `pkg/constants`.
- **Soft-Delete Enforcement**: Update the GORM models and Repository interfaces to use `gorm.DeletedAt` correctly across all modules (Product, Customer, Cart).
- **GraphQL Error Hardening**: Transition from `panic` calls to a unified `pkg/errors` that the API layer can translate into structured GraphQL errors with error codes.
