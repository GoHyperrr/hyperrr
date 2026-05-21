# Tasks for #3: Event Fabric Interface & In-Memory Provider

Parent issue: #3
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define `Event` and `EventBus` Interfaces ✅
**Status**: Completed  
**Output**: `pkg/eventbus/eventbus.go`.  
**Implementation**: Defined the core `Event` struct and the `EventBus` interface. Designed for extensibility to NATS/Kafka.

### 2. Implement Thread-Safe `InMemBus` ✅
**Status**: Completed  
**Output**: `pkg/eventbus/inmem.go`.  
**Implementation**: Developed an in-memory event bus using Go channels and mutexes, supporting asynchronous at-least-once delivery.

### 3. Test `InMemBus` Concurrency and Pub/Sub ✅
**Status**: Completed  
**Output**: `pkg/eventbus/inmem_test.go`.  
**Implementation**: Verified multiple subscribers, concurrent publishers, and edge cases (closed bus), achieving 95% coverage.
