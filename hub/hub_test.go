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
)

// testLogger implements global.Logger using testing.T for log output.
type testLogger struct {
	t *testing.T
}

func newTestLogger(t *testing.T) *testLogger {
	return &testLogger{t: t}
}

func (l *testLogger) Debug(msg string)                    { l.t.Log("[DEBUG] " + msg) }
func (l *testLogger) Debugf(format string, args ...any)   { l.t.Logf("[DEBUG] "+format, args...) }
func (l *testLogger) Info(msg string)                     { l.t.Log("[INFO] " + msg) }
func (l *testLogger) Infof(format string, args ...any)    { l.t.Logf("[INFO] "+format, args...) }
func (l *testLogger) Notice(msg string)                   { l.t.Log("[NOTICE] " + msg) }
func (l *testLogger) Noticef(format string, args ...any)  { l.t.Logf("[NOTICE] "+format, args...) }
func (l *testLogger) Warning(msg string)                  { l.t.Log("[WARNING] " + msg) }
func (l *testLogger) Warningf(format string, args ...any) { l.t.Logf("[WARNING] "+format, args...) }
func (l *testLogger) Error(msg string)                    { l.t.Log("[ERROR] " + msg) }
func (l *testLogger) Errorf(format string, args ...any)   { l.t.Logf("[ERROR] "+format, args...) }
func (l *testLogger) Fatal(msg string)                    { l.t.Log("[FATAL] " + msg) }
func (l *testLogger) Fatalf(format string, args ...any)   { l.t.Logf("[FATAL] "+format, args...) }
func (l *testLogger) Close()                              {}

// Compile-time check that testLogger satisfies global.Logger.
var _ global.Logger = (*testLogger)(nil)

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
	logger := newTestLogger(t)
	configs := newTestConfigs()

	provider := NewHubProvider(configs, logger)
	require.NotNil(t, provider)

	tools := provider.RegisterTools()
	assert.Empty(t, tools, "RegisterTools should return an empty slice")
	assert.IsType(t, []global.ToolDefinition{}, tools,
		fmt.Sprintf("RegisterTools should return []global.ToolDefinition, got %T", tools))
}

func TestHubProvider_NewHubProvider(t *testing.T) {
	logger := newTestLogger(t)
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
	logger := newTestLogger(t)
	configs := newTestConfigs()

	provider := NewHubProvider(configs, logger)
	require.NotNil(t, provider)

	// Shutdown without Start should not panic.
	assert.NotPanics(t, func() {
		provider.Shutdown()
	}, "Shutdown without prior Start should not panic")
}
