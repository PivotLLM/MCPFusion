# Google Workspace Integration Status

## Current State

### Completed

- **OAuth scopes reconciled** — Server config and fusion-auth helper now use identical scopes
- **Draft email management** — 5 endpoints: create, get, update, delete, list (no send)
- **Contacts via People API** — 3 endpoints: list, get, search (using per-endpoint baseURL override)
- **Token invalidation** — Configured for 401 and 403 status codes
- **`gmail.send` removed** — Replaced with `gmail.compose` (drafts only, no sending)
- **Per-endpoint baseURL override** — Go code change to support endpoints with different base URLs (used by contacts)
- **Test scripts** — Full test coverage in `tests/Google/`
- **`oauth2_external` auth type** — New auth strategy for externally-provided tokens (via fusion-auth). Supports token refresh with client_secret. Returns clear error when no token exists.
- **Service name mismatch fix** — Added `ServiceKey` to `ServiceConfig`, handler now uses config map key for token lookups instead of display name
- **Auth middleware fix** — Fixed `nil` auth middleware being passed to `NewExtendedTransport`, enabling fusion-auth `/ping` and `/api/` endpoints
- **Google Workspace setup guide** — Comprehensive docs at `docs/Google_Workspace.md`

### Authentication Architecture

Google restricts OAuth2 device flow scopes, so a helper app (`fusion-auth`) was built to handle browser-based authorization code flow instead. The server config uses `oauth2_external` auth type which relies on stored tokens from fusion-auth and supports automatic token refresh.

**Intended flow:**
1. User runs `fusion-auth` to authenticate via browser (auth code flow + PKCE)
2. `fusion-auth` obtains tokens and pushes them to MCPFusion server
3. MCPFusion uses stored tokens for API calls
4. MCPFusion automatically refreshes expired tokens using the refresh token

**Key files:**
- `configs/google-workspace.json` — endpoint definitions, `oauth2_external` auth type
- `fusion/auth_strategy_oauth2_external.go` — OAuth2 external strategy implementation
- `cmd/auth/main.go` — fusion-auth entry point
- `cmd/auth/providers/google/provider.go` — Google provider (auth code flow + PKCE)
- `docs/Google_Workspace.md` — setup guide

### OAuth Scopes (server config + fusion-auth)

```
https://www.googleapis.com/auth/userinfo.email
https://www.googleapis.com/auth/userinfo.profile
https://www.googleapis.com/auth/calendar
https://www.googleapis.com/auth/gmail.readonly
https://www.googleapis.com/auth/gmail.compose
https://www.googleapis.com/auth/drive
https://www.googleapis.com/auth/contacts.readonly
```

### Endpoints (22 total)

**Profile:** get
**Calendar:** list, create, get, update, delete
**Gmail read:** list, get, search
**Gmail drafts:** create, get, update, delete, list
**Drive:** list, get, download, create, delete, share
**Contacts:** list, get, search

## Remaining Work

### 1. Gmail Body Format Transformer

Gmail API requires RFC 2822 base64url-encoded messages for creating/updating drafts and messages. The current config uses structured body params (to, subject, body, cc, bcc) which is consistent but doesn't match the actual Gmail API format. A Go body transformer is needed to convert structured params to RFC 2822 format at request time.

This affects: `gmail_draft_create`, `gmail_draft_update`

### 2. Verify Token Refresh

Confirm automatic token refresh works when Google access tokens expire (1 hour TTL).

### 3. Update Google Integration Tests

`fusion/google_integration_test.go` references the removed `gmail_message_send` endpoint. Should be updated to test draft endpoints instead.
