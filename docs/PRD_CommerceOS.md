# PRD: hyperrr — AI-Observable Distributed Commerce OS

## Problem Statement

Modern commerce operations are fragmented across disconnected systems, opaque automations, and manual coordination layers. Existing commerce platforms are built around CRUD interfaces and admin panels rather than operational execution, making them fundamentally unsuitable for AI-native, workflow-driven commerce. As commerce complexity increases through multi-channel operations, realtime automation, and distributed systems, merchants lack unified operational visibility, durable workflow orchestration, event-level observability, and AI-safe operational context. This results in operational fragility, invisible failures, and AI systems operating without sufficient context or safeguards.

## Solution

Build hyperrr, an AI-native commerce operating system where workflows, events, and operational context form the foundational runtime. The system abstracts commerce operations into deterministic, replayable DAGs (workflows) connected by an event fabric. AI (both LLMs and ML models) acts as an observable participant, never operating blind, but fully aware of the execution lineage, state, and causality. The platform is an executable knowledge graph of commerce state that supports policy-driven failure orchestration, allowing seamless handoffs between automated retries, AI suggestions, and human operators via a rich TUI mission control.

## User Stories

1. As an Operator, I want to view a real-time visualization of workflow executions in a TUI, so that I can immediately identify bottlenecks or failures.
2. As a Developer, I want to define commerce workflows (like order fulfillment) using a declarative YAML DSL, so that business logic is separated from hardcoded orchestration.
3. As a System, I want to evaluate a failure policy when a workflow step fails, so that I can automatically transition to a retry, fallback, compensation, or degraded state without crashing.
4. As an Operator, I want the workflow engine to pause and alert me (WAITING_HUMAN state) when retries and fallbacks are exhausted, so that I can manually intervene and resolve the issue.
5. As an AI Agent, I want access to the full execution lineage and entity graph via the Context Engine, so that I can provide highly accurate rerouting or intervention suggestions during a failure.
6. As a Merchant, I want all state changes to be driven exclusively by events and workflows, so that I have a complete, auditable, and replayable history of my commerce operations.
7. As an ML Model, I want to subscribe to the event fabric, so that I can asynchronously emit new segmentation or recommendation events back into the system.
8. As a Developer, I want the Event Fabric to guarantee at-least-once delivery and for all workflow steps to be strictly idempotent, so that the system remains consistent even during network partitions or crashes.

## Implementation Decisions

- **API Philosophy**: Modular Monolith with internal service mesh communication. Exposes a public GraphQL API for external querying and AI interaction.
- **Module Registry**: Every module (Core or Commerce) is implemented as a **Plugin** that registers its capabilities (Workflows, Handlers, Models) with a central Registry for dynamic discovery.
- **Server Runtime**: A long-running Go server process hosting the OS core, accessible via HTTP/GraphQL (Playground enabled for developer testing).
- **Core Rule**: Nothing mutates state directly. Everything flows through workflows and events.
- **Workflow Engine**: Custom DAG executor driven by a declarative DSL (YAML). Supports state machine transitions (RUNNING, RETRYING, WAITING_HUMAN, COMPENSATING, DEGRADED, etc.).
- **Failure Handling**: Policy-driven (not hardcoded). Explicit retry, fallback, compensation, and escalation policies defined per step.
- **Event Fabric**: Abstracted `EventBus` interface allowing swappable providers (NATS/Kafka/Redis) with at-least-once delivery guarantees.
- **Modular Data Persistence**: Database-agnostic abstraction (`pkg/db`) supporting SQLite and Postgres. Modules own their schemas and interact via Repositories. Cross-module foreign keys are prohibited; consistency is maintained via events.
- **Identity vs. Customer**: `internal/identity` handles the security boundary (User, Auth, API Keys, Actor Types: Human/AI/System) without knowledge of commerce domains. `commerce/customer` handles business profiles and ML segmentations.
- **Authentication**: JWT-based authentication at the OS level. `internal/auth` manages token lifecycles, and a central middleware injects the validated `Actor` into the request context.
- **Object Storage**: Centralized `internal/storage` module provides an abstraction for file handling (Local/S3/GCS) required by commerce plugins (images, labels, invoices).
- **Workflow-Driven Mutations**: Adhering to the "Nothing mutates state directly" doctrine, all GraphQL mutations (e.g., `createProduct`, `updateCustomer`) trigger declarative Workflows rather than direct DB calls. This ensures every change is auditable, replayable, and AI-observable via the Context Engine.
- **Standardized Observability**: Centralized logging system (`pkg/logger`) using structured logging (`slog`). Supports swappable handlers for future integrations (Sentry, GCP, OpenTelemetry).
- **AI Integration**: AI is an observable participant runtime. It does not dictate policy but can provide suggestions and operate within explicit workflow steps or asynchronously via events.
- **Observability**: Dedicated Context Engine to aggregate state, event history, and operational metrics for both human operators and AI consumption.

## Module Design

### 1. Event Fabric (`/pkg/eventbus`)
- **Responsibility**: Provides a unified, provider-agnostic interface for async messaging and event broadcasting.
- **Interface**: `Publish`, `Subscribe`, `Request`, `Reply` methods ensuring at-least-once delivery.
- **Tested**: Yes. Focus on provider implementations (e.g., NATS) and simulated network partitions.

### 2. Workflow Engine (`/internal/workflow`)
- **Responsibility**: Parses workflow DSL, executes DAGs, enforces idempotency, evaluates step-level policies, and manages execution state transitions.
- **Interface**: `StartWorkflow`, `ResumeWorkflow` (for human/AI intervention), `GetWorkflowState`.
- **Tested**: Yes. Heavy unit testing for policy evaluation, failure state transitions, and DAG execution rules.

### 3. Context Engine (`/internal/context`)
- **Responsibility**: Aggregates workflow state, event history, and entity graphs to construct rich, safe operational context for LLMs and Operators.
- **Interface**: GraphQL endpoints to query execution lineage and correlated entities.
- **Tested**: Yes. Snapshot testing of context payloads based on specific event histories.

### 4. Operator Console (`/tui`)
- **Responsibility**: A rich terminal-based mission control for visualizing real-time event streams, workflow DAGs, and handling required human interventions.
- **Interface**: Connects via gRPC streams to the Workflow Engine and Event Fabric.
- **Tested**: Yes. Mostly integration tests simulating workflow failures and verifying UI state changes.

## Testing Decisions

- **Test-First Architecture**: Contracts and tests must be written before implementation for all modules.
- **Chaos & Replay**: High emphasis on event replay tests (ensuring deterministic outcomes) and chaos tests (dropping events, timing out steps to verify policy evaluation).
- **Coverage**: Aiming for 95%+ coverage on core infrastructure (Workflow Engine, Event Fabric).

## Out of Scope

- Building storefront frontends (Next.js, etc.).
- Complex vector database implementations for semantic search (MVP will use Postgres FTS + simple embeddings).
- Hardcoding specific third-party integrations (e.g., Stripe, Shopify) directly into the core; these must be implemented as isolated plugins/workflows.

## Open Questions

- **AI Token/Cost Management**: Who owns tracking the cost and rate limits of AI participants when they are heavily utilized for workflow interventions? (Suggested Owner: Core Architecture Team).
- **Long-Running Saga Storage**: What is the primary storage mechanism for persistent, long-running workflow state across server restarts? (Suggested Resolution: Postgres via generic state interface).

## Current Progress

### ✅ Foundational OS Layer (Completed)
- **Scaffolding & DX**: Industry-standard Go setup with 95% coverage mandate, strict linting, and automated workflows.
- **Modular Persistence**: GORM-based abstraction supporting SQLite and Postgres with soft-relationship patterns.
- **Event Fabric**: In-memory event bus providing at-least-once delivery for inter-module communication.
- **Workflow Engine**: YAML DSL parser and execution runner with DAG support.
- **Resilient Execution**: Policy-driven retries (exponential backoff), fallbacks, and Saga-based compensation.
- **Mission Control**: Bubbletea-based TUI for real-time monitoring and manual intervention (Pause/Resume/Cancel).
- **Context Engine**: Execution lineage graph and GraphQL API to provide rich operational context for AI.

### ⏳ Next Milestones
- **Commerce Modules**: Catalog, Inventory, and Order plugins.
- **AI Integration**: Orchestration logic for LLM participants.

## Further Notes

- This system's moat is "Workflow Intelligence" and "AI Context Infrastructure." The goal is not just to sell commerce features, but to sell a resilient, executable commerce graph.