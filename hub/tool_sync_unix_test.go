//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatCallToolResult_ImageFileModesUnderUmask verifies that the modes in
// FormatOptions are applied exactly under a umask that would narrow them, to
// every folder created. Not parallel: the umask is process-wide.
func TestFormatCallToolResult_ImageFileModesUnderUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })

	img := mcp.NewImageContent(base64.StdEncoding.EncodeToString([]byte("png")), "image/png")
	result := &mcp.CallToolResult{Content: []mcp.Content{img}}

	dlDir := filepath.Join(t.TempDir(), "dl")
	out := FormatCallToolResult(result, &FormatOptions{DownloadDir: dlDir, TenantHash: "t1", DirMode: 0o770, FileMode: 0o660})
	assert.Contains(t, out, "Image saved:")

	saveDir := filepath.Join(dlDir, "t1")
	for _, dir := range []string{dlDir, saveDir} {
		di, err := os.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o770), di.Mode().Perm(), dir)
	}

	entries, err := os.ReadDir(saveDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	fi, err := entries[0].Info()
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o660), fi.Mode().Perm())
}
