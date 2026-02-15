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

// SSEClient manages an SSE MCP client connection
type SSEClient struct {
	config  *fusion.ServiceConfig
	manager *MCPClientManager
	backoff *ExponentialBackoff
	logger  global.Logger
}

// NewSSEClient creates a new SSE client for the given service config
func NewSSEClient(config *fusion.ServiceConfig, logger global.Logger) *SSEClient {
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

	return &SSEClient{
		config:  config,
		manager: NewMCPClientManager(config.ServiceKey, logger),
		backoff: NewExponentialBackoff(baseDelay, maxDelay, factor),
		logger:  logger,
	}
}

// Manager returns the underlying MCPClientManager
func (s *SSEClient) Manager() *MCPClientManager {
	return s.manager
}

// Connect creates and connects the SSE MCP client
func (s *SSEClient) Connect(ctx context.Context) error {
	s.logger.Infof("Hub service '%s': connecting to SSE endpoint: %s", s.config.ServiceKey, s.config.BaseURL)

	// Build transport options
	var opts []transport.ClientOption

	// Apply auth headers based on config
	headers := s.buildAuthHeaders()
	if len(headers) > 0 {
		opts = append(opts, transport.WithHeaders(headers))
	}

	// Create the SSE MCP client
	c, err := client.NewSSEMCPClient(s.config.BaseURL, opts...)
	if err != nil {
		return fmt.Errorf("failed to create SSE client: %w", err)
	}

	// Start the SSE transport (establishes the event stream)
	if err := c.Start(ctx); err != nil {
		c.Close()
		return fmt.Errorf("failed to start SSE transport: %w", err)
	}

	s.manager.SetClient(c)

	// Register notification handler before connecting
	s.manager.RegisterNotificationHandler()

	// Register connection loss handler
	c.OnConnectionLost(func(err error) {
		s.logger.Errorf("Hub service '%s': connection lost: %v", s.config.ServiceKey, err)
		s.manager.SetConnected(false)
	})

	// Initialize the MCP session
	if err := s.manager.Connect(ctx); err != nil {
		c.Close()
		s.manager.SetClient(nil)
		return fmt.Errorf("failed to initialize: %w", err)
	}

	s.backoff.Reset()
	s.logger.Infof("Hub service '%s': SSE connection established", s.config.ServiceKey)
	return nil
}

// buildAuthHeaders builds HTTP headers based on auth configuration
func (s *SSEClient) buildAuthHeaders() map[string]string {
	headers := make(map[string]string)

	switch s.config.Auth.Type {
	case fusion.AuthTypeBearer:
		if token, ok := s.config.Auth.Config["token"].(string); ok && token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	case fusion.AuthTypeBasic:
		username, _ := s.config.Auth.Config["username"].(string)
		password, _ := s.config.Auth.Config["password"].(string)
		if username != "" {
			encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
			headers["Authorization"] = "Basic " + encoded
		}
	case fusion.AuthTypeAPIKey:
		if key, ok := s.config.Auth.Config["apiKey"].(string); ok && key != "" {
			headerName := "X-API-Key"
			if name, ok := s.config.Auth.Config["headerName"].(string); ok && name != "" {
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
func (s *SSEClient) RunWithReconnect(ctx context.Context, onConnected func()) {
	for {
		if ctx.Err() != nil {
			return
		}

		err := s.Connect(ctx)
		if err != nil {
			s.logger.Errorf("Hub service '%s': connection failed: %v (retrying in %v)",
				s.config.ServiceKey, err, s.backoff.CurrentDelay())

			if waitErr := s.backoff.Wait(ctx); waitErr != nil {
				return
			}
			continue
		}

		if onConnected != nil {
			onConnected()
		}

		// Wait until connection is lost or context is cancelled
		s.waitForDisconnect(ctx)

		if ctx.Err() != nil {
			return
		}

		s.logger.Warningf("Hub service '%s': disconnected, will reconnect in %v",
			s.config.ServiceKey, s.backoff.CurrentDelay())
		s.manager.Disconnect()

		if waitErr := s.backoff.Wait(ctx); waitErr != nil {
			return
		}
	}
}

// waitForDisconnect blocks until the client disconnects or context is cancelled
func (s *SSEClient) waitForDisconnect(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.manager.IsConnected() {
				return
			}
		}
	}
}

// Close disconnects the SSE client
func (s *SSEClient) Close() error {
	return s.manager.Disconnect()
}
