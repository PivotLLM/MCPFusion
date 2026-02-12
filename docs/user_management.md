# User & Knowledge Management

MCPFusion provides stable user identity and persistent knowledge storage. Users are decoupled from API keys, allowing multiple keys to share the same identity and knowledge base.

## Table of Contents

- [User Management](#user-management)
- [Knowledge Store](#knowledge-store)
- [Database Storage](#database-storage)

## User Management

### Overview

Each user in MCPFusion has a UUID, a description, and one or more linked API keys. Knowledge entries are stored per-user rather than per-API-key, so a user retains their stored knowledge even when API keys are rotated or additional keys are added.

### Auto-Migration

On server startup, any API tokens that are not yet linked to a user are automatically assigned to newly created user accounts. This provides backward compatibility with existing deployments -- upgrading to a version with user management requires no manual intervention.

### CLI Commands

| Command | Description | Example |
|---------|-------------|---------|
| `-user-add "desc"` | Create a new user | `./mcpfusion -user-add "Alice"` |
| `-user-list` | List all users and linked keys | `./mcpfusion -user-list` |
| `-user-delete ID` | Delete a user (with confirmation) | `./mcpfusion -user-delete abc123` |
| `-user-link ID:HASH` | Link an API key to a user | `./mcpfusion -user-link abc123:def456` |
| `-user-unlink HASH` | Unlink an API key from its user | `./mcpfusion -user-unlink def456` |

When a user is deleted, all associated knowledge entries are also removed. Unlinking a key does not delete the user or their knowledge.

## Knowledge Store

### Overview

The knowledge store provides persistent, per-user storage organized by domain and key. AI clients can store user preferences, rules, and context that persists across sessions. Knowledge tools are exposed as native MCP tools, so any connected client can read and write entries without additional configuration.

### MCP Tools

Three MCP tools are available:

**`knowledge_set`** -- Store or update a knowledge entry.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `domain` | Yes | Category or namespace (e.g., `email`, `calendar`) |
| `key` | Yes | Identifier within the domain (e.g., `newsletter-rules`) |
| `content` | Yes | The knowledge content to store |

**`knowledge_get`** -- Retrieve knowledge entries. Supports three retrieval modes depending on which parameters are provided:

| Parameter | Required | Description |
|-----------|----------|-------------|
| `domain` | No | Return all entries in this domain |
| `key` | No | Combined with `domain`, return a specific entry |

- Provide `domain` and `key` to retrieve a specific entry.
- Provide `domain` alone to retrieve all entries in that domain.
- Omit both to retrieve all entries across all domains.

**`knowledge_delete`** -- Delete a knowledge entry by domain and key.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `domain` | Yes | Domain of the entry to delete |
| `key` | Yes | Key of the entry to delete |

### Domain/Key Organization

Domains group related knowledge entries, and keys identify individual entries within a domain. Choose domain and key names that reflect the purpose of the stored content.

Examples:

| Domain | Key | Content |
|--------|-----|---------|
| `email` | `newsletter-rules` | User prefers newsletters to be archived |
| `email` | `signature` | Regards, Alice |
| `calendar` | `scheduling-preferences` | Prefer morning meetings, no Fridays |
| `writing` | `tone` | Use a professional but approachable tone |

### Requirements

Knowledge tools require:

- **Database**: The embedded BoltDB database must be available (this is the default).
- **User linkage**: The API key used for the request must be linked to a user account.
- **Authentication**: Requests must include a valid `Authorization: Bearer <token>` header.

### Example Tool Calls

Store a knowledge entry:

```json
{
  "tool": "knowledge_set",
  "args": {
    "domain": "email",
    "key": "newsletter-rules",
    "content": "Archive newsletters automatically unless they mention a current project."
  }
}
```

Retrieve a specific entry:

```json
{
  "tool": "knowledge_get",
  "args": {
    "domain": "email",
    "key": "newsletter-rules"
  }
}
```

Retrieve all entries in a domain:

```json
{
  "tool": "knowledge_get",
  "args": {
    "domain": "email"
  }
}
```

Retrieve all knowledge entries:

```json
{
  "tool": "knowledge_get",
  "args": {}
}
```

Delete an entry:

```json
{
  "tool": "knowledge_delete",
  "args": {
    "domain": "email",
    "key": "newsletter-rules"
  }
}
```

## Database Storage

Knowledge entries are stored in the embedded BoltDB database under the path `users/{user_id}/knowledge/{domain}/{key}`.

Each entry stores the following fields:

| Field | Description |
|-------|-------------|
| `domain` | The domain namespace |
| `key` | The key within the domain |
| `content` | The stored knowledge content |
| `created_at` | Timestamp of initial creation |
| `updated_at` | Timestamp of the most recent update |

When an entry is updated via `knowledge_set`, the `created_at` timestamp is preserved and only `updated_at` is refreshed. When an entry is deleted and its domain bucket becomes empty, the empty bucket is automatically cleaned up.
