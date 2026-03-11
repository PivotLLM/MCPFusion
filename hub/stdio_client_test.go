/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PivotLLM/MCPFusion/fusion"
)

func TestResolveCommand_AbsolutePathPassthrough(t *testing.T) {
	logger := newTestLogger(t)

	tests := []struct {
		name    string
		command string
	}{
		{
			name:    "existing absolute path",
			command: "/bin/sh",
		},
		{
			name:    "non-existing absolute path",
			command: "/no/such/path/nonexistent_binary",
		},
		{
			name:    "absolute path with spaces",
			command: "/some/path with spaces/binary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveCommand(tt.command, "", logger)
			assert.Equal(t, tt.command, result, "absolute path should be returned unchanged")
		})
	}
}

func TestResolveCommand_LookPathResolution(t *testing.T) {
	logger := newTestLogger(t)

	// "sh" is available on every Linux system and exec.LookPath should find it.
	result := resolveCommand("sh", "", logger)
	assert.True(t, filepath.IsAbs(result), "resolved path should be absolute, got %q", result)
	assert.Contains(t, result, "sh", "resolved path should contain 'sh'")

	// Verify the resolved path actually exists on disk.
	info, err := os.Stat(result)
	require.NoError(t, err, "resolved path should exist on disk")
	assert.False(t, info.IsDir(), "resolved path should not be a directory")
}

func TestResolveCommand_UnresolvableReturnsAsIs(t *testing.T) {
	logger := newTestLogger(t)

	const bogus = "nonexistent_cmd_xyz_12345"
	result := resolveCommand(bogus, "", logger)
	assert.Equal(t, bogus, result, "unresolvable command should be returned as-is")
}

func TestResolveCommand_ExecutableInPATH(t *testing.T) {
	logger := newTestLogger(t)

	// Create a temp directory with an executable file.
	tmpDir := t.TempDir()
	execFile := filepath.Join(tmpDir, "my_test_exec")
	require.NoError(t, os.WriteFile(execFile, []byte("#!/bin/sh\n"), 0755))

	// Prepend the temp directory to PATH so exec.LookPath finds it.
	t.Setenv("PATH", tmpDir+":"+os.Getenv("PATH"))

	result := resolveCommand("my_test_exec", "", logger)
	assert.Equal(t, execFile, result, "should resolve to the executable in the temp dir")
}

func TestResolveCommand_NonExecutableSkippedByLookPath(t *testing.T) {
	logger := newTestLogger(t)

	// Create a temp directory with a non-executable file (0644).
	tmpDir := t.TempDir()
	nonExecFile := filepath.Join(tmpDir, "not_executable_file_abc")
	require.NoError(t, os.WriteFile(nonExecFile, []byte("data"), 0644))

	// Prepend the temp directory to PATH. exec.LookPath requires the
	// executable bit, so this file should not be found.
	t.Setenv("PATH", tmpDir+":"+os.Getenv("PATH"))

	result := resolveCommand("not_executable_file_abc", "", logger)
	// Since the file is not executable, LookPath will skip it.
	// The function should return the command as-is (or via MCP_FUSION_ADD_PATH,
	// which we are not controlling here).
	assert.Equal(t, "not_executable_file_abc", result,
		"non-executable file should not be resolved via LookPath")
}

// ---------------------------------------------------------------------------
// buildEnv unit tests (Fix 1 / Fix 2 TestD)
// ---------------------------------------------------------------------------

// TestBuildEnv_PathMerging covers all four cases of the buildEnv helper.
func TestBuildEnv_PathMerging(t *testing.T) {
	// Helper to convert the []string slice into a map for easy assertion.
	toMap := func(env []string) map[string]string {
		m := make(map[string]string)
		for _, e := range env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				m[parts[0]] = parts[1]
			}
		}
		return m
	}

	t.Run("empty config env no addPath returns nil", func(t *testing.T) {
		result := buildEnv(nil, "")
		assert.Nil(t, result, "should return nil when nothing to add")
	})

	t.Run("non-nil empty config, no addPath → nil result", func(t *testing.T) {
		result := buildEnv(map[string]string{}, "")
		assert.Nil(t, result, "non-nil but empty config with no addPath should return nil")
	})

	t.Run("config env with custom vars no addPath", func(t *testing.T) {
		configEnv := map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		}
		result := buildEnv(configEnv, "")
		require.NotNil(t, result)
		// buildEnv seeds from os.Environ so the result may contain more than just
		// the configured keys; verify the configured values are present and correct.
		m := toMap(result)
		assert.Equal(t, "bar", m["FOO"])
		assert.Equal(t, "qux", m["BAZ"])
		// When addPath is empty no custom PATH modification should have occurred;
		// the inherited os PATH (if any) passes through unchanged.
	})

	t.Run("config env without PATH addPath set uses os PATH", func(t *testing.T) {
		osPath := os.Getenv("PATH")
		configEnv := map[string]string{
			"FOO": "bar",
		}
		result := buildEnv(configEnv, "/extra/bin")
		require.NotNil(t, result)
		m := toMap(result)
		expectedPath := "/extra/bin:" + osPath
		assert.Equal(t, expectedPath, m["PATH"],
			"addPath should be prepended to os.Getenv(PATH) when config has no PATH")
	})

	t.Run("config env WITH PATH set addPath prepended to config PATH not os PATH", func(t *testing.T) {
		configEnv := map[string]string{
			"PATH": "/config/bin:/usr/local/bin",
			"FOO":  "bar",
		}
		result := buildEnv(configEnv, "/extra/bin")
		require.NotNil(t, result)
		m := toMap(result)
		expectedPath := "/extra/bin:/config/bin:/usr/local/bin"
		assert.Equal(t, expectedPath, m["PATH"],
			"addPath should be prepended to the config PATH, not os PATH")
		// Ensure the os PATH is not accidentally included.
		osPath := os.Getenv("PATH")
		if osPath != "" {
			assert.NotContains(t, m["PATH"], osPath,
				"os PATH should not appear when config already specifies PATH")
		}
	})
}

// ---------------------------------------------------------------------------
// StdioClient integration tests (Fix 2, Tests A–C)
// ---------------------------------------------------------------------------

// TestStdioClient_Connect_InvalidCommand verifies that Connect returns an error
// when the subprocess binary does not exist.
func TestStdioClient_Connect_InvalidCommand(t *testing.T) {
	logger := newTestLogger(t)
	cfg := &fusion.ServiceConfig{
		ServiceKey: "test_invalid",
		Name:       "Test Invalid Command",
		Transport:  fusion.TransportTypeStdio,
		Command:    "/nonexistent/binary/xyz",
	}

	sc := NewStdioClient(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sc.Connect(ctx)
	assert.Error(t, err, "Connect should return an error for a non-existent command")
}

// TestStdioClient_Close_Unconnected verifies that calling Close on a client
// that has never been connected does not panic or return an unexpected error.
func TestStdioClient_Close_Unconnected(t *testing.T) {
	logger := newTestLogger(t)
	cfg := &fusion.ServiceConfig{
		ServiceKey: "test_close",
		Name:       "Test Close Unconnected",
		Transport:  fusion.TransportTypeStdio,
		Command:    "/bin/cat",
	}

	sc := NewStdioClient(cfg, logger)

	assert.NotPanics(t, func() {
		_ = sc.Close()
	}, "Close on an unconnected client should not panic")
}

// TestStdioClient_RunWithReconnect_ContextCancellation starts RunWithReconnect
// with a nonexistent command so that Connect fails immediately, then cancels
// the context and verifies the goroutine exits within a reasonable deadline.
//
// Using a nonexistent binary ensures Connect returns fast (NewStdioMCPClient
// fails at process-start time) so the onDisconnected callback fires before we
// need to cancel the context.
func TestStdioClient_RunWithReconnect_ContextCancellation(t *testing.T) {
	logger := newTestLogger(t)
	cfg := &fusion.ServiceConfig{
		ServiceKey: "test_reconnect",
		Name:       "Test Reconnect Cancel",
		Transport:  fusion.TransportTypeStdio,
		Command:    "/nonexistent/binary/xyz_reconnect_test",
	}

	sc := NewStdioClient(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// disconnected is signalled by onDisconnected when the first Connect
	// attempt fails (fast, because the binary does not exist). This confirms
	// the goroutine has started and executed at least one iteration before we
	// cancel the context, replacing the previous time.Sleep(200ms).
	disconnected := make(chan struct{}, 1)
	onDisconnected := func() {
		select {
		case disconnected <- struct{}{}:
		default:
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc.RunWithReconnect(ctx, nil, onDisconnected)
	}()

	// Wait until the goroutine has made its first (failed) connect attempt.
	select {
	case <-disconnected:
		// First attempt failed and onDisconnected fired — goroutine is running.
	case <-time.After(2 * time.Second):
		t.Fatal("onDisconnected was not called within 2 seconds — goroutine may not have started")
	}

	cancel()

	// The goroutine must exit within 5 seconds after cancellation.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutine exited as expected.
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithReconnect goroutine did not exit within 5 seconds after context cancellation")
	}
}
