# Tasks for #2: Modular Database Abstraction & GORM Setup

Parent issue: #2
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Install GORM and Dialects ✅
**Status**: Completed  
**Implementation**: Installed `gorm.io/gorm` along with `postgres` and a CGO-free `sqlite` driver (`github.com/glebarez/sqlite`) for Windows compatibility.

### 2. Implement Core DB Abstraction ✅
**Status**: Completed  
**Output**: `pkg/db/db.go`.  
**Implementation**: Created a `Connect` function that returns a wrapped GORM DB handle. Supports swappable dialects based on configuration.

### 3. Define Modular Migration Interface ✅
**Status**: Completed  
**Output**: `pkg/db/migrate.go`.  
**Implementation**: Implemented a global `Registry` and `AutoMigrateAll` function. Modules can now register their models independently, ensuring strict schema isolation.

### 4. Test Database Connectivity (SQLite & Postgres) ✅
**Status**: Completed  
**Output**: `pkg/db/db_test.go`.  
**Implementation**: Verified successful connections to SQLite and confirmed Postgres dialect logic paths.

### 5. Exhaustive Testing for Repository Pattern Base ✅
**Status**: Completed  
**Output**: `pkg/db/repo_test.go`.  
**Implementation**: Demonstrated CRUD operations across two isolated "modules" (User/Order) using soft relationships (external IDs), achieving 93%+ coverage for the persistence layer.
