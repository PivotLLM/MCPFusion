/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
)

// HTTPClient manages an HTTP MCP client connection
type HTTPClient struct {
	config  *fusion.ServiceConfig
	manager *MCPClientManager
	backoff *ExponentialBackoff
	logger  global.Logger
}

// NewHTTPClient creates a new HTTP client for the given service config
func NewHTTPClient(config *fusion.ServiceConfig, logger global.Logger) *HTTPClient {
	baseDelay := time.Second
	maxDelay := 60 * time.Second
	factor := 2.0

	if config.Retry != nil {
		if config.Retry.BaseDelay > 0 {
			baseDelay = config.Retry.BaseDelay
		}
		if config.Retry.MaxDelay > 0 {
			maxDelay = config.Retry.MaxDelay
		}
		if config.Retry.BackoffFactor > 0 {
			factor = config.Retry.BackoffFactor
		}
	}

	return &HTTPClient{
		config:  config,
		manager: NewMCPClientManager(config.ServiceKey, logger),
		backoff: NewExponentialBackoff(baseDelay, maxDelay, factor),
		logger:  logger,
	}
}

// Manager returns the underlying MCPClientManager
func (h *HTTPClient) Manager() *MCPClientManager {
	return h.manager
}

// Connect creates and connects the HTTP MCP client
func (h *HTTPClient) Connect(ctx context.Context) error {
	h.logger.Infof("Hub service '%s': connecting to HTTP endpoint: %s", h.config.ServiceKey, h.config.BaseURL)

	// Build transport options
	var opts []transport.StreamableHTTPCOption

	// Apply auth headers based on config
	headers := h.buildAuthHeaders()
	if len(headers) > 0 {
		opts = append(opts, transport.WithHTTPHeaders(headers))
	}

	// Create the streamable HTTP MCP client
	c, err := client.NewStreamableHttpClient(h.config.BaseURL, opts...)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	h.manager.SetClient(c)

	// Register notification handler before connecting
	h.manager.RegisterNotificationHandler()

	// Register connection loss handler
	c.OnConnectionLost(func(err error) {
		h.logger.Errorf("Hub service '%s': connection lost: %v", h.config.ServiceKey, err)
		h.manager.SetConnected(false)
	})

	// Initialize the MCP session
	if err := h.manager.Connect(ctx); err != nil {
		c.Close()
		h.manager.SetClient(nil)
		return fmt.Errorf("failed to initialize: %w", err)
	}

	h.backoff.Reset()
	h.logger.Infof("Hub service '%s': HTTP connection established", h.config.ServiceKey)
	return nil
}

// buildAuthHeaders builds HTTP headers based on auth configuration
func (h *HTTPClient) buildAuthHeaders() map[string]string {
	headers := make(map[string]string)

	switch h.config.Auth.Type {
	case fusion.AuthTypeBearer:
		if token, ok := h.config.Auth.Config["token"].(string); ok && token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	case fusion.AuthTypeBasic:
		username, _ := h.config.Auth.Config["username"].(string)
		password, _ := h.config.Auth.Config["password"].(string)
		if username != "" {
			encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
			headers["Authorization"] = "Basic " + encoded
		}
	case fusion.AuthTypeAPIKey:
		if key, ok := h.config.Auth.Config["apiKey"].(string); ok && key != "" {
			headerName := "X-API-Key"
			if name, ok := h.config.Auth.Config["headerName"].(string); ok && name != "" {
				headerName = name
			}
			headers[headerName] = key
		}
	case fusion.AuthTypeNone, "":
		// No auth headers needed
	}

	return headers
}

// RunWithReconnect runs the client with automatic reconnection on failure.
// This blocks until the context is cancelled.
func (h *HTTPClient) RunWithReconnect(ctx context.Context, onConnected func()) {
	for {
		if ctx.Err() != nil {
			return
		}

		err := h.Connect(ctx)
		if err != nil {
			h.logger.Errorf("Hub service '%s': connection failed: %v (retrying in %v)",
				h.config.ServiceKey, err, h.backoff.CurrentDelay())

			if waitErr := h.backoff.Wait(ctx); waitErr != nil {
				return
			}
			continue
		}

		if onConnected != nil {
			onConnected()
		}

		// Wait until connection is lost or context is cancelled
		h.waitForDisconnect(ctx)

		if ctx.Err() != nil {
			return
		}

		h.logger.Warningf("Hub service '%s': disconnected, will reconnect in %v",
			h.config.ServiceKey, h.backoff.CurrentDelay())
		h.manager.Disconnect()

		if waitErr := h.backoff.Wait(ctx); waitErr != nil {
			return
		}
	}
}

// waitForDisconnect blocks until the client disconnects or context is cancelled
func (h *HTTPClient) waitForDisconnect(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !h.manager.IsConnected() {
				return
			}
		}
	}
}

// Close disconnects the HTTP client
func (h *HTTPClient) Close() error {
	return h.manager.Disconnect()
}
