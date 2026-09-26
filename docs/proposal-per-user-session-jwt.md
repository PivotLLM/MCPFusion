# Proposal: per-user credentials for `session_jwt`

Status: proposal, not implemented. Needed by `configs/unifyem.json`.

## Problem

`session_jwt` logs in to an upstream API with a username and password and applies the returned token to every request. Today the login body can only be filled from environment variables, so every MCPFusion tenant shares one upstream identity.

UnifyEM has no long-lived API tokens. Administrators hold a username and password, trade them for an access token (default life 12 hours) and re-authenticate when it expires. We want each MCPFusion user to log in to UnifyEM as themselves, so UnifyEM's audit log records the real administrator, and we want MCPFusion to collect those credentials the same way `user_credentials` does: the `<service>_auth_setup` tool hands the user a `fusion-auth` command, `fusion-auth` prompts for the values and stores them against the user's MCPFusion API token.

`user_credentials` alone does not fit because it forwards stored values verbatim and cannot perform the login exchange or renew the resulting token.

## Proposed behaviour

`session_jwt` gains an optional `credentials` block. When present:

- the login body may reference stored per-user values with `{{credentials.<field>}}` placeholders;
- the credentials are collected per tenant through the existing `fusion-auth` flow and stored per tenant;
- the login is performed per tenant with that tenant's credentials, and the resulting token is cached per tenant;
- when the token expires, or the upstream returns a status listed in `tokenInvalidation.statusCodes`, MCPFusion logs in again with the stored credentials without user involvement;
- if a tenant has no stored credentials, the tool call fails with the same "credentials required" error `user_credentials` produces, so the LLM calls `<service>_auth_setup`.

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

`{{credentials.x}}` is deliberately different from `${ENV}` so the config manager's environment expansion leaves it alone. Placeholders are allowed in `loginBody` and `loginFormBody` values. `secret: true` is new and only affects prompting.

## Changes by component

### `fusion/config.go`

- Validation for `session_jwt`: if `credentials` is present, require a non-empty `fields` array where every field has a `name`. Reject `{{credentials.x}}` placeholders that name a field not in `fields`.
- Add `Secret bool` to the field definition type shared with `user_credentials` (harmless there).

### `fusion/multi_tenant_auth.go`

- New storage for per-tenant credentials, separate from the session token, so invalidating the token never discards the credentials. Suggested: store them through the existing token store under the service name plus a fixed suffix (for example `unifyem#credentials`), or add `StoreCredentials` / `GetCredentials` / `DeleteCredentials` to the datastore interface keyed by tenant hash and service. Values are `map[string]string`.
- `GetToken`: when the auth config has `credentials`, load the tenant's stored credentials before calling `Authenticate`. If none exist, return the credentials-required error that `user_credentials` uses today. Pass the credentials to the strategy through a reserved key in the config map copy (`__credentials`), which avoids changing the `AuthStrategy` interface for every strategy.
- `InvalidateToken`: continues to clear only the cached session token. Add `InvalidateCredentials` for use by `auth_setup`, which should clear both so a fresh `fusion-auth` run replaces the stored values.

### `fusion/auth_strategies.go` (`SessionJWTStrategy`)

- `Authenticate`: before building the login body, substitute `{{credentials.<name>}}` in `loginBody` and `loginFormBody` values from `config["__credentials"]`. Fail with a clear error if a placeholder has no value. Never log the substituted body.
- Keep the credentials out of `TokenInfo.Metadata`; only token location and format metadata belong there.

### `fusion/auth_setup.go` and `fusion/fusion.go`

- Register `<service>_auth_setup` for `session_jwt` services that declare `credentials` (today only `oauth2_external` and `user_credentials` get one).
- In the handler, treat `session_jwt` with `credentials` like `user_credentials`: clear stored credentials and token, then return the `fusion-auth` command with the `instructions` text.

### `fusion/oauth_api.go` (`handleServiceConfig`)

- Already returns `instructions` and `fields` from the auth config when present, but for `session_jwt` they live under `config.credentials`. Return `credentials.instructions` and `credentials.fields` as the top-level `instructions` and `fields` so `fusion-auth` needs no knowledge of the nesting.

### `cmd/auth` (`fusion-auth`)

- Run the credentials prompt flow whenever the service config carries `fields`, regardless of `auth_type`, and store the values with the sentinel access token the server expects for credentials.
- Mask input for fields with `secret: true` using `golang.org/x/term.ReadPassword` (new dependency).

### Optional: `refreshBody` for `session_jwt`

`RefreshToken` currently sends no body, so UnifyEM's `POST /api/v1/refresh`, which expects `{"refresh_token": "..."}`, cannot be used. Re-login with stored credentials makes refresh unnecessary for UnifyEM, so this is not required. If wanted later: add `refreshBody` with a `{refreshToken}` placeholder and `refreshTokenPath` to capture the refresh token from the login response.

## Tests

- Unit: placeholder substitution in JSON and form bodies, including missing-field errors and no leakage of credentials into logs or `TokenInfo`.
- Unit: `GetToken` with stored credentials performs login and caches the token; without them returns the credentials-required error; invalidation keeps the credentials and the next call logs in again.
- Unit: config validation accepts and rejects the shapes above.
- Live (per repository rules): a `tests/UnifyEM/` script that calls `unifyem_ping` before and after `fusion-auth`, and one that forces a 401 by shortening `access_token_life` on a test server and confirms the retry succeeds.

## Security notes

- Passwords are stored at rest in MCPFusion's bbolt database alongside OAuth refresh tokens. They inherit the same protections and the same exposure. The docs should say so.
- Credentials must never appear in debug logs. The login body is currently logged only as "prepared"; keep it that way after substitution.
- The `fusion-auth` code expires in 15 minutes, as today.
