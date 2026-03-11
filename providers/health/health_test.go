/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package health_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/PivotLLM/MCPFusion/providers/health"
)

func TestRegisterTools_NoDeps(t *testing.T) {
	p := health.New()
	tools := p.RegisterTools()
	require.Len(t, tools, 1)
	require.Equal(t, "health_status", tools[0].Name)
}

func TestHandleHealth_NoDeps_ReturnsValidJSON(t *testing.T) {
	p := health.New()
	tools := p.RegisterTools()
	require.Len(t, tools, 1)

	result, err := tools[0].Handler(map[string]interface{}{})
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Must be valid JSON.
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &out))

	// Top-level keys must exist.
	_, hasServer := out["server"]
	require.True(t, hasServer, "response must have 'server' key")
	_, hasServices := out["services"]
	require.True(t, hasServices, "response must have 'services' key")
}
