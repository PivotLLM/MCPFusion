/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestForwarder creates a progressForwarder suitable for unit tests that
// do not call SendNotificationToClient.
func newTestForwarder(token mcp.ProgressToken) *progressForwarder {
	return &progressForwarder{
		upstreamCtx:   context.Background(),
		upstreamToken: token,
		mcpServer:     nil,
	}
}

func TestProgressForwarder_RegisterAndLoad(t *testing.T) {
	logger := newTestLogger(t)
	mgr := NewMCPClientManager("test-service", logger)

	fwd := newTestForwarder("upstream-1")
	mgr.RegisterProgressForwarder("downstream-1", fwd)

	val, ok := mgr.progressForwarders.Load("downstream-1")
	require.True(t, ok, "forwarder should be loadable after registration")
	assert.Same(t, fwd, val.(*progressForwarder), "loaded forwarder should be the same instance")
}

func TestProgressForwarder_UnregisterRemoves(t *testing.T) {
	logger := newTestLogger(t)
	mgr := NewMCPClientManager("test-service", logger)

	fwd := newTestForwarder("upstream-1")
	mgr.RegisterProgressForwarder("downstream-1", fwd)

	mgr.UnregisterProgressForwarder("downstream-1")

	_, ok := mgr.progressForwarders.Load("downstream-1")
	assert.False(t, ok, "forwarder should not be loadable after unregistration")
}

func TestProgressForwarder_MultipleTokensLifecycle(t *testing.T) {
	logger := newTestLogger(t)
	mgr := NewMCPClientManager("test-service", logger)

	fwdA := newTestForwarder("upstream-A")
	fwdB := newTestForwarder("upstream-B")
	fwdC := newTestForwarder("upstream-C")

	mgr.RegisterProgressForwarder("token-A", fwdA)
	mgr.RegisterProgressForwarder("token-B", fwdB)
	mgr.RegisterProgressForwarder("token-C", fwdC)

	// Unregister only token-B
	mgr.UnregisterProgressForwarder("token-B")

	// token-A should still be present
	val, ok := mgr.progressForwarders.Load("token-A")
	require.True(t, ok, "token-A should still be registered")
	assert.Same(t, fwdA, val.(*progressForwarder))

	// token-B should be gone
	_, ok = mgr.progressForwarders.Load("token-B")
	assert.False(t, ok, "token-B should be unregistered")

	// token-C should still be present
	val, ok = mgr.progressForwarders.Load("token-C")
	require.True(t, ok, "token-C should still be registered")
	assert.Same(t, fwdC, val.(*progressForwarder))
}

func TestProgressForwarder_UnregisterNonExistent(t *testing.T) {
	logger := newTestLogger(t)
	mgr := NewMCPClientManager("test-service", logger)

	assert.NotPanics(t, func() {
		mgr.UnregisterProgressForwarder("does-not-exist")
	}, "unregistering a non-existent token should not panic")
}

func TestProgressForwarder_ConcurrentAccess(t *testing.T) {
	logger := newTestLogger(t)
	mgr := NewMCPClientManager("test-service", logger)

	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				token := fmt.Sprintf("token-%d-%d", id, j)
				fwd := newTestForwarder(mcp.ProgressToken(token))

				mgr.RegisterProgressForwarder(token, fwd)

				// Load to exercise concurrent reads
				mgr.progressForwarders.Load(token)

				mgr.UnregisterProgressForwarder(token)
			}
		}(i)
	}

	wg.Wait()

	// Verify all forwarders have been cleaned up
	count := 0
	mgr.progressForwarders.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count, "all forwarders should be unregistered after concurrent operations")
}
