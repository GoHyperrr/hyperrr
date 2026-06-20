# ⚡ Hyperrr — AI-Observable Distributed Commerce Engine

[![Go Reference](https://pkg.go.dev/badge/github.com/GoHyperrr/hyperrr.svg)](https://pkg.go.dev/github.com/GoHyperrr/hyperrr)
[![Go Coverage](https://github.com/GoHyperrr/hyperrr/wiki/coverage.svg)](https://raw.githack.com/wiki/GoHyperrr/hyperrr/coverage.html)

Hyperrr is a modern, event-native commerce engine designed as a modular monolith. It treats all commerce operations as deterministic, replayable DAG workflows connected by a robust event fabric, managed via a powerful and extensible Cobra-based Go CLI.

---

## 🏗️ System Architecture

```text
       ┌────────────────────────┐         ┌────────────────────────┐
       │  hyperrr CLI (Cobra)   │◄───────►│  GraphQL Playground    │
       │ (Go Developer Console) │         │   (Web API Tester)     │
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

* **🔌 Pluggable Out-of-Tree Infrastructure**: Decoupled PostgreSQL (GORM dialect) and NATS JetStream (EventBus/Locker/State Store) into pluggable modules (`database` and `event-bus` repositories). The core engine retains zero external dependencies, defaulting to SQLite and in-memory pub-sub.
* **Consolidated Developer Tooling**: A single compiled binary (`hyperrr`) controls the entire commerce engine, configuration, diagnostics, and modules.
* **Unified GraphQL API Gateway**: Merges schema definitions and dynamic module resolvers dynamically.
* **Extensible CLI Command Structure**: A clean, structured Cobra CLI command layout featuring resource-based groupings, self-documenting commands, and diagnostic health checks.
* **Declarative Sagas & Compensation**: Handles complex multi-step operations (like Order Checkout) as DAGs, automatically running compensation procedures if a step fails.
* **AI-Observable Context**: A dedicated Context Engine tracks exact execution steps and outputs, exposing resources to AI agents over SSE (Model Context Protocol).

---

## 🚥 Getting Started

### 1. Run Schema Aggregation & Compile the Executable
Discover GraphQL schemas, run `gqlgen` code generation, and compile the unified `hyperrr` server executable:
```bash
go run ./cmd/builder
```
This writes the compiled binary to `bin/hyperrr` (or `bin/hyperrr.exe` on Windows).

After compiling, you can also use the CLI's built-in build command to rebuild dynamically:
```bash
./bin/hyperrr build
```

### 2. Start the Backend Commerce Server
```bash
./bin/hyperrr server
```
By default, the backend server will launch on port `8080`, migrate its SQLite database (`hyperrr.db`), and make the **GraphQL Playground** available at `http://localhost:8080`.

### 3. Run System Diagnostics
```bash
./bin/hyperrr doctor
```
Run a full diagnostic check on the system, active configuration, database connection, module registry, and server port.

### 4. Manage System Configuration
```bash
./bin/hyperrr config list
./bin/hyperrr config set SERVER_PORT 8080
```
Easily view, modify, and list all resolved configurations.

### 5. Centralized Configuration
Hyperrr loads its settings from `hyperrr.yml` at boot time.
* **Environment Variable Expansion**: You can use `${VAR_NAME}` or `${env.VAR_NAME:fallback}` anywhere in `hyperrr.yml`.
* **Strict Validation**: All settings (ports, database drivers, auth providers, etc.) are strictly validated on boot.
* **Auth Providers**: Explicitly list active authentication methods for HTTP API and MCP agent gateways using the `auth_providers` and `mcp_auth_providers` configuration options.

---

## 🧪 GraphQL Seed Data Sandbox

When starting with a fresh database, visit the GraphQL Playground at `http://localhost:8080/` and execute these queries to populate your database with mock records:

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

### B. Register a Customer Profile & API Keys
You can register users via GraphQL or directly from your terminal using the dynamic CLI command:

```bash
# Register a user via the CLI
./bin/hyperrr auth user register dev@example.com mypassword "Developer User"

# Generate an API Key for MCP AI Agents
./bin/hyperrr auth apikey generate
```

To register via the GraphQL Playground (which emits `identity.user_created` to automatically create a customer profile):
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

## 📖 Reference Documentation

For detailed design specifications, guides, and tutorials, please visit the official documentation website:
* [Hyperrr Documentation Website](https://hyperrr.com/docs/)

Detailed developer guides, architecture plans, and specifications have been consolidated on the site:
* [System Architecture](https://hyperrr.com/docs/architecture.html)
* [Module Development Kit (MDK)](https://hyperrr.com/docs/mdk.html)
* [Core Gateway & Builder](https://hyperrr.com/docs/core-gateway.html)
* [End-to-End E-Commerce Backend Recipe](https://hyperrr.com/docs/recipe-e2e.html)
* [Branching, Versioning & Release Promotion](https://hyperrr.com/docs/release-management.html)


