# Database Architecture and Schema Auto-Migrations

Hyperrr leverages GORM (Go Object-Relational Mapper) as its relational database connection framework (`pkg/db`). The database subsystem is designed to handle multiple dialects, register schemas dynamically as modules load, and support transaction idempotency.

---

## 1. Pluggable Database Dialects

Hyperrr natively supports two relational database backends, controlled via standard configuration variables (`DB_DRIVER` and `DB_DSN`):

```go
switch cfg.DBDriver {
case "sqlite":
	// Resolves sqlite file path inside the .hyperrr/ folder at the project root
	dialect = sqlite.Open(dsn)
case "postgres":
	// Establishes PostgreSQL connection
	dialect = postgres.Open(cfg.DBDSN)
}
```

### SQLite Workspace Convenience
If SQLite is selected with a relative filename (e.g. `hyperrr.db`), the driver automatically resolves the project root, creates a `.hyperrr` subdirectory, and writes the database file there. This keeps the workspace root clean while keeping dev state segregated.

---

## 2. Dynamic Model Registration and Auto-Migrations

To ensure that the core boot loader has no hardcoded references to module-specific database tables, Hyperrr utilizes a global migration registry (`pkg/db/migrate.go`):

```go
var (
	registryMu sync.Mutex
	Registry   = []interface{}{&IdempotencyKey{}} // Core models pre-registered
)

func Register(models ...interface{}) {
	registryMu.Lock()
	defer registryMu.Unlock()
	Registry = append(Registry, models...)
}
```

### Lifecycle Execution Flow
1. **Model Registration**: As the boot loader (`internal/app/app.go`) iterates over the enabled modules, it fetches the list of GORM-mapped schemas from each module:
   ```go
   if models := mod.Models(); len(models) > 0 {
       db.Register(models...)
   }
   ```
2. **Auto-Migration execution**: Right after all active modules are registered, the engine initiates migrations:
   ```go
   if err := database.AutoMigrateAll(); err != nil {
       return fmt.Errorf("failed to run migrations: %w", err)
   }
   ```
GORM evaluates the registered model structs against the existing database tables, automatically creating tables, creating indexes, and altering columns to match the code definitions without dropping columns or data.

---

## 3. Idempotency Key Engine

To prevent double-billing and duplicate transactions (e.g. if an agent submits the same checkout tool call multiple times due to a timeout), the core database layer pre-registers the `IdempotencyKey` schema:

```go
type IdempotencyKey struct {
	ID            string    `gorm:"primaryKey"`
	Key           string    `gorm:"uniqueIndex:idx_key_owner"`
	Owner         string    `gorm:"uniqueIndex:idx_key_owner"`
	LockedAt      time.Time
	CompletedAt   *time.Time
	ResponseCode  int
	ResponseBody  []byte
}
```

### Execution Flow
1. When a transaction workflow is started with an idempotency key, the engine inserts the key to the database with `LockedAt` set.
2. If the database returns a duplicate key conflict error (indicating the key is already in progress or completed), the runner blocks the execution.
3. If it is already completed, the runner returns the cached `ResponseBody` directly to the client without running the workflow logic a second time.

---

## 4. Database Modularity and Isolation

Because Hyperrr is built to allow decoupling domain modules (such as moving the `commerce` package out of the repository), modules must adhere to **Logical Database Isolation**:

*   **Soft Relations**: Do not declare foreign key relationships across database tables owned by different modules (e.g. `order` model containing GORM pointers to `product.Product`). Use flat fields (like `ProductID string`) to store soft relations.
*   **Encapsulation**: Modules must only read/write from tables they own. If the `billing` module needs customer data, it must query the `commerce.customer` module APIs rather than performing a cross-table SQL query against the customer table directly.
