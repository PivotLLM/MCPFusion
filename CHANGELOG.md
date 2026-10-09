# Changelog

Breaking changes to exported identifiers, signatures or behaviour, for the
applications that import MCPFusion packages.

## Unreleased

### Identity and behaviour

- Removed `global.AppName` and `global.AppVersion`. The application identity
  now lives in the `app` package; use `app.Name()`, `app.Version()` (display)
  or `app.SemVer()` (protocol handshakes and comparisons).
- `-debug` now defaults to false. Deployments that relied on debug logging
  must pass `-debug` (for example in `mcpfusion.service`).
- `mcpserver.MCPServer.Start()` now returns an error when the listen address
  cannot be opened, instead of running with nothing listening.
- The auth helper is built as `fusion-auth` (was `fusion-oauth`) and sends
  User-Agent `fusion-auth/<version>`.
- Removed `fusion.InMemoryCache` (deprecated, empty).

### Error strings

Callers that match on the exact text must be updated:

- `hub service '%s' is currently unavailable. The server will automatically reconnect`
  → `hub service '%s' is unavailable (reconnecting automatically)`
- `this tool performs a destructive operation and is currently disabled. Set the MCP_FUSION_ALLOW_DESTRUCTIVE environment variable to 'true' to enable destructive tools`
  → `destructive operations are disabled (set MCP_FUSION_ALLOW_DESTRUCTIVE=true to allow)`
- `no stored token found for this service. Please run fusion-auth to authenticate.`
  (`OAuth2ExternalStrategy.Authenticate`) → `no stored token for this service (authenticate with fusion-auth)`
- `no user ID associated with this API key — link with: mcpfusion -user-link <user_id>:<key_hash>`
  → `no user ID linked to this API key (link with mcpfusion -user-link <user_id>:<key_hash>)`

### `fusion.New`

- `fusion.New(options ...Option) *Fusion` → `fusion.New(options ...Option) (*Fusion, error)`.
  An auth manager without a cache now returns an error instead of panicking.

### Logger is now a `WithLogger` option

- Each of `fusion`, `hub` and `mcpserver` has one `WithLogger(global.Logger)`,
  returning a `LoggerOption` accepted by the package's main constructor and its
  other constructors. With no logger (or nil) components log nothing;
  `mcpserver.New` still requires one.
- New exported types: `fusion.ComponentOption`, `fusion.LoggerOption`,
  `hub.ClientOption`, `hub.LoggerOption`, `mcpserver.TransportOption`,
  `mcpserver.LoggerOption`.
- `fusion.Option`, `hub.HubOption` and `mcpserver.Option` are now interfaces
  instead of function types. Options built from raw functions no longer
  compile; the `With…` functions are unchanged.
- `fusion.WithLogger` and `mcpserver.WithLogger` now return `LoggerOption`.
- **fusion** (replace a positional `logger` with a trailing `fusion.WithLogger(logger)`):
  - `NewCommandExecutor(opts ...ComponentOption)`
  - `NewMultiTenantAuthManager(database TokenStore, cache Cache, opts ...ComponentOption)`
  - `NewOAuth2DeviceFlowStrategy(httpClient *http.Client, opts ...ComponentOption)`
  - `NewBearerTokenStrategy(opts ...ComponentOption)`
  - `NewAPIKeyStrategy(opts ...ComponentOption)`
  - `NewBasicAuthStrategy(opts ...ComponentOption)`
  - `NewSessionJWTStrategy(httpClient *http.Client, opts ...ComponentOption)`
  - `NewUserCredentialsStrategy(opts ...ComponentOption)`
  - `NewOAuth2ExternalStrategy(httpClient *http.Client, opts ...ComponentOption)`
  - `NewDatabaseCache(database TokenStore, opts ...ComponentOption)`
  - `NewDatabaseCacheWithDefaultTTL(database TokenStore, defaultTTL time.Duration, opts ...ComponentOption)` (TTL moves before the options)
  - `NewDataStoreTokenStore(ds toolspec.DataStore, opts ...ComponentOption)`
  - `NewMetricsCollector(enabled bool, opts ...ComponentOption)` (was `(logger, enabled)`)
  - `NewTimeTokenProcessor(opts ...ComponentOption)`
  - `NewValidator(opts ...ComponentOption)`
  - `NewMapper(opts ...ComponentOption)`
  - `NewRetryExecutor(config *RetryConfig, opts ...ComponentOption)`
  - `NewCircuitBreaker(config *CircuitBreakerConfig, opts ...ComponentOption)`
- **hub:**
  - `NewSSEClient(config *fusion.ServiceConfig, opts ...ClientOption)`
  - `NewHTTPClient(config *fusion.ServiceConfig, opts ...ClientOption)`
  - `NewStdioClient(config *fusion.ServiceConfig, opts ...ClientOption)`
  - `NewMCPClientManager(serviceName string, opts ...ClientOption)`
  - `NewHubProvider(configs map[string]*fusion.ServiceConfig, opts ...HubOption)` (logger via `hub.WithLogger`)
- **mcpserver:**
  - `NewAuthenticatedTransport(underlying http.Handler, middleware func(http.Handler) http.Handler, opts ...TransportOption)`
    (was `underlying MCPServerTransport` with a positional logger; it can no longer return nil)
  - `NewExtendedTransport(sseTransport, httpTransport MCPServerTransport, oauthEngine OAuthRouteProvider, authMiddleware func(http.Handler) http.Handler, opts ...TransportOption)`
  - New methods `(*AuthenticatedTransport).Serve(net.Listener) error` and
    `(*ExtendedTransport).Serve(net.Listener) error`.

### Removed `Get` prefixes

**config**
- `Manager.GetService` → `Service`
- `Manager.GetAllServices` → `Services`
- `Manager.GetServiceNames` → `ServiceNames`
- `Manager.GetAvailableServices` → `AvailableServices`
- `Manager.GetServiceAuthConfig` → `ServiceAuthConfig`
- `Manager.GetConfig` → `Config`
- `Manager.GetCommand` → `Command`
- `Manager.GetAllCommands` → `Commands`
- `Manager.GetCommandGroupNames` → `CommandGroupNames`

**db** (the same renames apply to the `db.Database` interface)
- `DB.GetUser` → `LoadUser`
- `DB.GetUserByAPIKey` → `LookupUserByAPIKey`
- `DB.GetOAuthToken` → `LoadOAuthToken`
- `DB.GetKnowledge` → `LoadKnowledge`
- `DB.GetTenantInfo` → `LoadTenantInfo`
- `DB.GetTenantResourceCount` → `CountTenantResources`
- `DB.GetCredentials` → `LoadCredentials`
- `DB.GetCredentialsByType` → `LoadCredentialsByType`
- `DB.GetAPITokenMetadata` → `LoadAPITokenMetadata`

**fusion**
- `APIError.GetCategory` removed; use the `Category` field
- `NetworkError.GetRetryAfter` → `RetryDelay`
- `GetBodyEncoder` → `LookupBodyEncoder`
- `AuthenticationError`, `ConfigurationError`, `ValidationError`: `GetUserFriendlyMessage` → `UserFriendlyMessage`
- `AuthStrategy.GetAuthType` → `Type` (interface and all seven strategies)
- `TokenInfo.GetAuthorizationHeader` → `AuthorizationHeader`
- `MetricsCollector`: `GetServiceMetrics` → `ServiceMetrics`, `GetAllMetrics` → `AllMetrics`, `GetGlobalMetrics` → `GlobalMetrics`, `GetErrorRate` → `ErrorRate`
- `MultiTenantAuthManager`: `GetUserCredentials` → `LoadUserCredentials`, `GetToken` → `AcquireToken`, `GetRegisteredStrategies` → `RegisteredStrategies`, `GetTenantTokens` → `ListTenantTokens`
- `DatabaseCache.GetStats` → `Stats`
- `TimeTokenProcessor.GetSupportedTokens` → `SupportedTokens`
- `GetMCPParameterName` → `MCPParameterName`
- `ParameterNameMapper`: `GetOriginalName` → `OriginalName`, `GetMCPName` → `MCPName`
- `Fusion`: `GetConfig` → `Config`, `GetHTTPClient` → `HTTPClient`, `GetCache` → `Cache`, `GetLogger` → `Logger`, `GetServiceNames` → `ServiceNames`, `GetService` → `Service`, `GetEndpoint` → `Endpoint`, `GetCircuitBreakerMetrics` → `CircuitBreakerMetrics`, `GetAllCircuitBreakerMetrics` → `AllCircuitBreakerMetrics`, `GetMetrics` → `Metrics`, `GetServiceMetrics` → `ServiceMetrics`, `GetGlobalMetrics` → `GlobalMetrics`, `GetCircuitBreakerSource` → `CircuitBreakerSource`
- `MultiTenantFusion`: `GetFusionForTenant` → `FusionForTenant`, `GetResource` → `ReadResource`, `GetPrompt` → `RenderPrompt`, `GetAuthManager` → `AuthManager`, `GetDatabaseCache` → `DatabaseCache`, `GetStats` → `Stats`
- `TokenStore`: `GetOAuthToken` → `LoadOAuthToken`, `GetCredentials` → `LoadCredentials` (interface and implementations)
- `CircuitBreaker`: `GetState` → `State`, `GetMetrics` → `Metrics`
- `AuthConfig.GetEffectiveTokenInvalidationConfig` → `EffectiveTokenInvalidationConfig`
- `ServiceConfig`: `GetEndpointByID` → `EndpointByID`, `GetEffectiveCircuitBreakerConfig` → `EffectiveCircuitBreakerConfig`
- `EndpointConfig`: `GetRequiredParameters` → `RequiredParameters`, `GetParameterByName` → `ParameterByName`, `GetEffectiveRetryConfig` → `EffectiveRetryConfig`
- `ParameterConfig.GetTransformedParameterName` → `TransformedParameterName`
- `Config`: `GetServiceByName` → `ServiceByName`, `GetAllEndpoints` → `AllEndpoints`, `GetRequiredEnvironmentVariables` → `RequiredEnvironmentVariables`
- `WithConfigManager` now takes `interface{ Config() *Config }`

**hub**
- `MCPClientManager.GetCachedTools` → `CachedTools`

**mcpserver**
- `MCPServer.GetMCPServer` → `Server`
- `ServiceProvider`: `GetAvailableServices` → `AvailableServices`, `GetService` → `Service`, `GetServiceAuthConfig` → `ServiceAuthConfig`

**metrics**
- `Collector`: `GetServiceStats` → `ServiceStats`, `GetAllServiceStats` → `AllServiceStats`, `GetUptime` → `Uptime`

**providers/health**
- `CircuitBreakerSource.GetAllCircuitBreakerMetrics` → `AllCircuitBreakerMetrics`
