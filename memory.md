# MCPFusion Knowledge Store — Implementation Plan

## Vision

MCPFusion needs a persistent knowledge store that allows LLM clients to remember user-specific context across sessions and apply it when processing emails, calendar events, and other data. This knowledge is maintained by the LLM on behalf of the user — for example, "when Dymon notifies me about a package, always ask if they placed it in my Mini 20" — and must be accessible to any MCP-compatible client without vendor lock-in. The knowledge store should support multiple domains (email, calendar, contacts, and future integrations) within a single flexible structure, using natural language content that any LLM can read and act on without requiring a domain-specific schema.

## Problem: Tenant Identity Tied to API Key

Currently, the only tenant identifier is the SHA-256 hash of the API token. If a user rotates their API key, all associated data (OAuth tokens, credentials, and future knowledge entries) is orphaned under the old hash. For transient data like OAuth tokens, this is tolerable — the user simply re-authenticates. For knowledge data that builds up over weeks and months, losing it on key rotation is unacceptable.

## Phase 1: Stable User Identity

Introduce a **user ID** that is independent of API keys. API keys are then associated with the user ID and can be rotated freely without affecting underlying data.

### Database Changes

Add new BoltDB buckets:

```
users/
  {user_id}/
    metadata        -> UserMetadata JSON (created_at, description, etc.)
    api_keys/
      {key_hash}    -> reference back to api_tokens bucket
    knowledge/
      {domain}/
        {key}       -> KnowledgeEntry JSON
```

Add a reverse index for fast lookup during request auth:

```
key_to_user/
  {key_hash}        -> user_id
```

### Data Model

```go
type UserMetadata struct {
    UserID      string    // UUID, immutable after creation
    Description string    // Human-readable label
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Migration Path

- Existing API tokens that are not associated with a user get auto-migrated: create a user ID and link the existing key hash to it.
- Existing OAuth tokens and credentials remain under the tenant hash bucket (no change to auth flow). Long term, these could migrate under the user bucket, but that's not required for Phase 1.

### CLI Commands

```bash
./mcpfusion -user-add "Eric's account"          # Create user, returns user ID
./mcpfusion -user-list                           # List all users
./mcpfusion -user-delete <user_id>               # Delete user and all data
./mcpfusion -user-link <user_id> <key_hash>      # Link existing API key to user
./mcpfusion -user-unlink <key_hash>              # Unlink API key from user
```

### Auth Flow Change

When a request arrives with `Authorization: Bearer <token>`:

1. Hash the token → `key_hash`
2. Validate `key_hash` in `api_tokens` bucket (unchanged)
3. **New**: Look up `key_to_user[key_hash]` → `user_id`
4. Set both `TenantHash` and `UserID` on `TenantContext`

The `TenantHash` continues to scope OAuth tokens and credentials (no migration needed). The `UserID` scopes knowledge data and any future per-user persistent state.

## Phase 2: Knowledge Store

### Data Model

```go
type KnowledgeEntry struct {
    Domain    string    // e.g., "email", "calendar", "contacts", "general"
    Key       string    // e.g., "dymon-packages", "weekly-standup", "boss-preferences"
    Content   string    // Natural language or lightly structured text
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Content is intentionally unstructured — the LLM interprets it contextually. Examples:

- **Domain**: `email`, **Key**: `dymon-packages`, **Content**: "When Dymon Self Storage sends a package notification, always ask the user whether the courier placed the package in their Mini 20 unit."
- **Domain**: `calendar`, **Key**: `meeting-preferences`, **Content**: "The user prefers 30-minute meetings. Default to Eastern time zone. Always include a Teams link."
- **Domain**: `general`, **Key**: `communication-style`, **Content**: "The user prefers concise responses. Use bullet points when listing items."

### BoltDB Storage

Knowledge entries live under the user's bucket:

```
users/
  {user_id}/
    knowledge/
      email/
        dymon-packages   -> KnowledgeEntry JSON
        newsletter-rules -> KnowledgeEntry JSON
      calendar/
        meeting-prefs    -> KnowledgeEntry JSON
      general/
        comm-style       -> KnowledgeEntry JSON
```

### MCP Tools

Three tools registered as native MCP tools (not HTTP endpoint tools):

#### `knowledge_set`
- **Parameters**: `domain` (string, required), `key` (string, required), `content` (string, required)
- **Behavior**: Creates or updates a knowledge entry. Returns confirmation.
- **Auth**: Requires valid API key. Scoped to the user ID associated with the key.

#### `knowledge_get`
- **Parameters**: `domain` (string, optional), `key` (string, optional)
- **Behavior**:
  - Both provided: return the single matching entry
  - Domain only: return all entries in that domain
  - Neither: return all entries across all domains
- **Auth**: Same as above.

#### `knowledge_delete`
- **Parameters**: `domain` (string, required), `key` (string, required)
- **Behavior**: Deletes the entry. Returns confirmation or "not found".
- **Auth**: Same as above.

### Tool Registration

These are **internal tools** — they don't come from JSON config files. They are registered alongside the config-driven HTTP tools in `Fusion.RegisterTools()`. The handler extracts `UserID` from the `TenantContext` to scope all operations.

### LLM Usage Pattern

The LLM is responsible for:

1. **Querying knowledge** at appropriate moments — e.g., when reading emails, call `knowledge_get(domain="email")` to retrieve relevant rules.
2. **Storing knowledge** when the user says "remember this" or expresses a preference — call `knowledge_set` with a descriptive domain/key and natural language content.
3. **Applying knowledge** to subsequent operations — include relevant entries in its reasoning context.

MCPFusion provides no MCP prompts or instructions telling the LLM *when* to query knowledge. The LLM's tool descriptions should be sufficient for it to understand the use case.

## Phase 3: Future Considerations

These are out of scope for the initial implementation but worth noting:

- **Knowledge search**: A `knowledge_search` tool with keyword/fuzzy matching across content fields, for when the LLM doesn't know the exact domain/key.
- **Knowledge export/import**: CLI commands to dump and restore a user's knowledge as JSON, useful for backup or migration.
- **Bulk knowledge retrieval via MCP resources**: Expose all knowledge for a domain as an MCP resource (read-only), allowing LLMs to pull context in bulk without multiple tool calls.
- **TTL/expiry**: Optional expiration on entries for time-sensitive knowledge.
- **Migrate OAuth/credentials under user ID**: Move all per-tenant data under the user bucket so key rotation is fully transparent for everything, not just knowledge.

## Implementation Order

1. **Phase 1a**: `UserMetadata` struct, `users/` and `key_to_user/` buckets, CRUD operations in `db/` package
2. **Phase 1b**: CLI commands (`-user-add`, `-user-list`, `-user-delete`, `-user-link`, `-user-unlink`)
3. **Phase 1c**: Auto-migration of existing API keys to user IDs on startup
4. **Phase 1d**: Update `TenantContext` with `UserID` field, update auth flow to populate it
5. **Phase 2a**: `KnowledgeEntry` struct, knowledge CRUD operations in `db/` package
6. **Phase 2b**: Register `knowledge_set`, `knowledge_get`, `knowledge_delete` as native MCP tools
7. **Phase 2c**: Integration testing — full lifecycle via MCP calls

## Files Affected

| File | Change |
|------|--------|
| `db/types.go` | Add `UserMetadata`, `KnowledgeEntry` structs |
| `db/users.go` | **NEW** — User CRUD, key linking, migration |
| `db/knowledge.go` | **NEW** — Knowledge CRUD operations |
| `db/internal/buckets.go` | Add `users`, `key_to_user` bucket constants |
| `db/users_test.go` | **NEW** — User management tests |
| `db/knowledge_test.go` | **NEW** — Knowledge store tests |
| `main.go` | Add `-user-*` CLI flags |
| `fusion/fusion.go` | Register knowledge tools in `RegisterTools()` |
| `fusion/knowledge_handler.go` | **NEW** — MCP tool handlers for knowledge_set/get/delete |
| `fusion/multi_tenant_auth.go` | Add `UserID` to `TenantContext`, populate in auth flow |
| `global/interfaces.go` | Add `UserID` field if `TenantContext` is defined here |
| `docs/knowledge.md` | **NEW** — Knowledge store documentation |
