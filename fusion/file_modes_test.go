/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/tenebris-tech/mlogger"
)

func newBinaryHandler(t *testing.T, opts ...Option) *HTTPHandler {
	t.Helper()
	config := &Config{
		Services: map[string]*ServiceConfig{
			"testsvc": {
				Name:    "testsvc",
				BaseURL: "http://localhost",
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]any{"token": "test-token"},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "testep",
						Name:        "Test Endpoint",
						Description: "Test",
						Method:      "GET",
						Path:        "/test",
						Parameters:  []ParameterConfig{},
						Response:    ResponseConfig{Type: ResponseTypeBinary},
					},
				},
			},
		},
	}
	opts = append([]Option{WithConfig(config), WithLogger(mlogger.NewMemoryLogger())}, opts...)
	f, err := New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	svc := config.Services["testsvc"]
	return NewHTTPHandler(f, svc, &svc.Endpoints[0])
}

func TestWithFileModes(t *testing.T) {
	tests := []struct {
		name     string
		opts     []Option
		wantDir  fs.FileMode
		wantFile fs.FileMode
	}{
		{"defaults", nil, global.DefaultDirMode, global.DefaultFileMode},
		{"both set", []Option{WithFileModes(0700, 0600)}, 0700, 0600},
		{"zero dir keeps default", []Option{WithFileModes(0, 0600)}, global.DefaultDirMode, 0600},
		{"zero file keeps default", []Option{WithFileModes(0700, 0)}, 0700, global.DefaultFileMode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := New(append([]Option{WithLogger(mlogger.NewMemoryLogger())}, tt.opts...)...)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if f.dirMode != tt.wantDir {
				t.Errorf("dirMode = %o, want %o", f.dirMode, tt.wantDir)
			}
			if f.fileMode != tt.wantFile {
				t.Errorf("fileMode = %o, want %o", f.fileMode, tt.wantFile)
			}
		})
	}
}

// TestBinaryResponse_FileModes verifies that a saved binary response uses the
// modes set by WithFileModes. 0700/0600 are unaffected by any usual umask.
func TestBinaryResponse_FileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions not applicable on Windows")
	}
	dlDir := filepath.Join(t.TempDir(), "dl")
	h := newBinaryHandler(t, WithDownloadDir(dlDir), WithFileModes(0700, 0600))

	resp := syntheticResponse(200, "application/pdf", []byte("%PDF-1.4"))
	if _, err := h.handleResponse(context.Background(), resp, "corr", nil); err != nil {
		t.Fatalf("handleResponse: %v", err)
	}

	di, err := os.Stat(dlDir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := di.Mode().Perm(); got != 0700 {
		t.Errorf("dir mode = %o, want 700", got)
	}

	entries, err := os.ReadDir(dlDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one saved file, got %d (err %v)", len(entries), err)
	}
	fi, err := entries[0].Info()
	if err != nil {
		t.Fatalf("file info: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0600 {
		t.Errorf("file mode = %o, want 600", got)
	}
}
