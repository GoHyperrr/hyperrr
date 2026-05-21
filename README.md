# hyperrr — AI-Observable Distributed Commerce OS

hyperrr is an AI-native commerce operating system where workflows, events, and operational context form the foundational runtime. It treats commerce operations as deterministic, replayable DAGs connected by a robust event fabric.

## 🏗️ Architecture

```text
┌────────────────────┐      ┌────────────────────┐
│   Mission Control  │◄─────┤   Operator (Human) │
│      (Rich TUI)    │      └────────────────────┘
└─────────┬──────────┘
          │ (gRPC/Events)
          ▼
┌────────────────────────────────────────────────────┐
│                  hyperrr Runtime                   │
├─────────────────┬─────────────────┬────────────────┤
│ Workflow Engine │  Event Fabric    │ Context Engine │
│ (DAGs + Sagas)  │ (PubSub/Stream) │ (Entity Graph) │
└─────────┬───────┴────────┬────────┴────────┬───────┘
          │                │                 │
          ▼                ▼                 ▼
┌────────────────────────────────────────────────────┐
│                 Commerce Modules                   │
│  (Catalog, Orders, Inventory, Search, AI Plugins)  │
└──────────────────────────┬─────────────────────────┘
                           │
          ┌────────────────┴────────────────┐
          ▼                                 ▼
┌──────────────────┐               ┌──────────────────┐
│  SQL Persistence │               │   AI Participant │
│ (SQLite/Postgres)│               │  (LLM/ML Models) │
└──────────────────┘               └──────────────────┘
```

## 🚀 Key Features

- **Event-Native**: All state changes flow through the Event Fabric.
- **Workflow-First**: Commerce logic defined as declarative YAML DAGs.
- **Resilient**: Built-in Retry, Fallback, and Saga (Compensation) policies.
- **AI-Observable**: Dedicated Context Engine provides rich lineage for AI reasoning.
- **Operator-Driven**: Mission Control TUI for real-time visualization and manual intervention.
- **Modular Monolith**: Strict isolation between modules with swappable infrastructure.

## 🛠️ Tech Stack

- **Language**: Go 1.22+
- **Persistence**: GORM (SQLite/Postgres)
- **TUI**: Bubbletea / Lipgloss
- **Config**: Viper
- **Logging**: Structured `slog`
- **Linting**: golangci-lint
- **Hooks**: Lefthook

## 📖 Sample Workflow (YAML)

```yaml
name: product.enrichment.v1
steps:
  - id: fetch_product
    uses: catalog.get_product
    retry:
      max_attempts: 3
      backoff: exponential

  - id: generate_seo
    uses: ai.generate_seo
    depends_on: [fetch_product]
    fallback:
      step: catalog.use_basic_seo

  - id: save_product
    uses: catalog.save
    depends_on: [generate_seo]
    saga:
      uses: catalog.rollback_save
```

## 🚥 Getting Started

### Prerequisites
- Go 1.22+
- Make

### Setup
```bash
make setup
```

### Run Tests & Coverage (93%+ mandate)
```bash
make test
make coverage
```

### Run Mission Control
```bash
go run cmd/tui/main.go
```

## 🔄 How it Works: Workflow Execution

hyperrr orchestrates commerce operations through a "Failure-Native" execution model. Below is a sample flow of an Order Fulfillment workflow.

```text
  [ YAML DSL ]          [ Workflow Engine ]          [ Event Fabric ]
       │                        │                           │
       │─── (1) Parse DSL ─────►│                           │
       │                        │─── (2) Emit: Started ────►│
       │                        │                           │
       │                        │─── (3) Exec: Payment ────►│
       │                        │                           │
       │                        │◄── (4) Result: SUCCESS ───│
       │                        │                           │
       │                        │─── (5) Exec: Inventory ──►│
       │                        │                           │
       │                        │◄── (6) Result: FAILURE ───│
       │                        │                           │
       │                        │─── (7) Evaluate Policy ──►│
       │                        │           │               │
       │                        │     [ RETRY? ] ── NO      │
       │                        │     [ FALLBACK? ] ─ NO    │
       │                        │     [ SAGA? ] ── YES      │
       │                        │           │               │
       │                        │◄── (8) Exec: Refund ──────│
       │                        │                           │
       │                        │─── (9) Emit: Failed ─────►│
       ▼                        ▼                           ▼
```

### Features Demonstrated:
1.  **Deterministic DAG**: Steps execute only when dependencies are met.
2.  **Event-Driven**: Every transition is captured and broadcast.
3.  **Self-Healing**: Automatic retry/fallback based on declarative policies.
4.  **Sagas**: Automatic compensation (e.g., refunding if inventory fails).
5.  **Auditability**: Complete lineage of what happened, why, and what was undone.

## 🛡️ Guiding Doctrine
> "Nothing mutates state directly. Everything flows through workflows and events."
