/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/mlogger/testlogger"
)

// newTestLogger returns a Logger backed by t for use across hub package tests.
func newTestLogger(t *testing.T) *testlogger.Logger {
	t.Helper()
	return testlogger.New(t)
}

// newTestConfigs returns a standard set of service configs for tests.
func newTestConfigs() map[string]*fusion.ServiceConfig {
	return map[string]*fusion.ServiceConfig{
		"test_stdio": {
			ServiceKey: "test_stdio",
			Name:       "Test Stdio",
			Transport:  fusion.TransportTypeStdio,
			Command:    "/bin/echo",
		},
		"test_http": {
			ServiceKey: "test_http",
			Name:       "Test HTTP",
			Transport:  fusion.TransportTypeMCPHTTP,
			BaseURL:    "http://localhost:9999/mcp",
		},
	}
}

func TestHubProvider_RegisterTools_ReturnsEmpty(t *testing.T) {
	logger := testlogger.New(t)
	configs := newTestConfigs()

	provider := NewHubProvider(configs, logger)
	require.NotNil(t, provider)

	tools := provider.RegisterTools()
	assert.Empty(t, tools, "RegisterTools should return an empty slice")
	assert.IsType(t, []global.ToolDefinition{}, tools,
		fmt.Sprintf("RegisterTools should return []global.ToolDefinition, got %T", tools))
}

func TestHubProvider_NewHubProvider(t *testing.T) {
	logger := testlogger.New(t)
	configs := newTestConfigs()

	provider := NewHubProvider(configs, logger)
	require.NotNil(t, provider, "NewHubProvider should return a non-nil provider")

	assert.Len(t, provider.configs, 2, "provider should store both configs")
	assert.Contains(t, provider.configs, "test_stdio", "configs should contain test_stdio")
	assert.Contains(t, provider.configs, "test_http", "configs should contain test_http")
	assert.Equal(t, fusion.TransportTypeStdio, provider.configs["test_stdio"].Transport)
	assert.Equal(t, fusion.TransportTypeMCPHTTP, provider.configs["test_http"].Transport)
	assert.NotNil(t, provider.clients, "clients map should be initialized")
	assert.Empty(t, provider.clients, "clients map should be empty before Start")
}

func TestHubProvider_Shutdown_NoStart(t *testing.T) {
	logger := testlogger.New(t)
	configs := newTestConfigs()

	provider := NewHubProvider(configs, logger)
	require.NotNil(t, provider)

	// Shutdown without Start should not panic.
	assert.NotPanics(t, func() {
		provider.Shutdown()
	}, "Shutdown without prior Start should not panic")
}
