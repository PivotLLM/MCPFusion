/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			result := resolveCommand(tt.command, logger)
			assert.Equal(t, tt.command, result, "absolute path should be returned unchanged")
		})
	}
}

func TestResolveCommand_LookPathResolution(t *testing.T) {
	logger := newTestLogger(t)

	// "sh" is available on every Linux system and exec.LookPath should find it.
	result := resolveCommand("sh", logger)
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
	result := resolveCommand(bogus, logger)
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

	result := resolveCommand("my_test_exec", logger)
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

	result := resolveCommand("not_executable_file_abc", logger)
	// Since the file is not executable, LookPath will skip it.
	// The function should return the command as-is (or via MCP_FUSION_ADD_PATH,
	// which we are not controlling here).
	assert.Equal(t, "not_executable_file_abc", result,
		"non-executable file should not be resolved via LookPath")
}
