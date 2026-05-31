# Composable TUI Admin Dashboard (Decoupled client mode)

The **Composable TUI Admin Dashboard** (`mission-control`) serves as the terminal-based command center for managing catalog products, tracking orders, listing customer profiles, and monitoring live workflows.

Unlike typical monolithic CLI tools, the TUI is designed to be **completely decoupled from the database** and run as a **standalone client** on developer/operator workstations, communicating with the Hyperrr server exclusively via the network.

---

## 1. Subsystem Architecture

The TUI architecture employs a modular composition pattern where individual domain verticals declare their views, but compile into a unified, database-free executable.

```
       +--------------------------------------------------------+
       |                  TUI Client Executable                 |
       |       (Bubble Tea v2 Layout Container & Tabs)         |
       +---------------------------+----------------------------+
                                   |
                     GraphQL Queries & Mutations (HTTP)
                                   |
                                   v
       +--------------------------------------------------------+
       |                  Hyperrr Backend server                |
       |            (API Gateway & Module Resolvers)            |
       +---------------------------+----------------------------+
                                   |
                             GORM / DB Queries
                                   |
                                   v
                       [ SQLite / Postgres DB ]
```

### Key Architectural Concepts
1.  **Zero Local Database Dependencies**: The local TUI client executable does not connect to Postgres, SQLite, or run any GORM migrations. It holds no local database handles.
2.  **Statically Linked Composition**: View models and interface elements are defined inside domain vertical modules (`commerce/product/tui.go`, `commerce/order/tui.go`, etc.). When compiled, Go’s static linker compiles these views into a single, self-contained executable.
3.  **Standalone Client Mode**: When run with the `--server` CLI flag or `HYPERRR_SERVER` environment variable, the TUI loads these views statically, using the global registry to build the tabs, and bypasses local backend/DB bootstrap.

---

## 2. Dynamic Tab Sorting & Navigation

The TUI shell container (`internal/tui/app.go`) scans the registry for modules implementing the `TUIProvider` interface:

```go
type TUIProvider interface {
    TUIPages() []TUIPage
}
```

To ensure a stable, predictable layout when Go’s randomized map iteration returns modules, the container sorts active modules alphabetically by their unique ID:
1.  **Customers** (`commerce.customer`)
2.  **Orders** (`commerce.order`)
3.  **Products** (`commerce.product`)
4.  **Workflows** (`core.context`)

Users navigate the tabs using number keys `1-9`, `Tab` / `Shift+Tab`, or horizontal Arrow keys. Focus is delegated directly to the active sub-page view for inputs or scrolling.

---

## 3. Bubble Tea v2 Specification

The TUI utilizes **Bubble Tea v2** (`charm.land/bubbletea/v2`) and **Lip Gloss v2** (`charm.land/lipgloss/v2`) to provide a declarative view rendering cycle.

### v2 Event Loop
Key presses are captured as `tea.KeyPressMsg` instead of the legacy `tea.KeyMsg` structs. Keystrokes are inspected using standard string matching on `msg.String()`:

```go
func (p *productPage) Update(msg any) (registry.TUIPage, any) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        switch msg.String() {
        case "j", "down":
            p.activeRow = (p.activeRow + 1) % len(p.products)
        case "esc":
            p.mode = 0 // List mode
        }
    }
    return p, nil
}
```

---

## 4. GraphQL Network Data Layer

When running in decoupled client mode, the TUI fetches lists and submits form mutations via standard HTTP POST calls to the server's `/query` endpoint.

```go
// QueryGraphQL executes a remote GraphQL request over HTTP.
func QueryGraphQL(serverURL string, query string, variables map[string]any, out any) error {
    reqBody, _ := json.Marshal(GraphQLRequest{
        Query:     query,
        Variables: variables,
	})
    resp, err := http.Post(serverURL+"/query", "application/json", bytes.NewBuffer(reqBody))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    var gqlResp GraphQLResponse
    _ = json.NewDecoder(resp.Body).Decode(&gqlResp)
    return json.Unmarshal(gqlResp.Data, out)
}
```

*   **List Views**: Fetched using `listProducts`, `listOrders`, `listLineages`, and `listCustomers` queries.
*   **Write Submissions**: Submitted using mutations like `createProduct` or `updateProduct` when completing forms.

---

## 5. Standalone Execution

The CLI utility compiles into a single binary and can be run without source files or config databases:

```bash
# Compile standalone command center
go build -o bin/mission-control ./cmd/tui

# Execute pointing to the remote server URL
./bin/mission-control --server http://api.hyperrr-commerce.com:8080
```
