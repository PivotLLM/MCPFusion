/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

// token_concurrent_test.go validates that AddAPIToken, ValidateAPIToken,
// DeleteAPIToken, and ListAPITokens are safe to call from many goroutines
// simultaneously.  Run with -race to detect data races.

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tenebris-tech/mlogger"
)

// TestTokenConcurrentLifecycle spins up many goroutines that concurrently
// create, validate, list, and delete API tokens.  It verifies:
//   - No panics (via recover in each goroutine).
//   - No data races (run with -race).
//   - The DB is still usable for a fresh operation after the concurrent load.
func TestTokenConcurrentLifecycle(t *testing.T) {
	const numWorkers = 40
	const opsPerWorker = 10

	tempDir := t.TempDir()
	logger := mlogger.NewMemoryLogger()

	database, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	var wg sync.WaitGroup
	panicCh := make(chan any, numWorkers)
	errCh := make(chan error, numWorkers*opsPerWorker)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()

			for op := 0; op < opsPerWorker; op++ {
				desc := fmt.Sprintf("worker-%d-op-%d", workerID, op)

				// AddAPIToken
				token, hash, err := database.AddAPIToken(desc)
				if err != nil {
					errCh <- fmt.Errorf("worker %d op %d AddAPIToken: %w", workerID, op, err)
					continue
				}

				// ValidateAPIToken
				valid, returnedHash, err := database.ValidateAPIToken(token)
				if err != nil {
					errCh <- fmt.Errorf("worker %d op %d ValidateAPIToken: %w", workerID, op, err)
				} else if !valid || returnedHash != hash {
					errCh <- fmt.Errorf("worker %d op %d ValidateAPIToken: valid=%v hashMatch=%v", workerID, op, valid, returnedHash == hash)
				}

				// ListAPITokens (concurrent reads must not race with writes)
				_, listErr := database.ListAPITokens()
				if listErr != nil {
					errCh <- fmt.Errorf("worker %d op %d ListAPITokens: %w", workerID, op, listErr)
				}

				// DeleteAPIToken
				if delErr := database.DeleteAPIToken(hash); delErr != nil {
					errCh <- fmt.Errorf("worker %d op %d DeleteAPIToken: %w", workerID, op, delErr)
				}
			}
		}(w)
	}

	wg.Wait()
	close(panicCh)
	close(errCh)

	// Fail on any panics.
	for p := range panicCh {
		t.Errorf("goroutine panicked: %v", p)
	}

	// Fail on any operation errors.
	for e := range errCh {
		t.Errorf("concurrent operation error: %v", e)
	}

	// DB must still be usable after concurrent load.
	_, _, err = database.AddAPIToken("post-load health check")
	require.NoError(t, err, "DB unusable after concurrent lifecycle test")
}

// TestTokenConcurrentValidations hammers ValidateAPIToken from many goroutines
// against the same set of tokens to exercise concurrent reads and the
// lastUsedCh write path without introducing races.
func TestTokenConcurrentValidations(t *testing.T) {
	const numTokens = 20
	const numWorkers = 50

	tempDir := t.TempDir()
	logger := mlogger.NewMemoryLogger()

	database, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	// Create the tokens serially before the concurrent phase.
	tokens := make([]string, numTokens)
	for i := 0; i < numTokens; i++ {
		tok, _, err := database.AddAPIToken(fmt.Sprintf("concurrent-validate-%d", i))
		require.NoError(t, err)
		tokens[i] = tok
	}

	var wg sync.WaitGroup
	panicCh := make(chan any, numWorkers)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()

			for _, tok := range tokens {
				valid, _, err := database.ValidateAPIToken(tok)
				if err != nil {
					t.Errorf("worker %d ValidateAPIToken error: %v", workerID, err)
					return
				}
				if !valid {
					t.Errorf("worker %d: expected valid token, got invalid", workerID)
					return
				}
			}
		}(w)
	}

	wg.Wait()
	close(panicCh)

	for p := range panicCh {
		t.Errorf("goroutine panicked: %v", p)
	}
}
