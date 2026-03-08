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
	addPath string
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

	// Read MCP_FUSION_ADD_PATH once at construction time so the value is
	// stable for the lifetime of this client and test-friendly (t.Setenv works).
	addPath := os.Getenv("MCP_FUSION_ADD_PATH")
	if addPath != "" {
		logger.Debugf("MCP_FUSION_ADD_PATH: %s", addPath)
	}

	return &StdioClient{
		config:  config,
		manager: NewMCPClientManager(config.ServiceKey, logger),
		backoff: NewExponentialBackoff(baseDelay, maxDelay, factor),
		logger:  logger,
		addPath: addPath,
	}
}

// Manager returns the underlying MCPClientManager
func (s *StdioClient) Manager() *MCPClientManager {
	return s.manager
}

// buildEnv merges configEnv and addPath into a []string slice of "KEY=VALUE"
// pairs suitable for passing to exec.Cmd.Env.
//
// If addPath is non-empty it is prepended to the PATH value:
//   - If configEnv already contains a "PATH" key, addPath is prepended to that
//     value so the subprocess inherits the operator-specified PATH rather than
//     the parent process PATH.
//   - Otherwise addPath is prepended to os.Getenv("PATH").
//
// If configEnv is empty and addPath is empty the function returns nil, which
// causes exec.Cmd to inherit the full parent environment unchanged.
//
// If configEnv is empty but addPath is non-empty, the function seeds the
// environment from os.Environ() and patches only the PATH entry, so the
// subprocess retains HOME, USER, SHELL, and other variables it needs.
func buildEnv(configEnv map[string]string, addPath string) []string {
	if len(configEnv) == 0 && addPath == "" {
		return nil
	}

	// Work on a copy so the caller's map is not mutated.
	merged := make(map[string]string, len(configEnv)+1)

	// When the operator has not specified explicit env vars, seed from the
	// parent environment so the subprocess inherits HOME, USER, SHELL, etc.
	if len(configEnv) == 0 {
		for _, entry := range os.Environ() {
			if k, v, ok := strings.Cut(entry, "="); ok {
				merged[k] = v
			}
		}
	} else {
		for k, v := range configEnv {
			merged[k] = v
		}
	}

	if addPath != "" {
		existingPath, ok := merged["PATH"]
		if !ok {
			existingPath = os.Getenv("PATH")
		}
		merged["PATH"] = addPath + string(filepath.ListSeparator) + existingPath
	}

	env := make([]string, 0, len(merged))
	for k, v := range merged {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

// Connect creates and connects the stdio MCP client
func (s *StdioClient) Connect(ctx context.Context) error {
	// Build environment variables, merging PATH correctly when both
	// config.Env and MCP_FUSION_ADD_PATH supply a PATH value.
	env := buildEnv(s.config.Env, s.addPath)

	// Resolve the command to an absolute path
	command := resolveCommand(s.config.Command, s.addPath, s.logger)

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

	// Initialize the MCP session. On failure, clear the manager reference
	// first so concurrent callers cannot use the client while it is closing.
	if err := s.manager.Connect(ctx); err != nil {
		s.manager.SetClient(nil)
		c.Close()
		return fmt.Errorf("failed to initialize: %w", err)
	}

	s.backoff.Reset()
	s.logger.Infof("Hub service '%s': stdio connection established", s.config.ServiceKey)
	return nil
}

// RunWithReconnect runs the client with automatic reconnection on failure.
// This blocks until the context is cancelled.
func (s *StdioClient) RunWithReconnect(ctx context.Context, onConnected func(), onDisconnected func()) {
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

			if onDisconnected != nil {
				onDisconnected()
			}

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

		if onDisconnected != nil {
			onDisconnected()
		}

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
// directories specified via addPath (the value of MCP_FUSION_ADD_PATH read at
// client construction time). This handles cases where MCPFusion is launched
// from an environment with a limited PATH (e.g., systemd) that doesn't include
// directories like nvm, pyenv, or other version managers.
//
// Security: candidates from addPath must have an executable bit set, but
// ownership is not verified. Operators should ensure that directories listed in
// MCP_FUSION_ADD_PATH are owned by the service user and are not world-writable.
func resolveCommand(command string, addPath string, logger global.Logger) string {
	// Already absolute — nothing to resolve
	if filepath.IsAbs(command) {
		return command
	}

	// Try the current process PATH first
	if resolved, err := exec.LookPath(command); err == nil {
		logger.Debugf("Resolved command '%s' to '%s' via process PATH", command, resolved)
		return resolved
	}

	// Fall back to addPath directories (MCP_FUSION_ADD_PATH)
	if addPath != "" {
		for _, dir := range strings.Split(addPath, string(filepath.ListSeparator)) {
			candidate := filepath.Join(dir, command)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode().Perm()&0111 != 0 {
				logger.Debugf("Resolved command '%s' to '%s' via MCP_FUSION_ADD_PATH", command, candidate)
				return candidate
			}
		}
	}

	// Could not resolve — return as-is and let the caller report the error
	logger.Debugf("Could not resolve command '%s' in any PATH", command)
	return command
}
