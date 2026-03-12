/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package perf_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/PivotLLM/MCPFusion/providers/perf"
)

// helpers to find tools by name from RegisterTools output.
func findTool(t *testing.T, p *perf.Provider, name string) func(map[string]interface{}) (string, error) {
	t.Helper()
	for _, tool := range p.RegisterTools() {
		if tool.Name == name {
			return tool.Handler
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

// TestEcho verifies the echo tool returns the input message.
func TestEcho(t *testing.T) {
	p := perf.New()
	handler := findTool(t, p, "perf_echo")

	result, err := handler(map[string]interface{}{"message": "hello world"})
	require.NoError(t, err)

	var out map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &out))
	require.Equal(t, "hello world", out["message"])
}

// TestDelay_ContextCancel verifies that the delay tool respects context cancellation.
func TestDelay_ContextCancel(t *testing.T) {
	p := perf.New()
	handler := findTool(t, p, "perf_delay")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := handler(map[string]interface{}{
		"seconds":       30.0,
		"__mcp_context": ctx,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "context")
}

// TestDelay_Completes verifies the delay tool returns slept_seconds.
func TestDelay_Completes(t *testing.T) {
	p := perf.New()
	handler := findTool(t, p, "perf_delay")

	result, err := handler(map[string]interface{}{"seconds": 0.0})
	require.NoError(t, err)

	var out map[string]float64
	require.NoError(t, json.Unmarshal([]byte(result), &out))
	require.Equal(t, 0.0, out["slept_seconds"])
}

// TestRandomData_Cap verifies the random data tool caps at 1 MiB.
func TestRandomData_Cap(t *testing.T) {
	p := perf.New()
	handler := findTool(t, p, "perf_random_data")

	// Request more than 1 MiB — should be capped.
	result, err := handler(map[string]interface{}{"bytes": float64(2_000_000)})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &out))
	bytesVal, ok := out["bytes"].(float64)
	require.True(t, ok)
	require.Equal(t, float64(1_048_576), bytesVal)

	dataStr, ok := out["data"].(string)
	require.True(t, ok)
	// Each byte is 2 hex chars.
	require.Equal(t, 1_048_576*2, len(dataStr))
}

// TestError_ReturnsError verifies the error tool always returns an error.
func TestError_DefaultMessage(t *testing.T) {
	p := perf.New()
	handler := findTool(t, p, "perf_error")

	_, err := handler(map[string]interface{}{})
	require.Error(t, err)
	require.Equal(t, "perf provider error", err.Error())
}

func TestError_CustomMessage(t *testing.T) {
	p := perf.New()
	handler := findTool(t, p, "perf_error")

	_, err := handler(map[string]interface{}{"message": "boom"})
	require.Error(t, err)
	require.Equal(t, "boom", err.Error())
}

// TestCounter_Concurrent verifies the counter is monotonically increasing under
// concurrent calls.
func TestCounter_Concurrent(t *testing.T) {
	p := perf.New()
	handler := findTool(t, p, "perf_counter")

	const goroutines = 100
	results := make([]int64, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result, err := handler(map[string]interface{}{})
			if err != nil {
				t.Errorf("counter error: %v", err)
				return
			}
			var out map[string]int64
			if err := json.Unmarshal([]byte(result), &out); err != nil {
				t.Errorf("unmarshal error: %v", err)
				return
			}
			results[idx] = out["count"]
		}(i)
	}

	wg.Wait()

	// All values must be unique (each increment is atomic).
	seen := make(map[int64]bool, goroutines)
	for _, v := range results {
		require.False(t, seen[v], "duplicate counter value %d", v)
		seen[v] = true
	}

	// Values must span 1..100 (a fresh provider starts at 0).
	var min, max int64 = results[0], results[0]
	for _, v := range results {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	require.Equal(t, int64(1), min)
	require.Equal(t, int64(goroutines), max)
}
