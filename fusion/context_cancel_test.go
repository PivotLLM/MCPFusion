/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

// context_cancel_test.go verifies that cancelling the caller's context
// mid-flight causes the HTTP handler to return promptly rather than
// waiting for the full server response time.  It also checks that no
// goroutine leak results from the cancellation.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/tenebris-tech/mlogger"
)

// withCancellableContext creates an args map whose __mcp_context value is a
// context derived from the provided cancellable context but extended with the
// TenantContext that the auth middleware requires.
func withCancellableContext(ctx context.Context) map[string]any {
	tc := &TenantContext{
		TenantHash:  "test-tenant-hash",
		ServiceName: "test",
		CreatedAt:   time.Now(),
	}
	tenantCtx := context.WithValue(ctx, global.TenantContextKey, tc)
	return map[string]any{
		"__mcp_context": tenantCtx,
	}
}

// TestContextCancel_MidFlight starts a goroutine that calls tool.Handler
// against a slow httptest server (sleeps 2 s), cancels the parent context
// after 100 ms, and asserts that the handler returns well within the server
// sleep window.
func TestContextCancel_MidFlight(t *testing.T) {
	const serverSleep = 2 * time.Second
	const cancelAfter = 100 * time.Millisecond
	const wantReturnWithin = 1 * time.Second // generous margin

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(serverSleep)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	// Use a short endpoint timeout that is still longer than cancelAfter so
	// that the context cancellation — not the endpoint timeout — fires first.
	cfg := `{
		"services": {
			"ctxtest": {
				"name": "Context Test API",
				"baseURL": "` + server.URL + `",
				"auth": {
					"type": "bearer",
					"config": { "token": "test-token" }
				},
				"retry": { "enabled": false },
				"endpoints": [
					{
						"id": "ping",
						"name": "Ping",
						"description": "Context cancel test endpoint",
						"method": "GET",
						"path": "/ping",
						"parameters": [],
						"response": { "type": "json" },
						"connection": { "timeout": "5s" }
					}
				]
			}
		}
	}`

	f := New(
		WithJSONConfigData([]byte(cfg), "ctx-cancel.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "ctxtest_ping")

	baselineGoroutines := liveGoroutineCount()

	ctx, cancel := context.WithCancel(context.Background())

	type result struct {
		err     error
		elapsed time.Duration
	}
	resultCh := make(chan result, 1)

	go func() {
		start := time.Now()
		_, err := tool.Handler(withCancellableContext(ctx))
		resultCh <- result{err: err, elapsed: time.Since(start)}
	}()

	// Cancel after a short delay.
	time.Sleep(cancelAfter)
	cancel()

	select {
	case r := <-resultCh:
		if r.err == nil {
			t.Error("expected an error from context cancellation, got nil")
		}
		if r.elapsed > wantReturnWithin {
			t.Errorf("handler took %v to return after context cancel (want < %v) — context propagation broken",
				r.elapsed, wantReturnWithin)
		}
		t.Logf("handler returned in %v after context cancel: %v", r.elapsed, r.err)

	case <-time.After(wantReturnWithin + 500*time.Millisecond):
		cancel() // ensure cancel is called even on timeout
		t.Fatalf("handler did not return within %v after context cancellation", wantReturnWithin)
	}

	// Allow goroutines to settle then verify no leak.
	afterGoroutines := liveGoroutineCount()
	const leakThreshold = 5
	delta := afterGoroutines - baselineGoroutines
	if delta > leakThreshold {
		t.Errorf("goroutine leak after context cancel: baseline=%d after=%d delta=%d (threshold=%d)",
			baselineGoroutines, afterGoroutines, delta, leakThreshold)
	}

	_ = runtime.NumGoroutine() // ensure runtime import is used
}

// TestContextCancel_AlreadyCancelled verifies that if the context is already
// cancelled before Handle is called, the handler returns immediately with an
// error rather than proceeding with the request.
func TestContextCancel_AlreadyCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This handler should not be reached.
		time.Sleep(5 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	cfg := `{
		"services": {
			"precancelled": {
				"name": "Pre-Cancelled Test API",
				"baseURL": "` + server.URL + `",
				"auth": {
					"type": "bearer",
					"config": { "token": "test-token" }
				},
				"retry": { "enabled": false },
				"endpoints": [
					{
						"id": "ping",
						"name": "Ping",
						"description": "Pre-cancelled test endpoint",
						"method": "GET",
						"path": "/ping",
						"parameters": [],
						"response": { "type": "json" }
					}
				]
			}
		}
	}`

	f := New(
		WithJSONConfigData([]byte(cfg), "precancelled.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "precancelled_ping")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	start := time.Now()
	_, err := tool.Handler(withCancellableContext(ctx))
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error for pre-cancelled context, got nil")
	}

	// Should return almost immediately — well under 500 ms.
	if elapsed > 500*time.Millisecond {
		t.Errorf("handler with pre-cancelled context took %v (want < 500ms)", elapsed)
	}
}
