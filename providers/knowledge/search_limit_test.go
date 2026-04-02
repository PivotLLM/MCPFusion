/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package knowledge_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tenebris-tech/mlogger"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/providers/knowledge"
)

// setupSearchLimitDB creates a temporary real database for integration-style
// provider tests and returns the db, a cleanup function, and a valid userID.
func setupSearchLimitDB(t *testing.T) (db.Database, string, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "knowledge_search_limit_")
	require.NoError(t, err)

	logger := mlogger.NewMemoryLogger()
	database, err := db.New(
		db.WithDataDir(tempDir),
		db.WithLogger(logger),
	)
	require.NoError(t, err)

	// Create a user so knowledge operations have a valid owner.
	user, err := database.CreateUser("search-limit-test-user")
	require.NoError(t, err)
	userID := user.UserID

	cleanup := func() {
		_ = database.Close()
		_ = os.RemoveAll(tempDir)
	}

	return database, userID, cleanup
}

// findTool locates a tool by name from a registered slice.
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

// TestKnowledgeSearch_QueryTooLong verifies that a query exceeding 512 characters
// produces an error that surfaces through the provider handler.
func TestKnowledgeSearch_QueryTooLong(t *testing.T) {
	database, userID, cleanup := setupSearchLimitDB(t)
	defer cleanup()

	p := knowledge.New(
		knowledge.WithDatabase(database),
		knowledge.WithUserIDExtractor(func(_ context.Context) (string, error) {
			return userID, nil
		}),
	)

	tools := p.RegisterTools()
	require.NotNil(t, tools)

	searchTool := findTool(t, tools, "knowledge_search")

	// Build a query one character over the 512-character limit.
	longQuery := strings.Repeat("a", 513)

	_, err := searchTool.Handler(map[string]interface{}{
		"query": longQuery,
	})

	require.Error(t, err, "query longer than 512 characters should produce an error")
	assert.True(t,
		strings.Contains(err.Error(), "512") ||
			strings.Contains(strings.ToLower(err.Error()), "exceed") ||
			strings.Contains(strings.ToLower(err.Error()), "maximum"),
		"error message should mention the length limit, got: %s", err.Error(),
	)
}

// TestKnowledgeSearch_QueryAtLimit verifies that a query of exactly 512 characters
// is accepted by the provider (no length-limit error is returned).
func TestKnowledgeSearch_QueryAtLimit(t *testing.T) {
	database, userID, cleanup := setupSearchLimitDB(t)
	defer cleanup()

	p := knowledge.New(
		knowledge.WithDatabase(database),
		knowledge.WithUserIDExtractor(func(_ context.Context) (string, error) {
			return userID, nil
		}),
	)

	tools := p.RegisterTools()
	require.NotNil(t, tools)

	searchTool := findTool(t, tools, "knowledge_search")

	// A query of exactly 512 characters is at the limit and must be accepted.
	atLimitQuery := strings.Repeat("a", 512)

	result, err := searchTool.Handler(map[string]interface{}{
		"query": atLimitQuery,
	})

	require.NoError(t, err, "query at the limit (512 chars) should not produce an error")
	// No entries match, so the result should indicate an empty result set.
	assert.NotEmpty(t, result)
}
