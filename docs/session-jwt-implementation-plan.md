# Session JWT Authentication Strategy - Implementation Plan

## Overview

Add a new authentication strategy `session_jwt` to MCPFusion that supports login-based JWT authentication with flexible token application (headers, cookies, or query parameters).

## Use Case

APIs like PwnDoc that require:
1. POST credentials to a login endpoint
2. Extract JWT from JSON response
3. Apply token as a cookie (not Authorization header)
4. Support token refresh via separate endpoint

## Files to Modify

### 1. `fusion/config.go`

**Add constant** (after line 28):
```go
AuthTypeSessionJWT AuthType = "session_jwt"
```

**Add validation** (in `ValidateWithLogger` around line 611):
```go
case AuthTypeSessionJWT:
    if _, ok := a.Config["loginURL"]; !ok {
        return fmt.Errorf("session_jwt auth requires loginURL")
    }
    if _, ok := a.Config["tokenPath"]; !ok {
        return fmt.Errorf("session_jwt auth requires tokenPath")
    }
    if _, ok := a.Config["tokenLocation"]; !ok {
        return fmt.Errorf("session_jwt auth requires tokenLocation")
    }
```

### 2. `fusion/auth_strategies.go`

**Add new strategy** (~300 lines):

```go
// SessionJWTStrategy implements session-based JWT authentication
type SessionJWTStrategy struct {
    httpClient *http.Client
    logger     global.Logger
}

// Configuration fields:
// - loginURL: Login endpoint path (required)
// - loginMethod: HTTP method (default: POST)
// - loginContentType: Content-Type (default: application/json)
// - loginBody: Request body as map (optional, for JSON)
// - loginFormBody: Request body as map (optional, for form-urlencoded)
// - username: Username value or ${ENV_VAR}
// - password: Password value or ${ENV_VAR}
// - tokenPath: Dot-notation path to token in response (required)
// - tokenType: Token type prefix (default: Bearer)
// - tokenLocation: Where to apply token: header, cookie, query (required)
// - headerName: Header name if location=header (default: Authorization)
// - headerFormat: Header value format (default: "{tokenType} {token}")
// - cookieName: Cookie name if location=cookie
// - cookieFormat: Cookie value format (default: "{token}")
// - queryParam: Query parameter name if location=query
// - expiresIn: Default expiration in seconds
// - expiresInPath: Path to expiration in response
// - refreshURL: Token refresh endpoint (optional)
// - refreshMethod: Refresh HTTP method (default: POST)
// - refreshTokenPath: Path to refresh token in login response
// - refreshTokenLocation: Where refresh token is: body, cookie
// - refreshTokenCookieName: Cookie name for refresh token
```

**Key methods:**
- `NewSessionJWTStrategy(httpClient, logger)` - Constructor
- `GetAuthType()` - Returns `AuthTypeSessionJWT`
- `SupportsRefresh()` - Returns true if refreshURL configured
- `Authenticate(ctx, config)` - Performs login, extracts token
- `RefreshToken(ctx, tokenInfo, config)` - Refreshes token
- `ApplyAuth(req, tokenInfo)` - Applies token to request (header/cookie/query)

**Helper methods:**
- `extractValueByPath(data, path)` - Extract value using dot notation
- `buildLoginRequest(ctx, config)` - Build login HTTP request
- `parseLoginResponse(body, config)` - Parse response and extract token

### 3. `fusion/multi_tenant_fusion.go`

**Register strategy** (in `registerDefaultAuthStrategies`, after line 270):
```go
// Register session JWT strategy
sessionJWTStrategy := NewSessionJWTStrategy(mtf.httpClient, mtf.logger)
mtf.authManager.RegisterStrategy(sessionJWTStrategy)
```

### 4. `main.go`

**Register strategy** (after line 231):
```go
// Register session JWT strategy
sessionJWTStrategy := fusion.NewSessionJWTStrategy(httpClient, logger)
multiTenantAuth.RegisterStrategy(sessionJWTStrategy)
```

## Configuration Example (PwnDoc)

```json
{
  "services": {
    "pwndoc": {
      "name": "PwnDoc",
      "baseURL": "${PWNDOC_URL}",
      "auth": {
        "type": "session_jwt",
        "config": {
          "loginURL": "/api/users/token",
          "loginMethod": "POST",
          "loginContentType": "application/json",
          "loginBody": {
            "username": "${PWNDOC_USERNAME}",
            "password": "${PWNDOC_PASSWORD}"
          },
          "tokenPath": "datas.token",
          "tokenLocation": "cookie",
          "cookieName": "token",
          "cookieFormat": "JWT {token}",
          "expiresIn": 3600,
          "refreshURL": "/api/users/refreshtoken",
          "refreshMethod": "POST",
          "refreshTokenLocation": "cookie",
          "refreshTokenCookieName": "refreshToken"
        }
      },
      "endpoints": [...]
    }
  }
}
```

## Implementation Order

1. Add `AuthTypeSessionJWT` constant to `config.go`
2. Add validation rules for session_jwt config
3. Create `SessionJWTStrategy` struct and constructor
4. Implement `GetAuthType()` and `SupportsRefresh()`
5. Implement helper functions for path extraction
6. Implement `Authenticate()` method
7. Implement `ApplyAuth()` method with header/cookie/query support
8. Implement `RefreshToken()` method
9. Register strategy in both `multi_tenant_fusion.go` and `main.go`
10. Build and verify compilation
11. Test with PwnDoc configuration

## Testing

1. Build MCPFusion: `go build -o mcpfusion .`
2. Create test config with PwnDoc service
3. Start server and verify authentication works
4. Verify token refresh works after expiration

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Cookie handling complexity | Use standard `http.Cookie` struct |
| Token path extraction errors | Provide clear error messages |
| Refresh token in cookies | Parse Set-Cookie headers from response |
