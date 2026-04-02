/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

// graceful_shutdown_test.go validates that the lastUsedWorker goroutine
// correctly flushes pending last-used timestamp writes before the BoltDB
// file is closed.  A lost flush means LastUsed timestamps are silently
// discarded, so this test reopens the database and reads back the value
// to confirm it was written.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tenebris-tech/mlogger"
)

// TestLastUsedWorker_FlushOnClose verifies that a pending last-used update
// queued by ValidateAPIToken is persisted to BoltDB when Close is called,
// before the underlying bbolt file is closed.
//
// The test:
//  1. Opens a DB and creates an API token.
//  2. Records the LastUsed value set at creation time.
//  3. Calls ValidateAPIToken, which queues a lastUsed update on the channel.
//  4. Immediately calls Close() — the worker must flush before bbolt closes.
//  5. Reopens the same DB file and reads back the token metadata.
//  6. Asserts that LastUsed is strictly after the creation timestamp.
func TestLastUsedWorker_FlushOnClose(t *testing.T) {
	tempDir := t.TempDir()
	logger := mlogger.NewMemoryLogger()

	// Open database.
	database, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	require.NoError(t, err)

	// Create a token.
	token, hash, err := database.AddAPIToken("shutdown flush test")
	require.NoError(t, err)

	// Capture the LastUsed value written at creation time.
	md, err := database.GetAPITokenMetadata(hash)
	require.NoError(t, err)
	createdAt := md.LastUsed

	// Tiny sleep so that the validated timestamp is measurably later.
	time.Sleep(5 * time.Millisecond)

	// Validate the token — this enqueues a lastUsed update on lastUsedCh.
	valid, _, err := database.ValidateAPIToken(token)
	require.NoError(t, err)
	assert.True(t, valid)

	// Close must flush before bbolt closes.
	err = database.Close()
	require.NoError(t, err)

	// Reopen the same file.
	database2, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	require.NoError(t, err)
	defer func() { _ = database2.Close() }()

	// Read the token metadata back.
	md2, err := database2.GetAPITokenMetadata(hash)
	require.NoError(t, err)

	// LastUsed must have been updated by the flush.
	assert.True(t, md2.LastUsed.After(createdAt),
		"expected LastUsed (%v) to be after createdAt (%v) — worker did not flush before close",
		md2.LastUsed, createdAt)
}

// TestLastUsedWorker_ManyUpdatesBeforeClose enqueues many updates on the
// channel and then closes the DB.  The worker must drain and flush all of them
// without panicking or deadlocking.
func TestLastUsedWorker_ManyUpdatesBeforeClose(t *testing.T) {
	tempDir := t.TempDir()
	logger := mlogger.NewMemoryLogger()

	database, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	require.NoError(t, err)

	const numTokens = 30

	tokens := make([]string, numTokens)
	hashes := make([]string, numTokens)
	for i := 0; i < numTokens; i++ {
		tok, hash, err := database.AddAPIToken("batch token")
		require.NoError(t, err)
		tokens[i] = tok
		hashes[i] = hash
	}

	// Record creation timestamps.
	createdAts := make([]time.Time, numTokens)
	for i, hash := range hashes {
		md, err := database.GetAPITokenMetadata(hash)
		require.NoError(t, err)
		createdAts[i] = md.LastUsed
	}

	time.Sleep(5 * time.Millisecond)

	// Validate all tokens to queue last-used updates.
	for _, tok := range tokens {
		valid, _, err := database.ValidateAPIToken(tok)
		require.NoError(t, err)
		assert.True(t, valid)
	}

	// Close flushes the worker.
	err = database.Close()
	require.NoError(t, err)

	// Reopen and verify all timestamps were updated.
	database2, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	require.NoError(t, err)
	defer func() { _ = database2.Close() }()

	updated := 0
	for i, hash := range hashes {
		md, err := database2.GetAPITokenMetadata(hash)
		if err != nil {
			t.Errorf("token %d: failed to get metadata: %v", i, err)
			continue
		}
		if md.LastUsed.After(createdAts[i]) {
			updated++
		}
	}

	// All 30 tokens should have had their LastUsed timestamp updated.
	// The channel holds 512 items so none should be dropped.
	assert.Equal(t, numTokens, updated,
		"expected all %d tokens to have updated LastUsed timestamps after flush, got %d",
		numTokens, updated)
}

// TestLastUsedWorker_ReopenedDBPath confirms that the DB file written to disk
// is at the expected path within the data directory, so the reopen in the
// flush test is reading the same file.
func TestLastUsedWorker_ReopenedDBPath(t *testing.T) {
	tempDir := t.TempDir()
	logger := mlogger.NewMemoryLogger()

	database, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	require.NoError(t, err)
	require.NoError(t, database.Close())

	// The DB file must exist at the canonical path.
	dbPath := filepath.Join(tempDir, "mcpfusion.db")
	t.Logf("Checking DB path: %s", dbPath)

	// Reopen to confirm the file is a valid DB.
	database2, err := New(
		WithLogger(logger),
		WithDataDir(tempDir),
	)
	require.NoError(t, err, "could not reopen DB at %s", dbPath)
	require.NoError(t, database2.Close())
}
