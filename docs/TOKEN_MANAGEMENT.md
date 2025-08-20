# MCPFusion Token Management Guide

This guide explains how to manage API tokens for MCPFusion's multi-tenant authentication system.

## Overview

MCPFusion uses a multi-tenant token system where each API token represents a unique namespace. Each tenant can have independent OAuth tokens and service credentials for various services (Microsoft365, Google, etc.).

## Prerequisites

- MCPFusion server installed
- Access to the `mcpfusion-token` CLI tool
- Appropriate permissions for the data directory

## Data Directory

Tokens are stored in a BoltDB database located in:
1. `/opt/mcpfusion/` (if writable)
2. `~/.mcpfusion/` (user home directory fallback)

## CLI Commands

### Generate New API Token

```bash
# Generate token with description
mcpfusion-token add "Production API access"

# Generate token without description
mcpfusion-token add
```

**Output:**
```
Token generated successfully!
Hash: a1b2c3d4e5f6789...
IMPORTANT: Save this token securely. It cannot be retrieved later.
Token: 1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef1234567890
```

⚠️ **Security Warning**: The token is displayed only once. Save it securely immediately.

### List All Tokens

```bash
mcpfusion-token list
```

**Output:**
```
PREFIX    HASH               CREATED             LAST USED          DESCRIPTION
1a2b3c4d  a1b2c3d4e5f6...    2025-01-15 10:30:00 2025-01-15 11:45:00 Production API access
9f8e7d6c  f9e8d7c6b5a4...    2025-01-14 09:15:00 Never used          Development token

Total tokens: 2
Use 'mcpfusion-token delete <PREFIX>' to remove tokens.
```

### Delete Token

```bash
# Delete by prefix (8 characters)
mcpfusion-token delete 1a2b3c4d

# Delete by full hash
mcpfusion-token delete a1b2c3d4e5f6789abcdef...
```

**Interactive Confirmation:**
```
Token Details:
  Prefix: 1a2b3c4d
  Hash: a1b2c3d4e5f6789...
  Created: 2025-01-15 10:30:00
  Last Used: 2025-01-15 11:45:00
  Description: Production API access

Are you sure you want to delete this token? (y/N): y
Token deleted successfully: a1b2c3d4e5f6789...
```

## CLI Options

### Global Flags

- `--data-dir <path>` - Custom data directory
- `--debug` - Enable debug logging
- `--no-color` - Disable colored output (for automation)

### Examples

```bash
# Use custom data directory
mcpfusion-token --data-dir /custom/path list

# Enable debug mode
mcpfusion-token --debug add "Debug token"

# Disable colors for scripting
mcpfusion-token --no-color list
```

## Using API Tokens

### HTTP Authentication

Include the API token in the Authorization header:

```bash
curl -H "Authorization: Bearer <your-token>" \
     https://your-mcpfusion-server/api/endpoint
```

### Multiple Tenants

Each API token represents a separate tenant with isolated:
- OAuth tokens (Microsoft365, Google, etc.)
- Service credentials
- Authentication state

Example multi-tenant usage:
```bash
# Tenant 1 (Production)
curl -H "Authorization: Bearer 1a2b3c4d..." \
     https://server/api/microsoft365/profile

# Tenant 2 (Development) 
curl -H "Authorization: Bearer 9f8e7d6c..." \
     https://server/api/microsoft365/profile
```

Both tenants can access Microsoft365 independently with different OAuth tokens.

## Security Best Practices

### Token Management

1. **Store Securely**: Save tokens in environment variables or secure vaults
2. **One-Time Display**: Tokens are shown only during creation
3. **Regular Rotation**: Delete and recreate tokens periodically
4. **Descriptive Names**: Use clear descriptions to identify token purposes

### Environment Variables

```bash
# Set token as environment variable
export MCPFUSION_TOKEN="1a2b3c4d5e6f7890..."

# Use in applications
curl -H "Authorization: Bearer $MCPFUSION_TOKEN" ...
```

### Access Control

- Limit database directory access (0600 permissions)
- Use separate tokens for different applications/environments
- Monitor token usage via the `list` command

## Troubleshooting

### Common Issues

**Database Permission Errors:**
```bash
# Ensure proper ownership
sudo chown -R $(whoami) /opt/mcpfusion/
chmod 700 /opt/mcpfusion/
```

**Token Not Found:**
```bash
# List all tokens to verify
mcpfusion-token list

# Try exact prefix or full hash
mcpfusion-token delete a1b2c3d4e5f6...
```

**Authentication Failures:**
- Verify token format (64 hex characters)
- Check Authorization header format: `Bearer <token>`
- Confirm server is configured for database authentication

### Debug Mode

Enable debug logging for troubleshooting:

```bash
mcpfusion-token --debug add "Test token"
```

Debug logs are written to temporary files and logged to stdout.

## Migration from File Cache

If migrating from file-based token storage:

1. **Create First Token**: Generate new API token with CLI
2. **Update Configuration**: Configure applications to use new tokens
3. **Remove Old Cache**: Delete old file-based cache directory
4. **Test Access**: Verify API access with new tokens

## Integration Examples

### Docker Environment

```dockerfile
# Set token as environment variable
ENV MCPFUSION_TOKEN=1a2b3c4d5e6f7890...

# Use in application
CMD ["curl", "-H", "Authorization: Bearer ${MCPFUSION_TOKEN}", "..."]
```

### Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mcpfusion-token
data:
  token: <base64-encoded-token>
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        env:
        - name: MCPFUSION_TOKEN
          valueFrom:
            secretKeyRef:
              name: mcpfusion-token
              key: token
```

### Application Code

```python
import os
import requests

token = os.environ.get('MCPFUSION_TOKEN')
headers = {'Authorization': f'Bearer {token}'}

response = requests.get(
    'https://mcpfusion/api/microsoft365/profile',
    headers=headers
)
```

## Support

For additional help:
- Check server logs for authentication errors
- Verify database connectivity and permissions
- Ensure MCPFusion server is running with database support
- Review token format and API endpoint configuration