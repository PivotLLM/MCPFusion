/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

// connection_pool_test.go validates that the HTTP connection pool is not
// exhausted by sustained request load, that response bodies from failed or
// retried requests are correctly drained, and that the server remains
// functional after encountering repeated upstream errors.
//
// All tests are self-contained: they spin up an in-process httptest.Server so
// no external network access is required.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/tenebris-tech/mlogger"
)

// poolTestConfig returns a Fusion JSON config that points at baseURL and has
// retry enabled so that we exercise the body-drain path on 5xx responses.
func poolTestConfig(baseURL string) string {
	return `{
		"services": {
			"pooltest": {
				"name": "Pool Test API",
				"baseURL": "` + baseURL + `",
				"auth": {
					"type": "bearer",
					"config": { "token": "test-token" }
				},
				"retry": {
					"enabled": true,
					"maxAttempts": 3,
					"strategy": "exponential",
					"baseDelay": "10ms",
					"maxDelay":  "50ms",
					"jitter": false,
					"backoffFactor": 2.0
				},
				"endpoints": [
					{
						"id": "ping",
						"name": "Ping",
						"description": "Simple GET that returns a JSON object",
						"method": "GET",
						"path": "/ping",
						"parameters": [],
						"response": { "type": "json" }
					}
				]
			}
		}
	}`
}

// findTool locates a tool by name in a slice of tool definitions.
func findTool(t *testing.T, tools []global.ToolDefinition, name string) *global.ToolDefinition {
	t.Helper()
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

// liveGoroutineCount returns the current goroutine count after allowing a
// brief settling period for goroutines started by the previous operation to
// either finish or stabilise.
func liveGoroutineCount() int {
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	return runtime.NumGoroutine()
}

// TestConnectionPool_SequentialLoad sends a large number of sequential
// requests through a Fusion instance, deliberately injecting periodic 5xx
// responses to exercise the retry body-drain path.  After all requests it
// verifies that:
//
//  1. The vast majority of requests succeed (pool not exhausted mid-run).
//  2. The goroutine count has not grown unboundedly.
//  3. A fresh request made after the load run still succeeds (pool healthy).
func TestConnectionPool_SequentialLoad(t *testing.T) {
	const total = 200 // total requests to send
	const errorEvery = 7 // inject a 5xx every N-th request

	var requestCount atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		if n%errorEvery == 0 {
			// Return a 5xx with a body to exercise the drain path.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"error":"simulated server error","detail":"this body must be drained before the connection can be reused"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"ok":true,"n":%d}`, n)
	}))
	defer server.Close()

	f := New(
		WithJSONConfigData([]byte(poolTestConfig(server.URL)), "pool-test.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "pooltest_ping")

	baselineGoroutines := liveGoroutineCount()

	successes := 0
	for i := 0; i < total; i++ {
		_, err := tool.Handler(withTestContext(map[string]any{}))
		if err == nil {
			successes++
		}
	}

	afterLoadGoroutines := liveGoroutineCount()

	// We injected a 5xx every 7th request.  With 3 retry attempts each of
	// those ultimately fails, so we expect ~(total - total/errorEvery)
	// successes.  Allow a generous margin.
	minExpectedSuccesses := total - (total/errorEvery)*2
	if successes < minExpectedSuccesses {
		t.Errorf("too many failures during load: got %d/%d successes (want >= %d)",
			successes, total, minExpectedSuccesses)
	}

	// Goroutine count should not have grown by more than a small constant.
	// A large delta here indicates a goroutine leak.
	const goroutineLeakThreshold = 10
	delta := afterLoadGoroutines - baselineGoroutines
	if delta > goroutineLeakThreshold {
		t.Errorf("goroutine leak detected: baseline=%d after-load=%d delta=%d (threshold=%d)",
			baselineGoroutines, afterLoadGoroutines, delta, goroutineLeakThreshold)
	}

	// Pool health check: one final request must succeed after all the errors.
	_, err := tool.Handler(withTestContext(map[string]any{}))
	if err != nil {
		t.Errorf("post-load health check failed — connection pool may be exhausted: %v", err)
	}
}

// TestConnectionPool_ConcurrentLoad fires many requests concurrently to stress
// the connection pool.  It verifies that the pool handles concurrent access
// without deadlock, excessive goroutine growth, or widespread failures.
func TestConnectionPool_ConcurrentLoad(t *testing.T) {
	const concurrency = 50

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	f := New(
		WithJSONConfigData([]byte(poolTestConfig(server.URL)), "pool-test.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "pooltest_ping")

	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := tool.Handler(withTestContext(map[string]any{}))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	failures := 0
	for err := range errs {
		if err != nil {
			failures++
		}
	}

	if failures > 0 {
		t.Errorf("concurrent load: %d/%d requests failed", failures, concurrency)
	}

	// After concurrent load verify a further burst still all succeeds — if the
	// pool were exhausted by the first wave, these would time out or error.
	for i := 0; i < 10; i++ {
		if _, err := tool.Handler(withTestContext(map[string]any{})); err != nil {
			t.Errorf("post-concurrent-load health check %d failed: %v", i+1, err)
		}
	}
}

// TestConnectionPool_RetryBodyDrain specifically targets the body-drain fix.
// The mock server returns 5xx for every request so that all retry attempts
// fail.  After exhausting retries for many requests, the pool must still be
// able to serve a final successful request.  Without the drain fix, this test
// would exhaust the connection pool (MaxConnsPerHost = 50) and the final
// request would time out.
func TestConnectionPool_RetryBodyDrain(t *testing.T) {
	// Send enough requests to exhaust MaxConnsPerHost (50) if bodies are not
	// drained.  With 3 retry attempts each, 20 requests × 3 = 60 abandoned
	// connections — enough to fill the pool and block.
	const errorRequests = 20

	// Track whether we should serve errors or success.
	var serveErrors atomic.Bool
	serveErrors.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveErrors.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, `{"error":"unavailable","detail":"body that must be drained on retry"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	f := New(
		WithJSONConfigData([]byte(poolTestConfig(server.URL)), "pool-test.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "pooltest_ping")

	// Phase 1: fire errorRequests requests, all of which exhaust their retries.
	for i := 0; i < errorRequests; i++ {
		_, _ = tool.Handler(withTestContext(map[string]any{})) // errors expected; ignore them
	}

	// Phase 2: switch the server to success mode and verify the pool is healthy.
	serveErrors.Store(false)

	result, err := tool.Handler(withTestContext(map[string]any{}))
	if err != nil {
		t.Fatalf("connection pool exhausted after %d error requests (body drain may be broken): %v",
			errorRequests, err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unexpected response after pool drain test: %v", err)
	}
	if parsed["ok"] != true {
		t.Errorf("unexpected response body: %s", result)
	}
}

// TestConnectionPool_MixedErrorLoad alternates between successful and failing
// requests to simulate a realistic degraded-upstream scenario, then verifies
// the pool remains fully operational at the end.
func TestConnectionPool_MixedErrorLoad(t *testing.T) {
	const total = 150
	const errorEvery = 5

	var requestCount atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		switch {
		case n%errorEvery == 0:
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprint(w, `{"error":"bad gateway"}`)
		case n%errorEvery == 1:
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":"rate limited"}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"ok":true,"n":%d}`, n)
		}
	}))
	defer server.Close()

	f := New(
		WithJSONConfigData([]byte(poolTestConfig(server.URL)), "pool-test.json"),
		WithLogger(mlogger.NewMemoryLogger()),
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "pooltest_ping")

	for i := 0; i < total; i++ {
		_, _ = tool.Handler(withTestContext(map[string]any{}))
	}

	// After mixed load the pool must still serve successful requests.
	const finalChecks = 5
	for i := 0; i < finalChecks; i++ {
		_, err := tool.Handler(withTestContext(map[string]any{}))
		if err != nil {
			t.Errorf("post-load check %d/%d failed — pool may be exhausted: %v",
				i+1, finalChecks, err)
		}
	}
}
