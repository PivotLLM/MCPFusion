/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package main

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tenebris-tech/mlogger"

	"github.com/PivotLLM/MCPFusion/db"
)

// TestServerRunReleasesOnStartFailure verifies that run closes the database
// when the MCP server cannot listen because the port is already in use.
func TestServerRunReleasesOnStartFailure(t *testing.T) {
	logger := mlogger.NewMemoryLogger()

	database, err := db.New(db.WithLogger(logger), db.WithDataDir(t.TempDir()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })

	srv := server{
		logger:   logger,
		database: database,
		listen:   busy.Addr().String(),
	}

	err = srv.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MCP server failed to start")
	assert.Contains(t, logger.Logs(), "INFO: Database connection closed successfully")
}
