# Proposal: per-user credentials for `session_jwt`

Status: implemented (see "Per-User Credentials" in `docs/config.md`). Kept as the design record for `configs/unifyem.json`.

## Problem

`session_jwt` logs in to an upstream API with a username and password and applies the returned token to every request. Today the login body can only be filled from environment variables, so every MCPFusion tenant shares one upstream identity.

UnifyEM has no long-lived API tokens. Administrators hold a username and password, trade them for an access token (default life 12 hours) and re-authenticate when it expires. We want each MCPFusion user to log in to UnifyEM as themselves, so UnifyEM's audit log records the real administrator, and we want MCPFusion to collect those credentials the same way `user_credentials` does: the `<service>_auth_setup` tool hands the user a `fusion-auth` command, `fusion-auth` prompts for the values and stores them against the user's MCPFusion API token.

`user_credentials` alone does not fit because it forwards stored values verbatim and cannot perform the login exchange or renew the resulting token.

This is not a new auth type. It is an optional extension of `session_jwt`; configs without the new block behave exactly as today.

## Proposed behaviour

`session_jwt` gains an optional `credentials` block. When present:

- the login URL, login headers, JSON body or form body may reference per-user values with `{{credentials.<field>}}` placeholders;
- the values are collected per tenant through the existing `fusion-auth` flow;
- what is persisted depends on `credentials.store` (see below): either the prompted values, or only the token obtained by exchanging them once;
- the resulting token is cached per tenant and applied exactly as `session_jwt` applies it today;
- if a tenant has nothing stored, the tool call fails with the same "credentials required" error `user_credentials` produces, so the LLM calls `<service>_auth_setup`.

### Universality

Nothing about the login exchange is UnifyEM-specific. `session_jwt` already takes from config: `loginURL`, `loginMethod`, `loginContentType`, `loginBody` or `loginFormBody`, `tokenPath`, `tokenType`, `tokenLocation` with `headerName`/`headerFormat`, `cookieName`/`cookieFormat` or `queryParam`, `expiresIn` or `expiresInPath`, and the optional refresh settings. This proposal changes only where the values in that exchange come from. Any API whose login is "send credentials, receive a token" works, including APIs that expect the username in the URL path or a header, because placeholders are accepted there too. A new `loginHeaders` map is added for the header case.

### Storage modes: `credentials.store`

| Value | What is stored per tenant | On token expiry or 401 | Use when |
|---|---|---|---|
| `"credentials"` (default) | The prompted field values. | MCPFusion logs in again with the stored values. No user action. | The upstream only issues short-lived tokens (UnifyEM today). |
| `"token"` | Only the token returned by performing the login once, with its expiry and any refresh token. The prompted values are discarded after the exchange. | MCPFusion refreshes if the upstream supports it; otherwise the tenant is sent back through `<service>_auth_setup` to re-enter credentials. | The upstream can issue a long-lived or personal token at login (UnifyEM if it adds that). Passwords are never at rest. |

In `"token"` mode the exchange runs inside MCPFusion at storage time, not in `fusion-auth`, so the strategy code is not duplicated in the CLI and the prompted values exist only for the duration of that one request. Asking the upstream for a long-lived token is ordinary config: a static field in `loginBody` (for example `"token_type": "api"`) or a different `loginURL`, whichever the API requires. If the upstream also returns a refresh token, set `refreshTokenPath` and the refresh settings and MCPFusion renews without user involvement for as long as the refresh token lives.

### Config shape

```json
"auth": {
  "type": "session_jwt",
  "config": {
    "loginURL": "/api/v1/login",
    "loginMethod": "POST",
    "loginContentType": "application/json",
    "loginBody": {
      "username": "{{credentials.username}}",
      "password": "{{credentials.password}}"
    },
    "tokenPath": "access_token",
    "tokenType": "Bearer",
    "tokenLocation": "header",
    "headerName": "Authorization",
    "headerFormat": "Bearer {token}",
    "expiresIn": 3600,
    "credentials": {
      "store": "credentials",
      "instructions": "Shown by fusion-auth before prompting.",
      "fields": [
        { "name": "username", "label": "UnifyEM username", "description": "..." },
        { "name": "password", "label": "UnifyEM password", "description": "...", "secret": true }
      ]
    }
  },
  "tokenInvalidation": { "statusCodes": [401], "retryOnInvalidation": true }
}
```

Switching UnifyEM to a future long-lived token needs only `"store": "token"`, whatever field or URL UnifyEM defines for requesting one, and an `expiresIn`/`expiresInPath` that matches.

`{{credentials.x}}` is deliberately different from `${ENV}` so the config manager's environment expansion leaves it alone. Placeholders are allowed in `loginURL`, `loginHeaders` values, and `loginBody` / `loginFormBody` values. `secret: true` is new and only affects prompting.

## Changes by component

### `fusion/config.go`

- Validation for `session_jwt`: if `credentials` is present, require a non-empty `fields` array where every field has a `name`, and `store` to be absent, `"credentials"` or `"token"`. Reject `{{credentials.x}}` placeholders that name a field not in `fields`.
- Add `loginHeaders` (map of header name to value) to the `session_jwt` config, applied on the login request; placeholders and `${ENV}` both allowed.
- Add `Secret bool` to the field definition type shared with `user_credentials` (harmless there).

### `fusion/multi_tenant_auth.go`

- Per-tenant credentials are stored through the `StoreCredentials` / `LoadCredentials` / `DeleteCredentials` methods that `TokenStore` already declares and that both the bbolt (`db.DB`) and DataStore (`dataStoreTokenStore`) backends already implement, using `db.ServiceCredentials` with type `custom` and the field values in `Data`. Nothing in `fusion` used these methods before. The record is separate from the OAuth token record, so the existing `InvalidateToken` (which only deletes the token record) leaves credentials intact. Only used when `store` is `"credentials"`.
- The `DatabaseCache` that `AcquireToken` consults is itself backed by the same token store, so "cache the token" and "persist the token" are one operation (`CacheToken`).
- `AcquireToken`, `store: "credentials"`: when no valid cached token exists, load the tenant's stored credentials before calling `Authenticate`. If none exist, return an `AuthenticationError` whose message names the `<service>_auth_setup` tool, mirroring the `user_credentials` behaviour. Pass the credentials to the strategy through a reserved key in a copy of the config map (`__credentials`); `HTTPHandler.prepareAuthConfig` already copies the map per request to inject `baseURL`, so this follows an existing pattern and avoids changing the `AuthStrategy` interface for every strategy. The copy holding credentials exists only for the `Authenticate` call.
- `AcquireToken`, `store: "token"`: no change to the existing path. The cached token is the stored token; when it is expired and refresh fails or is not configured, return the credentials-required error.
- `InvalidateToken`: continues to clear only the cached session token. Add `InvalidateCredentials` for use by `auth_setup`, which clears both so a fresh `fusion-auth` run replaces the stored values.

### `fusion/auth_strategies.go` (`SessionJWTStrategy`)

- `Authenticate`: before building the login request, substitute `{{credentials.<name>}}` in `loginURL`, `loginHeaders`, `loginBody` and `loginFormBody` from `config["__credentials"]`. Fail with a clear error if a placeholder has no value. Never log the substituted URL, headers or body.
- Keep the credentials out of `TokenInfo.Metadata`; only token location and format metadata belong there.

### `fusion/oauth_api.go`

- `handleServiceConfig`: already returns `instructions` and `fields` from the auth config when present, but for `session_jwt` they live under `config.credentials`. Return `credentials.instructions` and `credentials.fields` as the top-level `instructions` and `fields` so `fusion-auth` needs no knowledge of the nesting.
- `handleOAuthTokens` (the endpoint `fusion-auth` posts to): when the service is `session_jwt` with `credentials`, branch on `store`. For `"credentials"`, persist the posted field values in the credentials store. For `"token"`, run `SessionJWTStrategy.Authenticate` with the posted values immediately, persist the resulting `TokenInfo` as the tenant's cached token, discard the values, and return the login error to `fusion-auth` if the exchange fails so the user sees it at the prompt rather than on the next tool call.

### `fusion/auth_setup.go` and `fusion/fusion.go`

- Register `<service>_auth_setup` for `session_jwt` services that declare `credentials` (today only `oauth2_external` and `user_credentials` get one).
- In the handler, treat `session_jwt` with `credentials` like `user_credentials`: clear stored credentials and token, then return the `fusion-auth` command with the `instructions` text.

### `cmd/auth` (`fusion-auth`)

- Today `executeOAuthFlow` routes to the credentials prompt only when `auth_type` is `user_credentials`; any other type falls through to the local OAuth provider registry and fails for `session_jwt`. Run the credentials prompt flow whenever the service config carries `fields`, regardless of `auth_type`, and post the values with the sentinel access token the server expects for credentials. `fusion-auth` does not need to know which `store` mode is in effect; the server decides.
- Mask input for fields with `secret: true` using `golang.org/x/term.ReadPassword` (new dependency of the `cmd/auth` module, which is a separate Go module).
- Surface a failed exchange (token mode) as an error at the prompt.

### Optional: `refreshBody` for `session_jwt`

`RefreshToken` currently sends no body, so UnifyEM's `POST /api/v1/refresh`, which expects `{"refresh_token": "..."}`, cannot be used. In credentials mode re-login makes refresh unnecessary. In token mode refresh is the only unattended renewal path, so this becomes worthwhile: add `refreshBody` with a `{refreshToken}` placeholder and `refreshTokenPath` to capture the refresh token from the login response.

## Tests

- Unit: placeholder substitution in URL, headers, JSON and form bodies, including missing-field errors and no leakage of credentials into logs or `TokenInfo`.
- Unit, credentials mode: `AcquireToken` with stored credentials performs login and caches the token; without them returns the credentials-required error; invalidation keeps the credentials and the next call logs in again.
- Unit, token mode: posting values performs the exchange and stores only the token; the credentials store stays empty; an expired token with no refresh returns the credentials-required error; a failed exchange is reported to the caller and nothing is stored.
- Unit: config validation accepts and rejects the shapes above.
- Live (per repository rules): a `tests/UnifyEM/` script that calls `unifyem_ping` before and after `fusion-auth`, and one that forces a 401 by shortening `access_token_life` on a test server and confirms the retry succeeds.

## Security notes

- In credentials mode, passwords are stored at rest in MCPFusion's bbolt database alongside OAuth refresh tokens. They inherit the same protections and the same exposure. The docs should say so. Token mode avoids this and should be preferred whenever the upstream can issue a long-lived token.
- Credentials must never appear in debug logs. The login body is currently logged only as "prepared"; keep it that way after substitution.
- The `fusion-auth` code expires in 15 minutes, as today.
