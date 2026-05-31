# ISSUES: Composable TUI Admin Dashboard

This document details the Vertical Slice issues required to implement the composable TUI dashboard based on [PRD_TUI_Admin.md](file:///D:/hyperrr-commerce-ai/docs/PRD_TUI_Admin.md).

---

## Issue 1: Registry TUI Contracts & Core Shell Setup

**Type**: AFK  
**Blocked by**: None — can start immediately  

### Parent PRD

`docs/PRD_TUI_Admin.md`

### What to build

Define the `TUIPage` and `TUIProvider` Go interfaces inside `pkg/registry/module.go` (or `registry/registry.go`). 

Refactor `internal/tui/app.go` to act as the primary dashboard viewport. It must:
1. Scan all active modules using `registry.List()` at boot.
2. Filter modules implementing `registry.TUIProvider` and retrieve their `TUIPage` views.
3. Render a top navigation header listing the page titles (e.g., `1. Products | 2. Customers`).
4. Support keyboard navigation: number keys `1-9` or `Tab` / arrow keys to switch active tabs.
5. Capture global quit shortcuts (`q`, `ctrl+c`) and delegate all other keystrokes to the active `TUIPage` sub-model.

### How to verify

- **Manual**: Running `go run ./cmd/tui` starts the dashboard. Pressing `1` and `2` switches active tab indices.
- **Automated**: Write tests in `internal/tui/app_test.go` verifying:
  - App viewport initializes without panic.
  - Active tabs update on number/tab key strokes.
  - Global `q` key terminates the application loop.

### Acceptance criteria

- [ ] Given a registry with multiple dynamic modules that implement `TUIProvider`, when the TUI app boots, then all returned pages are displayed as navigation options in the header.
- [ ] Given an active page, when keys other than quit shortcuts are pressed, then those key messages are forwarded to the page's local `Update` routine.
- [ ] Given a quit key is pressed, then the program returns a `tea.Quit` command.

### User stories addressed

- User story 1: Unified navigation header
- User story 6: Dynamic module registration

---

## Issue 2: Products Page Component (List, Add, Update)

**Type**: AFK  
**Blocked by**: Issue 1  

### Parent PRD

`docs/PRD_TUI_Admin.md`

### What to build

Implement `TUIProvider` inside the `product` module (`commerce/product/module.go`). 

Build the `productPage` struct that implements `registry.TUIPage`:
1. **List Mode**: Displays a table of all products loaded from the database repository.
2. **Add Mode**: Pressing a key (e.g., `a`) displays a text input form (Name, SKU, Price, Stock) to create a new product. Submitting the form calls the product creation handler.
3. **Update Mode**: Selecting a product from the table and pressing `Enter` shows input prompts to update the selected product's Price and Stock Level. Submitting calls the database update.

### How to verify

- **Manual**: Run `go run ./cmd/tui`, select the "Products" tab, verify the product catalog lists correctly. Press `a`, fill in mock inputs, and submit. Verify the list updates with the new product. Select a product, press `Enter`, input a new price/stock, and confirm update.
- **Automated**: Write tests verifying:
  - `productPage.View()` renders the products table correctly.
  - Sending form submission messages (Add / Update) updates the database records and transitions the page state back to list mode.

### Acceptance criteria

- [ ] Given the products database is populated, when the Products page loads, then it displays a table containing ID, Name, SKU, Price, and Stock Level.
- [ ] Given a user is in add mode, when they input valid product details and submit, then a new database record is created and the table refreshes.
- [ ] Given a selected product, when the user modifies its price or stock and submits, then the database record is updated.

### User stories addressed

- User story 2: Product inventory table
- User story 3: Add new product
- User story 4: Update product price & stock

---

## Issue 3: Customers Page Component (List View)

**Type**: AFK  
**Blocked by**: Issue 1  

### Parent PRD

`docs/PRD_TUI_Admin.md`

### What to build

Implement `TUIProvider` inside the `customer` module (`commerce/customer/module.go`).

Build the `customerPage` struct that implements `registry.TUIPage`:
1. **List View**: Queries the customer repository and displays a formatted table of registered accounts, including Customer ID, Email, Name, and Registration Timestamp.
2. Enforce clean scroll controls (`j`/`k` or arrow keys) if the table extends past the viewport bounds.

### How to verify

- **Manual**: Run `go run ./cmd/tui`, select the "Customers" tab, and verify registered accounts render correctly.
- **Automated**: Write tests asserting that:
  - `customerPage.View()` outputs customer email and registration headers correctly.
  - Scroll updates adjust pagination or viewport focus.

### Acceptance criteria

- [ ] Given registered customer accounts, when the Customers page is active, then a table of customer profiles is displayed.
- [ ] Given scroll keys are pressed, then the page correctly offsets list indices.

### User stories addressed

- User story 5: View registered customer accounts
- User story 6: Dynamic module registration
