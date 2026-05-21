# Tasks for #12: OS-Level Authentication & JWT

Parent issue: #12
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Extend Identity Module with Passwords ✅
**Status**: Completed  
**Output**: `internal/identity/model.go`.  
**Implementation**: Added `PasswordHash` field to the `User` model with GORM `not null` constraint and JSON ignore tag.

### 2. Implement Auth Package: Token Management ✅
**Status**: Completed  
**Output**: `internal/auth/jwt.go`.  
**Implementation**: Built a dedicated `auth` package using `golang-jwt/jwt/v5`. Implemented secure signing and validation of actors.

### 3. Implement Login and Registration Logic ✅
**Status**: Completed  
**Output**: `internal/identity/handlers.go`.  
**Implementation**: Developed `Login` and `Register` methods. Registration now emits an `identity.user_created` event, which the `commerce.customer` module listens to for automatic profile creation.

### 4. Implement Auth Middleware ✅
**Status**: Completed  
**Output**: `api/middleware/auth.go`.  
**Implementation**: Created a standard HTTP middleware that extracts JWTs from the `Authorization` header and injects validated actors into the Go `context`.

### 5. Define Auth GraphQL Schema & Mutations ✅
**Status**: Completed  
**Output**: `internal/identity/identity.graphqls`.  
**Implementation**: Added `login` and `register` mutations to the schema. Wired them through the unified API layer.

### 6. Exhaustive Testing for Auth Flow ✅
**Status**: Completed  
**Output**: `tests/auth_test.go`.  
**Implementation**: Verified the end-to-end registration, login, and protected query flow. Achieved 90%+ total logic coverage.
