/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// isTransportError returns true if err indicates a transport-level failure
// (e.g. invalid session ID after upstream restart) rather than an
// application-level error from the tool itself.
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "transport error") ||
		strings.Contains(msg, "invalid session")
}

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
	cbMu               sync.Mutex
	cbFailures         int
	cbOpenUntil        time.Time
	cbHalfOpen         bool // true while exactly one probe call is in flight
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

	m.cbMu.Lock()
	m.cbFailures = 0
	m.cbOpenUntil = time.Time{}
	m.cbMu.Unlock()

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

const (
	cbFailureThreshold = 5
	cbOpenDuration     = 30 * time.Second
)

func (m *MCPClientManager) isCircuitOpen() bool {
	m.cbMu.Lock()
	defer m.cbMu.Unlock()
	if m.cbOpenUntil.IsZero() {
		// Closed or a probe call is already in flight (half-open).
		// If half-open, block additional callers until the probe resolves.
		return m.cbHalfOpen
	}
	if time.Now().After(m.cbOpenUntil) {
		// Timer just expired: transition to half-open and let exactly one
		// probe call through.  Subsequent callers see cbHalfOpen=true and
		// are blocked until recordCallSuccess/Failure resets the state.
		m.cbOpenUntil = time.Time{}
		m.cbFailures = 0
		m.cbHalfOpen = true
		if m.logger != nil {
			m.logger.Infof("Hub service '%s': circuit breaker half-open, allowing probe call", m.serviceName)
		}
		return false
	}
	return true
}

func (m *MCPClientManager) recordCallFailure() {
	m.cbMu.Lock()
	defer m.cbMu.Unlock()
	m.cbHalfOpen = false // probe call finished; allow re-evaluation
	m.cbFailures++
	if m.cbFailures >= cbFailureThreshold && m.cbOpenUntil.IsZero() {
		m.cbOpenUntil = time.Now().Add(cbOpenDuration)
		if m.logger != nil {
			m.logger.Warningf("Hub service '%s': circuit breaker opened after %d consecutive failures (open until %s)",
				m.serviceName, m.cbFailures, m.cbOpenUntil.Format(time.RFC3339))
		}
	}
}

func (m *MCPClientManager) recordCallSuccess() {
	m.cbMu.Lock()
	defer m.cbMu.Unlock()
	m.cbHalfOpen = false
	if m.cbFailures > 0 {
		m.cbFailures = 0
		m.cbOpenUntil = time.Time{}
		if m.logger != nil {
			m.logger.Infof("Hub service '%s': circuit breaker reset after successful call", m.serviceName)
		}
	}
}

// waitForReconnect blocks until the manager is connected or the timeout
// elapses. Returns true if the connection was re-established in time.
func (m *MCPClientManager) waitForReconnect(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.IsConnected() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
	return false
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
//
// If a transport error is detected (e.g. the upstream server restarted and
// invalidated the session without dropping the TCP connection), CallTool
// triggers the reconnect loop and retries the call once after reconnection.
func (m *MCPClientManager) CallTool(ctx context.Context, toolName string, args map[string]interface{}, meta *mcp.Meta) (*mcp.CallToolResult, error) {
	m.mu.RLock()
	c := m.client
	connected := m.connected
	m.mu.RUnlock()

	if !connected || c == nil {
		return nil, fmt.Errorf("hub service '%s' is currently unavailable. The server will automatically reconnect",
			m.serviceName)
	}

	if m.isCircuitOpen() {
		remaining := time.Until(m.cbOpenUntil).Round(time.Second)
		return nil, fmt.Errorf("hub service '%s' circuit breaker is open (resets in %v)", m.serviceName, remaining)
	}

	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args
	req.Params.Meta = meta

	if m.logger != nil {
		m.logger.Debugf("Hub service '%s': calling tool '%s' (timeout 60s) [ctx deadline: %v]",
			m.serviceName, toolName, func() string {
				if d, ok := callCtx.Deadline(); ok {
					return d.Sub(time.Now()).Round(time.Second).String()
				}
				return "none"
			}())
	}
	result, err := c.CallTool(callCtx, req)
	if m.logger != nil {
		if err != nil {
			m.logger.Debugf("Hub service '%s': tool '%s' returned error: %v", m.serviceName, toolName, err)
		} else {
			m.logger.Debugf("Hub service '%s': tool '%s' completed successfully", m.serviceName, toolName)
		}
	}
	if err != nil {
		if callCtx.Err() == context.DeadlineExceeded {
			m.recordCallFailure()
			return nil, fmt.Errorf("tool call timed out after 60s")
		}

		// Transport errors (e.g. "Invalid session ID" after an upstream restart
		// that did not drop the TCP connection) indicate the session is silently
		// broken. Trigger the existing reconnect loop and retry once.
		if isTransportError(err) {
			if m.logger != nil {
				m.logger.Warningf("Hub service '%s': transport error on tool '%s', triggering reconnect: %v",
					m.serviceName, toolName, err)
			}
			m.SetConnected(false)
			if m.waitForReconnect(callCtx, 15*time.Second) {
				m.mu.RLock()
				c = m.client
				m.mu.RUnlock()
				if c != nil {
					retryCtx, retryCancel := context.WithTimeout(ctx, 60*time.Second)
					defer retryCancel()
					if result, err = c.CallTool(retryCtx, req); err == nil {
						if m.logger != nil {
							m.logger.Infof("Hub service '%s': tool '%s' succeeded after reconnect",
								m.serviceName, toolName)
						}
						m.recordCallSuccess()
						return result, nil
					}
				}
			} else {
				if m.logger != nil {
					m.logger.Errorf("Hub service '%s': reconnect timed out, tool '%s' failed",
						m.serviceName, toolName)
				}
			}
		}
		m.recordCallFailure()
		return nil, err
	}

	m.recordCallSuccess()
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
			// ProgressToken is defined as `any` in the MCP spec. Try a direct
			// string assertion first (the common case for hub-generated tokens)
			// and fall back to fmt.Sprintf for numeric or other token types.
			tokenStr, ok := tokenVal.(string)
			if !ok {
				tokenStr = fmt.Sprintf("%v", tokenVal)
			}

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
