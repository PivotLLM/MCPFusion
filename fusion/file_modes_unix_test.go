//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestBinaryResponse_FileModesUnderUmask verifies that the modes set by
// WithFileModes are applied exactly under a umask that would narrow them.
// Not parallel: the umask is process-wide.
func TestBinaryResponse_FileModesUnderUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })

	dlDir := filepath.Join(t.TempDir(), "dl")
	h := newBinaryHandler(t, WithDownloadDir(dlDir), WithFileModes(0o770, 0o660))

	resp := syntheticResponse(200, "application/pdf", []byte("%PDF-1.4"))
	if _, err := h.handleResponse(context.Background(), resp, "corr", nil); err != nil {
		t.Fatalf("handleResponse: %v", err)
	}

	di, err := os.Stat(dlDir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := di.Mode().Perm(); got != 0o770 {
		t.Errorf("dir mode = %o, want 770", got)
	}
	entries, err := os.ReadDir(dlDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one saved file, got %d (err %v)", len(entries), err)
	}
	fi, err := entries[0].Info()
	if err != nil {
		t.Fatalf("file info: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o660 {
		t.Errorf("file mode = %o, want 660", got)
	}
}
