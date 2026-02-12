# Custom Authorization

MCPFusion supports pluggable tool-level authorization through the `Authorizer` interface. By default, all authenticated tenants can access all tools. To enforce fine-grained access control (per-tenant, per-service, or per-tool), implement a custom `Authorizer`.

## Interface

The `Authorizer` interface is defined in `global/interfaces.go`:

```go
// ToolRequest represents the context of a tool invocation for authorization decisions.
type ToolRequest struct {
    TenantHash  string
    ServiceName string
    ToolName    string
}

// Authorizer defines an interface for authorizing tool requests.
// Return nil to allow the request, or an error to deny it.
type Authorizer interface {
    Authorize(ctx context.Context, req ToolRequest) error
}
```

## Default Behavior

If no `Authorizer` is configured, MCPFusion uses `AllowAllAuthorizer`, which permits all requests:

```go
type AllowAllAuthorizer struct{}

func (a *AllowAllAuthorizer) Authorize(_ context.Context, _ ToolRequest) error {
    return nil
}
```

## Implementing a Custom Authorizer

Create a type that implements the `Authorizer` interface and return an error to deny access:

```go
package myauth

import (
    "context"
    "fmt"

    "github.com/PivotLLM/MCPFusion/global"
)

type RoleBasedAuthorizer struct {
    // permissions maps tenant hash -> set of allowed service names
    permissions map[string]map[string]bool
}

func (a *RoleBasedAuthorizer) Authorize(_ context.Context, req global.ToolRequest) error {
    allowed, ok := a.permissions[req.TenantHash]
    if !ok {
        return fmt.Errorf("tenant not authorized")
    }
    if !allowed[req.ServiceName] {
        return fmt.Errorf("tenant not authorized for service %s", req.ServiceName)
    }
    return nil
}
```

## Wiring It In

Pass your authorizer to the MCP server using the `WithAuthorizer` option in `main.go`:

```go
myAuthorizer := &myauth.RoleBasedAuthorizer{ /* ... */ }

mcpOpts := []mcpserver.Option{
    // ... other options ...
    mcpserver.WithAuthorizer(myAuthorizer),
}
```

## Authorization Flow

The authorization check occurs in the MCP tool handler middleware chain:

1. **HTTP Authentication** - Bearer token validated, `TenantContext` created
2. **Service Validation** - Tool name mapped to service, service existence verified
3. **Tenant Access** - `ValidateTenantAccess` checks tenant can access the service
4. **Authorization** - `Authorizer.Authorize` called with tenant, service, and tool name
5. **Tool Execution** - Request proceeds to the tool handler

If `Authorize` returns an error, the client receives an "authorization denied" error and the tool is not executed.

## Available Context

The `ToolRequest` struct provides:

| Field         | Description                                    | Example                              |
|---------------|------------------------------------------------|--------------------------------------|
| `TenantHash`  | SHA-256 hash identifying the tenant            | `a1b2c3d4e5f6...`                    |
| `ServiceName` | Service extracted from the tool name           | `google`, `microsoft365`             |
| `ToolName`    | Full MCP tool name                             | `google_calendar_events_list`        |

The `context.Context` parameter carries the full request context, including the `TenantContext` (accessible via `global.TenantContextKey`) if additional tenant metadata is needed.
