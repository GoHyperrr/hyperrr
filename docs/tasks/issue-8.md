# Tasks for #8: Plugin System & Module Registry

Parent issue: #8
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define the Module Interface ✅
**Status**: Completed  
**Output**: `pkg/registry/module.go`.  
**Implementation**: Defined a comprehensive `Module` interface that includes `ID()`, `Init()`, `Models()`, and `Handlers()`. This ensures all plugins adhere to a common contract for discovery and initialization.

### 2. Implement the Global Registry ✅
**Status**: Completed  
**Output**: `pkg/registry/registry.go`.  
**Implementation**: Built a thread-safe registry using a mutex-protected map. Provides `Register`, `List`, and `Get` methods for dynamic module management.

### 3. Implement Dependencies Struct ✅
**Status**: Completed  
**Output**: `pkg/registry/module.go` (integrated with interface).  
**Implementation**: Established a `Dependencies` container that injects the shared `DB`, `EventBus`, and `Runner` into modules during their initialization phase, preventing tight coupling to globals.

### 4. Refactor internal/app to Use Registry ✅
**Status**: Completed  
**Output**: `internal/app/app.go`.  
**Implementation**: Overhauled the application startup sequence. It now automatically discovers registered modules, performs their database migrations, registers their workflow task handlers, and executes their custom initialization logic.

### 5. Exhaustive Testing for Plugin System ✅
**Status**: Completed  
**Output**: `pkg/registry/registry_test.go`.  
**Implementation**: Verified the entire plugin lifecycle—from registration to dependency injection—using mock modules. Achieved high logic coverage across the new registry packages.
