# ⚡ Hyperrr — AI-Observable Distributed Commerce OS

Hyperrr is a modern, event-native commerce operating system designed as a modular monolith. It treats all commerce operations as deterministic, replayable DAG workflows connected by a robust event fabric. It features an interactive, decoupled terminal dashboard console alongside its GraphQL server, consolidated under a single Go CLI.

---

## 🏗️ System Architecture

```text
       ┌────────────────────────┐         ┌────────────────────────┐
       │  hyperrr admin (TUI)   │◄───────►│  GraphQL Playground    │
       │  (Bubble Tea v2 Console)│         │   (Web API Tester)     │
       └───────────┬────────────┘         └───────────┬────────────┘
                   │                                  │
                   │ (HTTP GraphQL POST /query)       │
                   ▼                                  ▼
       ┌───────────────────────────────────────────────────────────┐
       │                     hyperrr server                        │
       ├───────────────────┬───────────────────┬───────────────────┤
       │  Workflow Engine  │   Event Fabric    │  Context Engine   │
       │  (DAGs + Sagas)   │  (PubSub/Stream)  │  (Lineage Trace)  │
       └─────────┬─────────┴─────────┬─────────┴─────────┬─────────┘
                 │                   │                   │
                 ▼                   ▼                   ▼
       ┌───────────────────────────────────────────────────────────┐
       │                    Commerce Modules                       │
       │    (Catalog, Orders, Inventory, Search, AI Plugins)       │
       └─────────────────────────────┬─────────────────────────────┘
                                     │
                             GORM / DB Connection
                                     │
                                     ▼
                          [ SQLite / Postgres DB ]
```

### Architectural Guiding Doctrine
> **"Nothing mutates state directly. Everything flows through workflows and events."**
> In Hyperrr, all state modifications go through declarative workflow DAGs. This ensures every operational step is completely auditable, replayable, and observable.

---

## 🚀 Key Features

* **Consolidated Developer Tooling**: A single compiled binary (`hyperrr`) controls the entire commerce engine and operator dashboard.
* **Unified GraphQL API Gateway**: Merges schema definitions and dynamic module resolvers dynamically.
* **Composable TUI Dashboard**: Features a 100% decoupled Bubble Tea v2 dashboard console that loads state purely over HTTP POST queries.
* **Declarative Sagas & Compensation**: Handles complex multi-step operations (like Order Checkout) as DAGs, automatically running compensation procedures if a step fails.
* **AI-Observable context**: A dedicated Context Engine tracks exact execution steps and outputs, exposing resources to AI agents over SSE (Model Context Protocol).

---

## 🚥 Getting Started

### 1. Compile the Unified Executable
Compile the consolidated binary using Go:
```bash
# Using standard Go
go build -o bin/hyperrr ./cmd/hyperrr

# Or using Makefile
make build
```

### 2. Start the Backend Commerce Server
```bash
./bin/hyperrr server
```
By default, the backend server will launch on port `8080`, migrate its SQLite database (`hyperrr.db`), and make the **GraphQL Playground** available at `http://localhost:8080`.

### 3. Launch the Mission Control Console
Open a separate terminal window and launch the dashboard:
```bash
./bin/hyperrr admin
```

* **Connecting to custom/remote backends**: You can point the local console client to any Hyperrr server instance:
  ```bash
  ./bin/hyperrr admin --server http://api.my-store.com:8080
  ```

---

## ⌨️ TUI Navigation & Control Guide

* **Switch Tabs**: Press numbers `1` to `4` (or cycle using `Tab` / `Shift+Tab` and Left/Right arrows).
* **Scroll Tables**: Use `j` / `down` and `k` / `up` to move rows.
* **Interactive Forms**: Enter a page's edit/creation state, navigate text fields using `Tab` / `Shift+Tab`, and press `Enter` on the final field to save. Press `ESC` to cancel.
* **Live Refresh**:
  * **Auto-Reload**: Switching tabs automatically triggers a network query to keep data synchronized.
  * **Manual Reload**: Press `r` on any catalog list view to manually trigger a fresh network fetch from the backend.

---

## 🧪 GraphQL Seed Data Sandbox

When starting with a fresh database, visit the GraphQL Playground at `http://localhost:8080/` and execute these queries to populate your dashboard with mock records:

### A. Seed Catalog Products
```graphql
mutation {
  p1: createProduct(input: {
    id: "prod_mechanical"
    name: "Quantum Keyboard"
    description: "Mechanical keyboard with sub-atomic switches"
    price: 189.99
    currency: "USD"
  }) { id name price }

  p2: createProduct(input: {
    id: "prod_mouse"
    name: "Chrono Mouse"
    description: "Time-dilation optical gaming mouse"
    price: 85.50
    currency: "USD"
  }) { id name price }
}
```

### B. Register a Customer Profile
Registering a user emits an `identity.user_created` event that automatically triggers the creation of a customer profile:
```graphql
mutation {
  register(
    email: "alex@sterling.com"
    password: "securepassword123"
    name: "Alex Sterling"
  ) {
    token
    actor {
      id
      name
    }
  }
}
```

---

## 📖 Deep-Dive Reference Docs

Browse our specialized design docs inside the `docs/` folder to learn more about the core engines:
* [docs/tui.md](file:///D:/hyperrr-commerce-ai/hyperrr/docs/tui.md): Composable Decoupled TUI Shell and network layer.
* [docs/workflows_and_dags.md](file:///D:/hyperrr-commerce-ai/hyperrr/docs/workflows_and_dags.md): Declarative step definitions, RETRY gates, and parallel execution trees.
* [docs/event_fabric.md](file:///D:/hyperrr-commerce-ai/hyperrr/docs/event_fabric.md): Asynchronous Pub/Sub and namespace routing.
* [docs/model_context_protocol.md](file:///D:/hyperrr-commerce-ai/hyperrr/docs/model_context_protocol.md): MCP SSE gateway details.
* [docs/graphql_api_gateway.md](file:///D:/hyperrr-commerce-ai/hyperrr/docs/graphql_api_gateway.md): Zero Core Pollution dynamic module resolver container.

