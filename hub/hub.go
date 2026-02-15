/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// hubClient is a common interface for stdio and HTTP hub clients
type hubClient interface {
	Manager() *MCPClientManager
	RunWithReconnect(ctx context.Context, onConnected func(), onDisconnected func())
	Close() error
}

// HubProvider manages connections to downstream MCP servers and exposes their tools.
// It implements global.ToolProvider.
type HubProvider struct {
	mu              sync.RWMutex
	configs         map[string]*fusion.ServiceConfig // hub service configs keyed by service key
	clients         map[string]hubClient
	refreshCancels  map[string]context.CancelFunc // per-service periodic refresh cancellation
	mcpServer       *server.MCPServer
	logger          global.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	tokenCounter    int64 // atomic counter for unique downstream progress tokens (int64 overflow is not a practical concern)
	sharedCollector *metrics.Collector
}

// HubOption defines a functional option for configuring a HubProvider.
type HubOption func(*HubProvider)

// WithSharedCollector sets the cross-package metrics collector for request tracking.
func WithSharedCollector(c *metrics.Collector) HubOption {
	return func(h *HubProvider) {
		h.sharedCollector = c
	}
}

// NewHubProvider creates a new HubProvider with the given hub service configurations.
func NewHubProvider(configs map[string]*fusion.ServiceConfig, logger global.Logger, opts ...HubOption) *HubProvider {
	h := &HubProvider{
		configs:        configs,
		clients:        make(map[string]hubClient),
		refreshCancels: make(map[string]context.CancelFunc),
		logger:         logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterTools implements global.ToolProvider.
// Returns an empty list because hub tools are discovered asynchronously.
func (h *HubProvider) RegisterTools() []global.ToolDefinition {
	return []global.ToolDefinition{}
}

// SetMCPServer provides the underlying MCP server for dynamic tool registration.
func (h *HubProvider) SetMCPServer(srv *server.MCPServer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mcpServer = srv
}

// Start begins connecting to all configured hub services.
func (h *HubProvider) Start(ctx context.Context) {
	h.ctx, h.cancel = context.WithCancel(ctx)

	for serviceKey, config := range h.configs {
		var c hubClient

		switch config.Transport {
		case fusion.TransportTypeStdio:
			c = NewStdioClient(config, h.logger)
		case fusion.TransportTypeMCPHTTP:
			c = NewHTTPClient(config, h.logger)
		case fusion.TransportTypeSSE:
			c = NewSSEClient(config, h.logger)
		default:
			h.logger.Errorf("Hub service '%s': unsupported transport: %s", serviceKey, config.Transport)
			continue
		}

		h.mu.Lock()
		h.clients[serviceKey] = c
		h.mu.Unlock()

		// Set up the tools changed callback
		c.Manager().SetOnToolsChanged(h.onToolsChanged)

		// Start the connection in a goroutine
		h.wg.Add(1)
		go func(key string, client hubClient, cfg *fusion.ServiceConfig) {
			defer h.wg.Done()
			client.RunWithReconnect(h.ctx,
				func() {
					// Cancel any previous periodic refresh goroutine for this service
					h.mu.Lock()
					if cancelFn, ok := h.refreshCancels[key]; ok {
						cancelFn()
						delete(h.refreshCancels, key)
					}
					h.mu.Unlock()

					// Remove stale tools from previous connection before re-registering
					h.removeServiceTools(key, client.Manager())

					// Discover and register tools
					h.discoverAndRegisterTools(key, client.Manager())

					// Start periodic refresh if configured
					if cfg.ToolRefreshInterval > 0 {
						refreshCtx, refreshCancel := context.WithCancel(h.ctx)
						h.mu.Lock()
						h.refreshCancels[key] = refreshCancel
						h.mu.Unlock()

						h.wg.Add(1)
						go func() {
							defer h.wg.Done()
							h.periodicRefresh(refreshCtx, key, client.Manager(), cfg.ToolRefreshInterval)
						}()
					}
				},
				func() {
					// Set status to disconnected when connection drops or fails
					if h.sharedCollector != nil {
						h.sharedCollector.SetStatus(key, "disconnected")
					}
				},
			)
		}(serviceKey, c, config)
	}

	if len(h.configs) > 0 {
		h.logger.Infof("Hub: started %d hub service connection(s)", len(h.configs))
	}
}

// removeServiceTools removes all previously registered tools for a service from the MCP server.
func (h *HubProvider) removeServiceTools(serviceKey string, manager *MCPClientManager) {
	h.mu.RLock()
	srv := h.mcpServer
	h.mu.RUnlock()

	if srv == nil {
		return
	}

	cachedTools := manager.GetCachedTools()
	if len(cachedTools) == 0 {
		return
	}

	var names []string
	for name := range cachedTools {
		names = append(names, serviceKey+"_"+name)
	}
	srv.DeleteTools(names...)
	h.logger.Debugf("Hub service '%s': removed %d stale tools before reconnect", serviceKey, len(names))
}

// discoverAndRegisterTools discovers tools from a downstream server and registers them.
func (h *HubProvider) discoverAndRegisterTools(serviceKey string, manager *MCPClientManager) {
	ctx, cancel := context.WithTimeout(h.ctx, 30*time.Second)
	defer cancel()

	tools, err := manager.ListTools(ctx)
	if err != nil {
		h.logger.Errorf("Hub service '%s': failed to discover tools: %v", serviceKey, err)
		return
	}

	manager.SetCachedTools(tools)

	h.mu.RLock()
	srv := h.mcpServer
	h.mu.RUnlock()

	if srv == nil {
		h.logger.Errorf("Hub service '%s': MCP server not set, cannot register tools", serviceKey)
		return
	}

	// Convert and register each tool
	var serverTools []server.ServerTool
	for _, tool := range tools {
		toolDef := ConvertDownstreamTool(serviceKey, tool, manager.CallTool)
		mcpTool, handler := h.convertToServerTool(toolDef, manager)
		serverTools = append(serverTools, server.ServerTool{
			Tool:    mcpTool,
			Handler: handler,
		})
	}

	if len(serverTools) > 0 {
		srv.AddTools(serverTools...)
		h.logger.Infof("Hub service '%s': registered %d tools", serviceKey, len(serverTools))
	}

	// Register/update hub service in shared collector
	if h.sharedCollector != nil {
		transport := ""
		if cfg, ok := h.configs[serviceKey]; ok {
			switch cfg.Transport {
			case fusion.TransportTypeStdio:
				transport = global.TransportMCPStdio
			case fusion.TransportTypeSSE:
				transport = global.TransportMCPSSE
			case fusion.TransportTypeMCPHTTP:
				transport = global.TransportMCPHTTP
			default:
				transport = string(cfg.Transport)
			}
		}
		toolCount := len(tools)
		h.sharedCollector.RegisterService(serviceKey, transport, &toolCount)
		h.sharedCollector.SetStatus(serviceKey, global.StatusOperational)
	}
}

// convertToServerTool converts a global.ToolDefinition into a mcp.Tool and handler.
// When manager is non-nil, progress notifications from the downstream server are
// forwarded back to the upstream client.
func (h *HubProvider) convertToServerTool(toolDef global.ToolDefinition, manager *MCPClientManager) (mcp.Tool, server.ToolHandlerFunc) {
	toolOptions := []mcp.ToolOption{
		mcp.WithDescription(toolDef.Description),
	}

	for _, param := range toolDef.Parameters {
		options := []mcp.PropertyOption{mcp.Description(param.Description)}
		if param.Required {
			options = append(options, mcp.Required())
		}

		var toolOption mcp.ToolOption
		switch param.Type {
		case "string":
			toolOption = mcp.WithString(param.Name, options...)
		case "number":
			toolOption = mcp.WithNumber(param.Name, options...)
		case "integer":
			toolOption = mcp.WithNumber(param.Name, options...)
		case "boolean":
			toolOption = mcp.WithBoolean(param.Name, options...)
		case "array":
			options = append(options, mcp.WithStringItems())
			toolOption = mcp.WithArray(param.Name, options...)
		case "object":
			toolOption = mcp.WithObject(param.Name, options...)
		default:
			toolOption = mcp.WithString(param.Name, options...)
		}

		toolOptions = append(toolOptions, toolOption)
	}

	// Add hints
	if toolDef.Hints != nil {
		if toolDef.Hints.ReadOnly != nil {
			toolOptions = append(toolOptions, mcp.WithReadOnlyHintAnnotation(*toolDef.Hints.ReadOnly))
		}
		if toolDef.Hints.Destructive != nil {
			toolOptions = append(toolOptions, mcp.WithDestructiveHintAnnotation(*toolDef.Hints.Destructive))
		}
		if toolDef.Hints.Idempotent != nil {
			toolOptions = append(toolOptions, mcp.WithIdempotentHintAnnotation(*toolDef.Hints.Idempotent))
		}
		if toolDef.Hints.OpenWorld != nil {
			toolOptions = append(toolOptions, mcp.WithOpenWorldHintAnnotation(*toolDef.Hints.OpenWorld))
		}
	}

	mcpTool := mcp.NewTool(toolDef.Name, toolOptions...)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx = context.WithValue(ctx, global.ToolNameKey, toolDef.Name)

		options := req.GetArguments()
		ctxOptions := make(map[string]any)
		for k, v := range options {
			ctxOptions[k] = v
		}
		ctxOptions["__mcp_context"] = ctx

		// Forward progress notifications from downstream to upstream.
		// Each forwarder lives only for the duration of this handler call:
		// it is registered before CallTool and cleaned up by defer when the
		// handler returns. Concurrent tool calls each get their own token,
		// so forwarders do not interfere with each other.
		var downstreamMeta *mcp.Meta
		if manager != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
			if srv := server.ServerFromContext(ctx); srv != nil {
				downstreamToken := fmt.Sprintf("hub-%d", atomic.AddInt64(&h.tokenCounter, 1))
				fwd := &progressForwarder{
					upstreamCtx:   ctx,
					upstreamToken: req.Params.Meta.ProgressToken,
					mcpServer:     srv,
				}
				manager.RegisterProgressForwarder(downstreamToken, fwd)
				defer manager.UnregisterProgressForwarder(downstreamToken)
				downstreamMeta = &mcp.Meta{ProgressToken: downstreamToken}
			}
		}
		ctxOptions["__meta"] = downstreamMeta

		result, err := toolDef.Handler(ctxOptions)

		// Record to shared collector for cross-package health reporting.
		// Extract service key from the prefixed tool name (e.g. "svc_toolname" -> "svc").
		if h.sharedCollector != nil {
			if svcName := ctx.Value(global.ServiceNameKey); svcName != nil {
				if svc, ok := svcName.(string); ok {
					h.sharedCollector.RecordRequest(svc, err != nil)
				}
			}
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	}

	return mcpTool, handler
}

// onToolsChanged handles tool changes from any hub service.
func (h *HubProvider) onToolsChanged(serviceName string, added, removed []string) {
	h.mu.RLock()
	srv := h.mcpServer
	client := h.clients[serviceName]
	h.mu.RUnlock()

	if srv == nil || client == nil {
		return
	}

	manager := client.Manager()

	// Remove old tools
	if len(removed) > 0 {
		var prefixedRemoved []string
		for _, name := range removed {
			prefixedRemoved = append(prefixedRemoved, serviceName+"_"+name)
		}
		srv.DeleteTools(prefixedRemoved...)
		h.logger.Infof("Hub service '%s': removed %d tools", serviceName, len(removed))
	}

	// Add new tools
	if len(added) > 0 {
		cachedTools := manager.GetCachedTools()
		var serverTools []server.ServerTool
		for _, name := range added {
			if tool, ok := cachedTools[name]; ok {
				toolDef := ConvertDownstreamTool(serviceName, tool, manager.CallTool)
				mcpTool, handler := h.convertToServerTool(toolDef, manager)
				serverTools = append(serverTools, server.ServerTool{
					Tool:    mcpTool,
					Handler: handler,
				})
			}
		}
		if len(serverTools) > 0 {
			srv.AddTools(serverTools...)
			h.logger.Infof("Hub service '%s': added %d tools", serviceName, len(added))
		}
	}
}

// periodicRefresh periodically refreshes tools from a downstream server.
// It stops when refreshCtx is cancelled (on disconnect or shutdown).
func (h *HubProvider) periodicRefresh(refreshCtx context.Context, serviceKey string, manager *MCPClientManager, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-refreshCtx.Done():
			return
		case <-ticker.C:
			if !manager.IsConnected() {
				continue
			}

			ctx, cancel := context.WithTimeout(refreshCtx, 30*time.Second)
			if err := manager.RefreshTools(ctx); err != nil {
				h.logger.Debugf("Hub service '%s': periodic refresh failed: %v", serviceKey, err)
			}
			cancel()
		}
	}
}

// Shutdown stops all hub connections and waits for goroutines to finish.
func (h *HubProvider) Shutdown() {
	if h.cancel != nil {
		h.cancel()
	}

	h.mu.RLock()
	clients := make(map[string]hubClient, len(h.clients))
	for k, v := range h.clients {
		clients[k] = v
	}
	h.mu.RUnlock()

	for key, c := range clients {
		if err := c.Close(); err != nil {
			h.logger.Errorf("Hub service '%s': error closing: %v", key, err)
		}
	}

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		h.logger.Info("Hub: all connections closed")
	case <-time.After(5 * time.Second):
		h.logger.Warning("Hub: shutdown timed out waiting for connections to close")
	}
}
