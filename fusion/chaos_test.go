/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

// chaos_test.go exercises fault injection scenarios using httptest servers
// that simulate real-world transport failures:
//
//   - Connection reset mid-response (hijack the TCP connection and close it
//     after writing partial response headers).
//   - Slow response that exceeds the outbound timeout (handler sleeps longer
//     than the endpoint timeout; verify the call returns promptly with an error,
//     not after the full sleep).
//   - Pool health after each chaos scenario: a subsequent normal request must
//     succeed to confirm the connection pool is not broken.

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tenebris-tech/mlogger"
)

// chaosTestConfig returns a Fusion JSON config for chaos tests.  The endpoint
// timeout is set short so that slow-server tests complete quickly.
func chaosTestConfig(baseURL string, timeoutDuration string) string {
	if timeoutDuration == "" {
		timeoutDuration = "2s"
	}
	return `{
		"services": {
			"chaos": {
				"name": "Chaos Test API",
				"baseURL": "` + baseURL + `",
				"auth": {
					"type": "bearer",
					"config": { "token": "test-token" }
				},
				"retry": {
					"enabled": false
				},
				"endpoints": [
					{
						"id": "ping",
						"name": "Ping",
						"description": "Chaos test endpoint",
						"method": "GET",
						"path": "/ping",
						"parameters": [],
						"response": { "type": "json" },
						"connection": {
							"timeout": "` + timeoutDuration + `"
						}
					}
				]
			}
		}
	}`
}

// TestChaos_ConnectionReset verifies that a connection reset mid-response
// (simulated by hijacking and closing the TCP connection after writing partial
// headers) causes the handler to return an error rather than hang, and that
// the pool is still healthy for a subsequent normal request.
func TestChaos_ConnectionReset(t *testing.T) {
	var serveReset atomic.Bool
	serveReset.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveReset.Load() {
			// Hijack the connection and close it abruptly after writing partial headers.
			hj, ok := w.(http.Hijacker)
			if !ok {
				// httptest.Server always supports hijacking; fail fast if not.
				http.Error(w, "hijack not supported", http.StatusInternalServerError)
				return
			}
			conn, bufrw, err := hj.Hijack()
			if err != nil {
				return
			}
			// Write a partial HTTP/1.1 response line then slam the connection shut.
			_, _ = bufrw.WriteString("HTTP/1.1 200 OK\r\nContent-Typ")
			_ = bufrw.Flush()
			_ = conn.Close()
			return
		}
		// Normal response after chaos phase.
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	f := New(
		WithJSONConfigData([]byte(chaosTestConfig(server.URL, "5s")), "chaos-reset.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "chaos_ping")

	// Phase 1: connection reset must return an error.
	_, err := tool.Handler(withTestContext(map[string]any{}))
	if err == nil {
		t.Error("expected an error from connection reset, got nil")
	}

	// Phase 2: pool health check — normal request must succeed after the reset.
	serveReset.Store(false)
	_, err = tool.Handler(withTestContext(map[string]any{}))
	if err != nil {
		t.Errorf("pool unhealthy after connection reset — subsequent request failed: %v", err)
	}
}

// TestChaos_SlowResponseTimeout verifies that an endpoint whose outbound
// timeout (connection.timeout) is shorter than the server's response delay
// returns an error promptly and does not block for the full server sleep.
func TestChaos_SlowResponseTimeout(t *testing.T) {
	const serverSleep = 3 * time.Second
	const endpointTimeout = "500ms"

	var serveSlowly atomic.Bool
	serveSlowly.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveSlowly.Load() {
			time.Sleep(serverSleep)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	f := New(
		WithJSONConfigData([]byte(chaosTestConfig(server.URL, endpointTimeout)), "chaos-slow.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "chaos_ping")

	start := time.Now()
	_, err := tool.Handler(withTestContext(map[string]any{}))
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error from slow server, got nil")
	}

	// The call must return well before the server's sleep duration.
	// Allow generous margin: 2× the endpoint timeout.
	maxAllowed := 2 * time.Second
	if elapsed > maxAllowed {
		t.Errorf("handler took %v to return (want < %v) — timeout may not be honouring connection.timeout",
			elapsed, maxAllowed)
	}

	// Pool health check after timeout.
	serveSlowly.Store(false)
	_, err = tool.Handler(withTestContext(map[string]any{}))
	if err != nil {
		t.Errorf("pool unhealthy after slow-response timeout — subsequent request failed: %v", err)
	}
}

// TestChaos_PoolHealthAfterErrors sends several requests that will fail due to
// the server closing connections immediately, then verifies the pool is still
// functional.
func TestChaos_PoolHealthAfterErrors(t *testing.T) {
	const badRequests = 10

	var serveErrors atomic.Bool
	serveErrors.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveErrors.Load() {
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "no hijack", http.StatusInternalServerError)
				return
			}
			conn, bufrw, err := hj.Hijack()
			if err != nil {
				return
			}
			// Write nothing — just close the connection.
			_ = bufrw.Flush()
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	f := New(
		WithJSONConfigData([]byte(chaosTestConfig(server.URL, "5s")), "chaos-pool.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "chaos_ping")

	for i := 0; i < badRequests; i++ {
		_, _ = tool.Handler(withTestContext(map[string]any{}))
	}

	serveErrors.Store(false)

	const healthChecks = 5
	for i := 0; i < healthChecks; i++ {
		_, err := tool.Handler(withTestContext(map[string]any{}))
		if err != nil {
			t.Errorf("pool health check %d/%d failed after %d error requests: %v",
				i+1, healthChecks, badRequests, err)
		}
	}
}

// Ensure the bufio import is used (hijack returns a *bufio.ReadWriter).
var _ = (*bufio.ReadWriter)(nil)
var _ = (net.Conn)(nil)
