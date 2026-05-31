# PRD: Composable TUI Admin Dashboard (Sample Scope)

## Problem Statement

As Hyperrr transitions to a modular engine, developers and operators need a centralized dashboard to monitor system health, inspect inventory, and view customer accounts. Currently, there is no administrative console. Operators have to run GraphQL queries or query databases directly, which is inefficient.

## Solution

We will build an interactive, modular Terminal User Interface (TUI) as a terminal-based administrative dashboard. 

The dashboard shell acts as a layout container (rendering headers, navbar tabs, and footer keymaps). Individual modules register their own views dynamically using Go package initializations. 

For this initial sample scope, we will build:
1.  **Product Administration**: Support listing products, adding new products, and updating product records (price/inventory).
2.  **Customer Administration**: Support listing registered customer accounts.

---

## User Stories

1.  **As an** administrator, **I want** to see a unified navigation header, **so that** I can switch tabs between Products and Customers.
2.  **As a** merchant, **I want** to view the current product inventory table (ID, Name, SKU, Price, Stock Level), **so that** I can monitor inventory.
3.  **As a** merchant, **I want** to add a new product directly from the terminal, **so that** I can expand the catalog.
4.  **As a** merchant, **I want** to update the price and stock level of a selected product, **so that** I can manage pricing and inventory.
5.  **As an** operations team member, **I want** to view registered customer accounts, **so that** I can audit registration details.
6.  **As a** developer, **I want** to write a module and register its TUI pages without modifying core TUI code, **so that** modules remain decoupled.

---

## Implementation Decisions

### 1. Composable Page Registry
*   **TUIPage Interface**: Defined in `pkg/registry/module.go`. Each page is a Bubble Tea sub-model:
    *   `Title() string` (Tab name)
    *   `Init(context.Context, *Dependencies) tea.Cmd` (Initializes state)
    *   `Update(tea.Msg) (TUIPage, tea.Cmd)` (Processes local keys/messages)
    *   `View() string` (Renders viewport content)
*   **TUIProvider Interface**: Implemented by modules to expose pages:
    ```go
    type TUIProvider interface {
        TUIPages() []TUIPage
    }
    ```
*   **Dynamic Scanning**: At startup, the core layout loop scans all active modules in `registry.List()`. If a module implements `TUIProvider`, its pages are automatically loaded and rendered as tabs.

### 2. Tab Navigation Pattern
*   Number keys `1` through `9` switch directly to corresponding tabs.
*   `Tab` / `Shift+Tab` and Left/Right arrow keys cycle through tabs.
*   **Focused Context Delegation**: General keystrokes (`q`, `ctrl+c`) are captured globally. All other key messages (vertical scrolls, input forms) are delegated directly to the active `TUIPage`.

---

## Module Design

### 1. Master TUI App Container (`internal/tui/app.go`)
*   **Responsibility**: Renders header/footer, manages tab switching, and delegates events.
*   **Interface**: Implements `tea.Model`.
*   **Tested**: Yes.

### 2. Products Admin Page (`commerce/product/tui.go`)
*   **Responsibility**: Renders active product list, handles the "add product" text inputs, and updates pricing/inventory.
*   **Interface**: Implements `registry.TUIPage`.
*   **Tested**: Yes.

### 3. Customers Admin Page (`commerce/customer/tui.go`)
*   **Responsibility**: Lists active customer records.
*   **Interface**: Implements `registry.TUIPage`.
*   **Tested**: Yes.

---

## Testing Decisions

*   We will write unit tests using Bubble Tea's test patterns, verifying that:
    - The core layout correctly loads pages registered dynamically by active modules.
    - Page models successfully transition states when receiving messages (like update notifications or key presses).
    - Page views render the expected textual lists and status borders.
*   Tests will be added to `internal/tui/app_test.go` and module-specific TUI test files.

---

## Out of Scope

*   Workflows DAG execution and monitoring.
*   Order fulfillment triggers.
*   Generating graphs or charts.
