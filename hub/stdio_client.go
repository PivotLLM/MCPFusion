/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/mark3labs/mcp-go/client"
)

// StdioClient manages a stdio MCP client connection
type StdioClient struct {
	config  *fusion.ServiceConfig
	manager *MCPClientManager
	backoff *ExponentialBackoff
	logger  global.Logger
}

// NewStdioClient creates a new stdio client for the given service config
func NewStdioClient(config *fusion.ServiceConfig, logger global.Logger) *StdioClient {
	// Use retry config if provided, otherwise defaults
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

	return &StdioClient{
		config:  config,
		manager: NewMCPClientManager(config.ServiceKey, logger),
		backoff: NewExponentialBackoff(baseDelay, maxDelay, factor),
		logger:  logger,
	}
}

// Manager returns the underlying MCPClientManager
func (s *StdioClient) Manager() *MCPClientManager {
	return s.manager
}

// Connect creates and connects the stdio MCP client
func (s *StdioClient) Connect(ctx context.Context) error {
	// Build environment variables as []string in "KEY=VALUE" format
	var env []string
	for k, v := range s.config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// If MCP_FUSION_ADD_PATH is set, prepend its directories to the subprocess PATH
	// so child processes (e.g., node spawned by npx) can also be found.
	if addPath := getAddPath(s.logger); addPath != "" {
		env = append(env, fmt.Sprintf("PATH=%s:%s", addPath, os.Getenv("PATH")))
	}

	// Resolve the command to an absolute path
	command := resolveCommand(s.config.Command, s.logger)

	s.logger.Infof("Hub service '%s': starting stdio process: %s %v", s.config.ServiceKey, command, s.config.Args)

	// Create the stdio MCP client
	c, err := client.NewStdioMCPClient(command, env, s.config.Args...)
	if err != nil {
		return fmt.Errorf("failed to create stdio client: %w", err)
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
	s.logger.Infof("Hub service '%s': stdio connection established", s.config.ServiceKey)
	return nil
}

// RunWithReconnect runs the client with automatic reconnection on failure.
// This blocks until the context is cancelled.
func (s *StdioClient) RunWithReconnect(ctx context.Context, onConnected func()) {
	for {
		// Check if context is done
		if ctx.Err() != nil {
			return
		}

		// Try to connect
		err := s.Connect(ctx)
		if err != nil {
			s.logger.Errorf("Hub service '%s': connection failed: %v (retrying in %v)",
				s.config.ServiceKey, err, s.backoff.CurrentDelay())

			if waitErr := s.backoff.Wait(ctx); waitErr != nil {
				return // context cancelled
			}
			continue
		}

		// Connected successfully
		if onConnected != nil {
			onConnected()
		}

		// Wait until connection is lost or context is cancelled
		s.waitForDisconnect(ctx)

		// If context is done, exit
		if ctx.Err() != nil {
			return
		}

		// Connection lost, clean up and retry
		s.logger.Warningf("Hub service '%s': disconnected, will reconnect in %v",
			s.config.ServiceKey, s.backoff.CurrentDelay())
		s.manager.Disconnect()

		if waitErr := s.backoff.Wait(ctx); waitErr != nil {
			return // context cancelled
		}
	}
}

// waitForDisconnect blocks until the client disconnects or context is cancelled
func (s *StdioClient) waitForDisconnect(ctx context.Context) {
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

// Close disconnects the stdio client
func (s *StdioClient) Close() error {
	return s.manager.Disconnect()
}

// resolveCommand attempts to resolve a command name to an absolute path.
// It first checks exec.LookPath (process PATH), then searches any additional
// directories specified via the MCP_FUSION_ADD_PATH environment variable.
// This handles cases where MCPFusion is launched from an environment with a
// limited PATH (e.g., systemd) that doesn't include directories like nvm,
// pyenv, or other version managers.
func resolveCommand(command string, logger global.Logger) string {
	// Already absolute — nothing to resolve
	if filepath.IsAbs(command) {
		return command
	}

	// Try the current process PATH first
	if resolved, err := exec.LookPath(command); err == nil {
		logger.Debugf("Resolved command '%s' to '%s' via process PATH", command, resolved)
		return resolved
	}

	// Fall back to MCP_FUSION_ADD_PATH directories
	if addPath := getAddPath(logger); addPath != "" {
		for _, dir := range strings.Split(addPath, ":") {
			candidate := filepath.Join(dir, command)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				logger.Debugf("Resolved command '%s' to '%s' via MCP_FUSION_ADD_PATH", command, candidate)
				return candidate
			}
		}
	}

	// Could not resolve — return as-is and let the caller report the error
	logger.Debugf("Could not resolve command '%s' in any PATH", command)
	return command
}

// getAddPath reads and caches the MCP_FUSION_ADD_PATH environment variable.
// This variable contains colon-separated directories to search when the
// process PATH doesn't contain the required executables.
var (
	addPathOnce  sync.Once
	addPathCache string
)

func getAddPath(logger global.Logger) string {
	addPathOnce.Do(func() {
		addPathCache = os.Getenv("MCP_FUSION_ADD_PATH")
		if addPathCache != "" {
			logger.Infof("MCP_FUSION_ADD_PATH: %s", addPathCache)
		}
	})
	return addPathCache
}
