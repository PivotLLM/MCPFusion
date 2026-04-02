/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

// soak_test.go runs a sustained load against an in-process httptest server to
// verify that neither memory nor goroutine counts grow unboundedly over 2 000
// sequential requests.  The test is skipped when -short is given because it
// takes several seconds.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

// TestSoak_MemoryAndGoroutines sends soakRequests sequential requests and
// checks that:
//
//   - HeapAlloc growth is less than soakMaxHeapGrowthMB (100 MB).
//   - Goroutine delta is no more than soakMaxGoroutineDelta (15).
const (
	soakRequests             = 2000
	soakMaxHeapGrowthMB      = 100
	soakMaxGoroutineDelta    = 15
)

func TestSoak_MemoryAndGoroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true,"soak":true}`)
	}))
	defer server.Close()

	f := New(
		WithJSONConfigData([]byte(poolTestConfig(server.URL)), "soak-test.json"),
		WithLogger(nil), // no logger to avoid log buffer growth skewing heap
	)
	tools := f.RegisterTools()
	tool := findTool(t, tools, "pooltest_ping")

	// Warm up: a few requests to let the Go runtime reach a stable state.
	for i := 0; i < 20; i++ {
		_, _ = tool.Handler(withTestContext(map[string]any{}))
	}

	// Baseline measurements after warm-up.
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	goroutinesBefore := runtime.NumGoroutine()

	// Soak run.
	failures := 0
	for i := 0; i < soakRequests; i++ {
		_, err := tool.Handler(withTestContext(map[string]any{}))
		if err != nil {
			failures++
		}
	}

	// Post-soak measurements.
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Allow goroutines to settle.
	goroutinesAfter := liveGoroutineCount()

	t.Logf("soak: %d requests, %d failures", soakRequests, failures)
	t.Logf("heap: before=%d KB after=%d KB", memBefore.HeapAlloc/1024, memAfter.HeapAlloc/1024)
	t.Logf("goroutines: before=%d after=%d delta=%d",
		goroutinesBefore, goroutinesAfter, goroutinesAfter-goroutinesBefore)

	// Tolerate a small number of failures (e.g. race on server close).
	if failures > soakRequests/100 {
		t.Errorf("too many failures during soak: %d/%d", failures, soakRequests)
	}

	// Heap growth check.
	var heapGrowthMB int64
	if memAfter.HeapAlloc > memBefore.HeapAlloc {
		heapGrowthMB = int64(memAfter.HeapAlloc-memBefore.HeapAlloc) / (1024 * 1024)
	}
	if heapGrowthMB > soakMaxHeapGrowthMB {
		t.Errorf("heap grew by %d MB during soak (threshold %d MB) — possible memory leak",
			heapGrowthMB, soakMaxHeapGrowthMB)
	}

	// Goroutine leak check.
	goroutineDelta := goroutinesAfter - goroutinesBefore
	if goroutineDelta > soakMaxGoroutineDelta {
		t.Errorf("goroutine count grew by %d during soak (threshold %d) — possible goroutine leak",
			goroutineDelta, soakMaxGoroutineDelta)
	}
}
