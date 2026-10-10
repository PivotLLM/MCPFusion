/******************************************************************************
 * Copyright (c) 2026 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tenebris-tech/mlogger"
)

// TestNewAuthenticatedTransport_AppliesMiddleware ensures every request served by
// the wrapper passes through the middleware before reaching the transport.
func TestNewAuthenticatedTransport_AppliesMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		allow          bool
		wantStatus     int
		wantUnderlying bool
	}{
		{name: "middleware rejects", allow: false, wantStatus: http.StatusUnauthorized, wantUnderlying: false},
		{name: "middleware allows", allow: true, wantStatus: http.StatusOK, wantUnderlying: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reached := false
			underlying := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			})
			middleware := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !tt.allow {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					next.ServeHTTP(w, r)
				})
			}

			at := NewAuthenticatedTransport(underlying, middleware, WithLogger(mlogger.NewMemoryLogger()))

			rec := httptest.NewRecorder()
			at.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mcp", nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if reached != tt.wantUnderlying {
				t.Errorf("underlying reached = %v, want %v", reached, tt.wantUnderlying)
			}
		})
	}
}

// TestMCPServerStart_AddressInUse ensures Start fails when the listen address
// is already bound, instead of leaving the server running with nothing listening.
func TestMCPServerStart_AddressInUse(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind test listener: %v", err)
	}
	defer func() { _ = busy.Close() }()

	s, err := New(WithLogger(mlogger.NewMemoryLogger()), WithListen(busy.Addr().String()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := s.Start(); err == nil {
		_ = s.Stop()
		t.Fatal("Start() error = nil, want error for an address already in use")
	}
}

// TestMCPServerStartStop ensures a normal shutdown is not reported as an error.
func TestMCPServerStartStop(t *testing.T) {
	logger := mlogger.NewMemoryLogger()
	s, err := New(WithLogger(logger), WithListen("127.0.0.1:0"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	for _, line := range logger.Logs() {
		if strings.HasPrefix(line, "ERROR:") {
			t.Errorf("unexpected error logged on shutdown: %s", line)
		}
	}
}
