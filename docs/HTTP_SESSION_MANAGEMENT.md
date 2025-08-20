# HTTP Session Management in MCPFusion

MCPFusion includes advanced HTTP session management features designed to prevent connection timeouts, handle unreliable network conditions, and improve overall reliability when working with external APIs.

## Table of Contents

- [Overview](#overview)
- [Default Transport Configuration](#default-transport-configuration)
- [Per-Endpoint Connection Control](#per-endpoint-connection-control)
- [Automatic Health Management](#automatic-health-management)
- [Configuration Examples](#configuration-examples)
- [Monitoring and Debugging](#monitoring-and-debugging)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

MCPFusion addresses common HTTP connection issues through:

1. **Optimized Connection Pooling**: Intelligent limits and timeouts to prevent stale connections
2. **Per-Endpoint Control**: Fine-grained connection management for problematic APIs
3. **Automatic Health Management**: Background cleanup and error-triggered maintenance
4. **Connection Monitoring**: Comprehensive logging and debugging capabilities

## Default Transport Configuration

MCPFusion uses optimized HTTP transport settings out of the box:

### Connection Pool Settings
- **Maximum Total Idle Connections**: 100
- **Maximum Idle Connections Per Host**: 10
- **Maximum Connections Per Host**: 50
- **Idle Connection Timeout**: 30 seconds

### Timeout Configuration
- **Connection Establishment**: 10 seconds
- **TLS Handshake**: 10 seconds
- **Response Headers**: 30 seconds
- **Overall Request**: 60 seconds (increased from default 30s)
- **Keep-Alive Probes**: 30 seconds

### Health Management
- **Automatic Cleanup**: Every 5 minutes
- **Error-Triggered Cleanup**: On timeout and connection errors
- **Graceful Shutdown**: Proper resource cleanup on termination

## Per-Endpoint Connection Control

Configure connection behavior for specific endpoints that experience reliability issues.

### Configuration Structure

Add a `connection` object to any endpoint configuration:

```json
{
  "id": "endpoint_id",
  "name": "Endpoint Name",
  "method": "GET",
  "path": "/api/endpoint",
  "connection": {
    "disableKeepAlive": false,
    "forceNewConnection": false,
    "timeout": "60s"
  },
  "parameters": [...]
}
```

### Connection Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `disableKeepAlive` | boolean | `false` | Forces connection closure after each request by adding `Connection: close` header |
| `forceNewConnection` | boolean | `false` | Creates a new HTTP client with disabled connection pooling for each request |
| `timeout` | string | `"60s"` | Custom timeout for this endpoint (format: "30s", "2m", "1h") |

## Automatic Health Management

### Background Cleanup

MCPFusion runs a background goroutine that:
- Executes every 5 minutes
- Calls `transport.CloseIdleConnections()` to remove stale connections
- Logs cleanup activities for monitoring

### Error-Triggered Cleanup

Automatic cleanup is triggered when these errors are detected:
- Timeout errors (containing "timeout" or "deadline")
- Connection errors (containing "connection" or "network")

### Manual Cleanup

Use the `ForceConnectionCleanup()` method programmatically:
```go
fusionProvider.ForceConnectionCleanup()
```

### Graceful Shutdown

Proper resource cleanup on application termination:
```go
fusionProvider.Shutdown()
```

## Configuration Examples

### Example 1: Microsoft 365 Mail Search
For APIs that frequently timeout or reset connections:

```json
{
  "id": "microsoft365_mail_search",
  "name": "Search Microsoft 365 emails",
  "method": "GET",
  "path": "/me/messages",
  "connection": {
    "disableKeepAlive": true,
    "timeout": "45s"
  },
  "parameters": [
    {
      "name": "$filter",
      "alias": "filter",
      "description": "OData filter expression",
      "type": "string",
      "required": false,
      "location": "query"
    }
  ]
}
```

### Example 2: Long-Running Operations
For endpoints that may take longer than the default timeout:

```json
{
  "id": "large_file_download",
  "name": "Download large file",
  "method": "GET",
  "path": "/files/{fileId}/content",
  "connection": {
    "timeout": "5m"
  },
  "parameters": [...]
}
```

### Example 3: Authentication-Sensitive Endpoints
For APIs that require fresh connections for authentication:

```json
{
  "id": "oauth_token_refresh",
  "name": "Refresh OAuth token",
  "method": "POST",
  "path": "/oauth/token",
  "connection": {
    "forceNewConnection": true,
    "disableKeepAlive": true
  },
  "parameters": [...]
}
```

## Monitoring and Debugging

### Debug Logging

Enable debug mode to see connection management activities:

```bash
./mcpfusion -debug
```

### Log Messages

Monitor these log messages to understand connection health:

```
[DEBUG] Timeout detected, triggering connection cleanup [correlation-id]
[DEBUG] Connection error detected, triggering connection cleanup [correlation-id]
[DEBUG] Cleaned up idle HTTP connections
[INFO] Forcing connection pool cleanup
[DEBUG] Connection health management started
[DEBUG] Connection health management shutting down
```

### Connection Control Logging

When connection control options are applied:

```
[DEBUG] Disabling keep-alive for request [correlation-id]
[DEBUG] Forcing new connection for request [correlation-id]
[DEBUG] Using custom timeout 45s for request [correlation-id]
[WARNING] Invalid timeout format 'invalid' for endpoint endpoint_id [correlation-id]
```

## Best Practices

### When to Use Connection Control

**Use `disableKeepAlive: true` when:**
- Experiencing frequent "connection reset" errors
- API servers close connections unpredictably
- Working with APIs that have strict connection limits
- Debugging connection-related authentication issues

**Use `forceNewConnection: true` when:**
- Connection reuse causes authentication problems
- API requires fresh connections for security
- Debugging complex connection issues
- Working with load balancers that don't handle keep-alive well

**Use custom `timeout` when:**
- Endpoint typically takes longer than 60 seconds
- API documentation specifies different timeout requirements
- Dealing with large file uploads/downloads
- Working in environments with known slow network conditions

### Configuration Guidelines

1. **Start Conservative**: Begin with `disableKeepAlive: true` for problematic endpoints
2. **Monitor Logs**: Watch for connection-related errors and cleanup messages
3. **Gradual Optimization**: Remove restrictions as stability improves
4. **Test Thoroughly**: Verify that connection settings don't impact performance
5. **Document Decisions**: Comment configuration choices for future maintenance

### Performance Considerations

- `disableKeepAlive` adds minimal overhead (just an HTTP header)
- `forceNewConnection` creates new HTTP clients and may impact performance
- Custom timeouts should be realistic for the operation being performed
- Monitor connection pool exhaustion in high-traffic scenarios

## Troubleshooting

### Common Issues

#### "connection reset by peer"
**Solution**: Add `disableKeepAlive: true` to force fresh connections

#### "context deadline exceeded"
**Solutions**:
1. Increase timeout: `"timeout": "90s"`
2. Check if API is experiencing issues
3. Monitor for rate limiting (HTTP 429 responses)

#### "no such host" or DNS errors
**Solutions**:
1. Verify network connectivity
2. Check firewall settings
3. Confirm API endpoint URLs are correct

#### High connection count
**Solutions**:
1. Reduce `MaxIdleConns` and `MaxIdleConnsPerHost` if needed
2. Monitor for connection leaks
3. Ensure proper connection cleanup

### Debug Steps

1. **Enable Debug Logging**:
   ```bash
   ./mcpfusion -debug
   ```

2. **Monitor Connection Cleanup**:
   Look for automatic cleanup messages in logs

3. **Test Connection Settings**:
   Try progressively more restrictive settings:
   ```json
   // Step 1: Custom timeout
   {"timeout": "45s"}
   
   // Step 2: Disable keep-alive
   {"disableKeepAlive": true}
   
   // Step 3: Force new connections
   {"forceNewConnection": true}
   ```

4. **Check API Status**:
   Verify the external API is functioning normally

5. **Network Diagnostics**:
   Use tools like `curl` or `telnet` to test basic connectivity

### Performance Monitoring

Monitor these metrics:
- Request latency and success rates
- Connection pool utilization
- Frequency of connection cleanup events
- Error rates by endpoint

## Advanced Configuration

### Custom Transport Settings

For advanced users, the default transport settings can be modified in the source code (`fusion/fusion.go`):

```go
transport := &http.Transport{
    MaxIdleConns:          100,              // Adjust based on load
    MaxIdleConnsPerHost:   10,               // Increase for high-volume APIs
    IdleConnTimeout:       30 * time.Second, // Reduce for faster cleanup
    MaxConnsPerHost:       50,               // Increase for concurrent requests
    // ... other settings
}
```

### Environment-Specific Tuning

Consider different settings for different environments:

**Development**: More logging, shorter timeouts for faster feedback
**Staging**: Production-like settings with enhanced monitoring
**Production**: Optimized for reliability and performance

---

## Related Documentation

- [Configuration Guide](config.md) - General endpoint configuration
- [Microsoft 365 Setup](Microsoft365.md) - Specific Microsoft API setup
- [Troubleshooting Guide](config.md#troubleshooting) - General troubleshooting

---

*Last updated: January 2025*