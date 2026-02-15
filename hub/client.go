/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// progressForwarder holds the state needed to relay a progress notification
// from a downstream MCP server back to the upstream client.
type progressForwarder struct {
	upstreamCtx   context.Context
	upstreamToken mcp.ProgressToken
	mcpServer     *server.MCPServer
}

// MCPClientManager wraps an mcp-go client with lifecycle management,
// tool caching, and connection state tracking. All methods are safe
// for concurrent use.
type MCPClientManager struct {
	mu                 sync.RWMutex
	client             *client.Client
	serviceName        string
	connected          bool
	tools              map[string]mcp.Tool // cached tools keyed by name
	logger             global.Logger
	onToolsChanged     func(serviceName string, added, removed []string)
	progressForwarders sync.Map // downstream token string → *progressForwarder
}

// NewMCPClientManager creates a new client manager for the named service.
func NewMCPClientManager(serviceName string, logger global.Logger) *MCPClientManager {
	return &MCPClientManager{
		serviceName: serviceName,
		tools:       make(map[string]mcp.Tool),
		logger:      logger,
	}
}

// SetClient sets the underlying mcp-go client.
func (m *MCPClientManager) SetClient(c *client.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.client = c
}

// SetOnToolsChanged sets the callback invoked when the cached tool set changes.
func (m *MCPClientManager) SetOnToolsChanged(fn func(serviceName string, added, removed []string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onToolsChanged = fn
}

// Connect initializes the MCP session with the downstream server.
func (m *MCPClientManager) Connect(ctx context.Context) error {
	m.mu.Lock()
	c := m.client
	m.mu.Unlock()

	if c == nil {
		return fmt.Errorf("no client set")
	}

	// Initialize the MCP session
	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "MCPFusion-Hub",
		Version: "1.0.0",
	}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	_, err := c.Initialize(ctx, initReq)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP session: %w", err)
	}

	m.mu.Lock()
	m.connected = true
	m.mu.Unlock()

	return nil
}

// Disconnect closes the client connection and clears connection state.
func (m *MCPClientManager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connected = false
	if m.client != nil {
		err := m.client.Close()
		m.client = nil
		return err
	}
	return nil
}

// IsConnected returns whether the client is currently connected.
func (m *MCPClientManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// SetConnected sets the connection state.
func (m *MCPClientManager) SetConnected(connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = connected
}

// ListTools discovers tools from the downstream server without updating the cache.
func (m *MCPClientManager) ListTools(ctx context.Context) (map[string]mcp.Tool, error) {
	m.mu.RLock()
	c := m.client
	connected := m.connected
	m.mu.RUnlock()

	if !connected || c == nil {
		return nil, fmt.Errorf("not connected")
	}

	toolsResult, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	tools := make(map[string]mcp.Tool, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		tools[tool.Name] = tool
	}

	return tools, nil
}

// RefreshTools re-discovers tools from the downstream server, updates the
// cache, and invokes the onToolsChanged callback if the set has changed.
func (m *MCPClientManager) RefreshTools(ctx context.Context) error {
	newTools, err := m.ListTools(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	oldTools := m.tools
	m.tools = newTools
	callback := m.onToolsChanged
	m.mu.Unlock()

	// Compute diff
	diff := DiffTools(oldTools, newTools)

	if len(diff.Added) > 0 || len(diff.Removed) > 0 {
		if m.logger != nil {
			m.logger.Infof("Hub service '%s': tools changed — added: %v, removed: %v",
				m.serviceName, diff.Added, diff.Removed)
		}
		if callback != nil {
			callback(m.serviceName, diff.Added, diff.Removed)
		}
	}

	return nil
}

// GetCachedTools returns a copy of the cached tool set.
func (m *MCPClientManager) GetCachedTools() map[string]mcp.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]mcp.Tool, len(m.tools))
	for k, v := range m.tools {
		result[k] = v
	}
	return result
}

// SetCachedTools replaces the cached tool set.
func (m *MCPClientManager) SetCachedTools(tools map[string]mcp.Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools = tools
}

// CallTool invokes a tool on the downstream server with a 60-second timeout.
// If meta is non-nil, it is forwarded as _meta in the downstream request
// (typically carrying a progress token).
func (m *MCPClientManager) CallTool(ctx context.Context, toolName string, args map[string]interface{}, meta *mcp.Meta) (*mcp.CallToolResult, error) {
	m.mu.RLock()
	c := m.client
	connected := m.connected
	m.mu.RUnlock()

	if !connected || c == nil {
		return nil, fmt.Errorf("hub service '%s' is currently unavailable. The server will automatically reconnect",
			m.serviceName)
	}

	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args
	req.Params.Meta = meta

	result, err := c.CallTool(callCtx, req)
	if err != nil {
		if callCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("tool call timed out after 60s")
		}
		return nil, err
	}

	return result, nil
}

// RegisterProgressForwarder registers a forwarder for a downstream progress token.
func (m *MCPClientManager) RegisterProgressForwarder(downstreamToken string, fwd *progressForwarder) {
	m.progressForwarders.Store(downstreamToken, fwd)
}

// UnregisterProgressForwarder removes a forwarder for a downstream progress token.
func (m *MCPClientManager) UnregisterProgressForwarder(downstreamToken string) {
	m.progressForwarders.Delete(downstreamToken)
}

// RegisterNotificationHandler sets up a handler for tool list change
// notifications from the downstream server. When a notification arrives,
// tools are refreshed asynchronously.
func (m *MCPClientManager) RegisterNotificationHandler() {
	m.mu.RLock()
	c := m.client
	m.mu.RUnlock()

	if c == nil {
		return
	}

	c.OnNotification(func(notification mcp.JSONRPCNotification) {
		switch notification.Method {
		case "notifications/tools/list_changed":
			if m.logger != nil {
				m.logger.Infof("Hub service '%s': received tools/list_changed notification", m.serviceName)
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := m.RefreshTools(ctx); err != nil {
					if m.logger != nil {
						m.logger.Errorf("Hub service '%s': failed to refresh tools after notification: %v",
							m.serviceName, err)
					}
				}
			}()

		case "notifications/progress":
			tokenVal, ok := notification.Params.AdditionalFields["progressToken"]
			if !ok {
				return
			}
			tokenStr := fmt.Sprintf("%v", tokenVal)

			// Look up the forwarder for this downstream token. There is a benign
			// TOCTOU window: the forwarder could be unregistered between Load()
			// and SendNotificationToClient(). In that case we simply deliver one
			// extra (harmless) progress notification to the upstream client.
			val, loaded := m.progressForwarders.Load(tokenStr)
			if !loaded {
				return
			}
			fwd := val.(*progressForwarder)

			// Build upstream params with the original upstream token
			params := map[string]any{
				"progressToken": fwd.upstreamToken,
			}
			if p, exists := notification.Params.AdditionalFields["progress"]; exists {
				params["progress"] = p
			}
			if t, exists := notification.Params.AdditionalFields["total"]; exists {
				params["total"] = t
			}
			if msg, exists := notification.Params.AdditionalFields["message"]; exists {
				params["message"] = msg
			}

			if err := fwd.mcpServer.SendNotificationToClient(fwd.upstreamCtx, "notifications/progress", params); err != nil {
				if m.logger != nil {
					m.logger.Warningf("Hub service '%s': failed to forward progress notification: %v",
						m.serviceName, err)
				}
			}
		}
	})
}
