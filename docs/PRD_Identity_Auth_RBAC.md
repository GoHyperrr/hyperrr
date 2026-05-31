# PRD: Unified Identity, Auth & RBAC

## Problem Statement

The Hyperrr OS currently has a basic identity system that supports user registration and JWT-based authentication. however, it lacks several critical production-grade security features:
1.  **API Key Management**: No way to generate, list, or revoke API keys for AI agents or external integrations.
2.  **Granular Access Control (RBAC)**: All authenticated actors currently have the same level of access. There is no way to restrict an AI agent to only "Read-Only" tasks or a warehouse worker to only "Fulfillment" tasks.
3.  **Disconnected Auth Flow**: The authentication middleware only handles JWTs, while the MCP server manually handles API keys, leading to inconsistent security enforcement.

## Solution

We will implement a unified, industrially hardened security layer that provides a single source of truth for identities and their permissions.

Key components:
1.  **API Key Lifecycle**: Extend the `identity` module to allow authorized users to generate, name, and revoke API keys.
2.  **Unified Auth Middleware**: A single kernel-level middleware that transparently handles both `Authorization: Bearer <JWT>` and `X-API-Key: <KEY>`, injecting a standardized `Actor` into the context.
3.  **Flexible RBAC**: A Role-Based Access Control system where `Actors` (Human or AI) can be assigned `Roles`. Each `Role` contains a list of `Permissions` (e.g., `order:read`, `product:write`, `workflow:execute:*`).
4.  **Workflow Guards**: Integrate RBAC checks directly into the Workflow Engine. Workflows can define required permissions, and the `Runner` will automatically enforce them based on the acting `Actor`.

## User Stories

1.  **As a Developer**, I want to register a user and receive a JWT, so I can access protected GraphQL resources.
2.  **As an Administrator**, I want to generate a unique API Key for an AI Agent, so it can interact with the system autonomously via MCP or API.
3.  **As an Administrator**, I want to revoke an API Key immediately if an agent misbehaves, so that the system remains secure.
4.  **As a Platform Operator**, I want to assign the "Auditor" role to a specific agent, so it can read order history but cannot trigger refunds or create products.
5.  **As a Commerce Developer**, I want to protect a specific workflow by defining a required permission (e.g., `inventory:manage`), so that only authorized actors can trigger it.
6.  **As a Security Auditor**, I want all actions (human or agentic) to be logged with their associated Actor ID and Role, so that we have a perfect audit trail of who did what and why.

## Implementation Decisions

*   **Middleware Unification**: The `auth` module will provide a `UnifiedValidator` that checks both tokens and API keys against the database/blacklist.
*   **RBAC Schema**:
    *   `Role`: Name, Description.
    *   `Permission`: Resource (e.g., `order`), Action (e.g., `write`).
    *   `ActorRole`: Join table connecting `Actor` to `Role`.
*   **Workflow Integration**: We will add a `RequiredPermissions []string` field to the `Workflow` struct. The `Runner` will check these permissions before starting execution.
*   **JWT Claims**: Include assigned roles in the JWT claims to allow for fast, database-free permission checks in the middleware.

## Module Design

### 1. `modules/identity` (Enhanced)
*   **Responsibility**: Manage Users, Actors, API Keys, Roles, and Permissions.
*   **New Models**: `Role`, `Permission`, `ActorRole`.
*   **New Handlers**: `GenerateAPIKey`, `RevokeAPIKey`, `AssignRole`, `CheckPermission`.
*   **Interface**: Implements `registry.ActorResolver`.

### 2. `modules/auth` (Enhanced)
*   **Responsibility**: Token issuance and validation.
*   **Interface**: Implements `middleware.TokenValidator`.
*   **Changes**: Update `GenerateToken` to include roles/permissions in the claims.

### 3. `internal/workflow` (Security Upgrade)
*   **Responsibility**: Enforce RBAC during DAG execution.
*   **Changes**: The `Execute` method will check the `Actor` in the context against the `Workflow.RequiredPermissions`.

## Testing Decisions

*   **Auth Flow Test**: Complete end-to-end test of registration -> login -> token-based request -> restricted resource access.
*   **RBAC Tests**: Verify that an Actor with "Read" role fails when trying to execute a "Write" workflow.
*   **API Key Revocation Test**: Verify that a request with a revoked API key is rejected immediately.

## Out of Scope

*   OAuth2/OIDC integration (we will stick to our internal JWT/API Key provider for now).
*   Fine-grained "Attribute Based Access Control" (ABAC) (e.g., "Only allow if order price < $100"). We will use standard RBAC.

## Open Questions

*   **Default Roles**: Should we seed the system with "admin", "user", and "agent" roles by default?
    *   *Resolution*: Yes, common roles should be seeded during the initial migration.
*   **Permission String Format**: Should we use colon-delimited strings (e.g., `product:create`) or separate fields?
    *   *Resolution*: Use colon-delimited strings for flexibility.

## Further Notes

This refactor transforms the security layer from a simple "Is logged in?" check to a professional-grade "What are they allowed to do?" system, which is essential for multi-agent coordination.
