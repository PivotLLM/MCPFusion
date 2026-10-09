/******************************************************************************
 * Copyright (c) 2026 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import (
	"net/http"
	"net/http/httptest"
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

			at := NewAuthenticatedTransport(underlying, middleware, mlogger.NewMemoryLogger())

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
