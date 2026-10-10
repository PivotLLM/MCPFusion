/******************************************************************************
 * Copyright (c) 2026 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/tenebris-tech/mlogger"
)

type stubRoutes struct{}

func (stubRoutes) RegisterOAuthRoutes(*http.ServeMux) {}

// stubTransport is an MCPServerTransport that is also an http.Handler.
type stubTransport struct{}

func (stubTransport) Start(string) error                           { return nil }
func (stubTransport) Shutdown(context.Context) error               { return nil }
func (stubTransport) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestNewExtendedTransport_Logger(t *testing.T) {
	tests := []struct {
		name     string
		logger   *mlogger.MemoryLogger
		wantLogs bool
	}{
		{name: "with logger", logger: mlogger.NewMemoryLogger(), wantLogs: true},
		{name: "without logger", logger: nil, wantLogs: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []TransportOption
			if tt.logger != nil {
				opts = append(opts, WithLogger(tt.logger))
			}

			et := NewExtendedTransport(stubTransport{}, stubTransport{}, stubRoutes{}, nil, opts...)
			if et == nil {
				t.Fatal("NewExtendedTransport returned nil")
			}
			if tt.wantLogs && len(tt.logger.Logs()) == 0 {
				t.Error("expected the transport to log through WithLogger")
			}
		})
	}
}
