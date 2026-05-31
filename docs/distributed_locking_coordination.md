# Distributed Locking and Coordination Kernel

To guarantee transaction safety and prevent race conditions (such as double booking a hotel room or selling out-of-stock products), Hyperrr implements a pluggable **Distributed Locking Core** (`pkg/locking`). 

Every concurrent step execution inside the Workflow Runner utilizes this coordination layer to lock critical entities during mutation.

---

## 1. Pluggable Locker Interface

The core locking kernel exposes a simple, thread-safe interface:

```go
type Locker interface {
	// Acquire attempts to acquire a lock for the given key.
	// ttl is the maximum time the lock can be held before auto-expiring.
	// timeout is the maximum time to wait for the lock to become available.
	Acquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (bool, error)
	
	// Release releases the lock for the given key.
	Release(ctx context.Context, key string) error
	
	// Close shuts down the locker.
	Close() error
}
```

---

## 2. Lock Providers and Implementation Details

Hyperrr supports three locking drivers, configured dynamically at boot via `WORKFLOW_STORE_TYPE`:

### 2.1 In-Memory Locker (`InMemLocker`)
- Uses standard Go maps and `sync.Mutex` locks.
- Uses timers to implement TTL expiry.
- Ideal for single-node monoliths and fast local unit tests.

### 2.2 NATS JetStream Locker (`NATSLocker`)
- Utilizes a NATS JetStream KV bucket.
- Uses **Compare-and-Swap (CAS)** operations: it reads the KV version of the lock key and only updates it if the version hasn't changed.
- If the lock is held, it retries in a loop up to the specified `timeout`.

### 2.3 Redis Locker (`RedisLocker`)
- Utilizes Redis cache store.
- Employs a `SET key value NX PX ttl` command structure.
- Employs Redis Lua scripts to execute release locks atomically.

---

## 3. Lock Ownership and Thread-Safety

A major hazard in distributed locking is **delayed release**. If worker A acquires a lock with a 2-second TTL, gets stalled by a GC pause for 3 seconds, the lock auto-expires, and worker B acquires it. When worker A resumes, it calls `Release()`. If unchecked, worker A might release worker B's lock, exposing the system to a race condition.

### The Owner-Tracking Pattern
Hyperrr prevents this by enforcing **Lock Ownership**:

1. **Unique Owner Identification**: When a lock is acquired, the locker generates a unique session/goroutine identifier (e.g. UUID) and stores it in the database/KV bucket value under that key.
2. **Context Binding**: The identifier is stored in the Go `context.Context` using `LockOwnerKey`:
   ```go
   ctx = context.WithValue(ctx, LockOwnerKey, ownerID)
   ```
3. **Owner Validation**: During release, the locker queries the KV store or Redis. It runs a comparison script to confirm that the current value of the lock matches the owner ID stored in the context. If it does not match, it returns `ErrLockNotHeld` and prevents releasing a lock owned by another process.
