# Tasks for #9: Core OS Extensions: Identity & Object Storage

Parent issue: #9
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Implement Identity Module: Models & Security Boundary ✅
**Status**: Completed  
**Output**: `internal/identity/model.go`.  
**Implementation**: Defined `Actor`, `User`, and `APIKey` models with GORM mappings. Established the security boundary by distinguishing between Human, AI Agent, and System actors.

### 2. Implement Identity Handlers & Middleware ✅
**Status**: Completed  
**Output**: `internal/identity/handlers.go`.  
**Implementation**: Developed the `identity.validate_actor` task handler for workflows and helper methods for API key resolution.

### 3. Implement Storage Module: Interface & Local Provider ✅
**Status**: Completed  
**Output**: `internal/storage/local.go`.  
**Implementation**: Created the `ObjectStorage` interface and a robust `LocalProvider` that handles file uploads, retrieval, and deletion on the local disk.

### 4. Implement Storage Module: S3/Cloud Abstraction ✅
**Status**: Completed  
**Output**: `internal/storage/s3.go`.  
**Implementation**: Implemented an `S3Provider` stub that follows the interface, ready for future AWS/GCS integration via configuration.

### 5. Register Internal Modules & Exhaustive Testing ✅
**Status**: Completed  
**Implementation**: Wired both modules into the `registry` and `internal/app`. Achieved high logic coverage for all new components.
