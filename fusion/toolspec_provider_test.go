/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/toolspec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newToolSpecTestFusion builds a Fusion whose single auth:none service targets
// the given base URL, so generated toolspec handlers exercise the real fusion
// request path (which requires a tenant context in the call ctx).
func newToolSpecTestFusion(t *testing.T, baseURL string) *Fusion {
	t.Helper()
	f, err := New(WithConfig(&Config{
		Services: map[string]*ServiceConfig{
			"echo": {
				Name:    "Echo",
				BaseURL: baseURL,
				Auth:    AuthConfig{Type: AuthTypeNone},
				Endpoints: []EndpointConfig{
					{
						ID:          "get",
						Name:        "Get",
						Description: "fetch a value",
						Method:      "GET",
						Path:        "/get",
						Parameters: []ParameterConfig{
							{
								Name:     "id",
								Type:     ParameterTypeString,
								Location: ParameterLocationQuery,
								Required: true,
							},
						},
						Response: ResponseConfig{Type: ResponseTypeText},
					},
				},
			},
		},
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return f
}

func findToolSpec(defs []toolspec.ToolDefinition, name string) *toolspec.ToolDefinition {
	for i := range defs {
		if defs[i].Name == name {
			return &defs[i]
		}
	}
	return nil
}

func TestToolSpecDefinitions_SchemaAndFields(t *testing.T) {
	f := newToolSpecTestFusion(t, "https://api.example.com")

	defs := f.ToolSpecDefinitions("tenant-xyz")

	tool := findToolSpec(defs, "echo_get")
	require.NotNil(t, tool, "endpoint tool should be converted")
	assert.Equal(t, "Echo: fetch a value", tool.Description)
	require.NotNil(t, tool.Handler)

	// Schema present and reflects the declared parameter.
	schema := tool.Schema()
	require.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	_, hasID := props["id"]
	assert.True(t, hasID, "schema should carry the 'id' parameter")
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "id")

	// Hints carried across from the GET default (read-only).
	require.NotNil(t, tool.Hints)
	require.NotNil(t, tool.Hints.ReadOnly)
	assert.True(t, *tool.Hints.ReadOnly)
}

func TestToolSpecDefinitions_HandlerInjectsTenantAndSucceeds(t *testing.T) {
	// The fusion request path rejects a missing tenant context. A 200 response
	// therefore proves the wrapper injected a *TenantContext for the call.
	var sawRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		assert.Equal(t, "abc", r.URL.Query().Get("id"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok-body"))
	}))
	defer srv.Close()

	f := newToolSpecTestFusion(t, srv.URL)
	tool := findToolSpec(f.ToolSpecDefinitions("tenant-xyz"), "echo_get")
	require.NotNil(t, tool)

	res, err := tool.Handler(&toolspec.ToolCall{
		Ctx:  context.Background(),
		Args: map[string]any{"id": "abc"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
	assert.Equal(t, "ok-body", res.ForLLM)
	assert.True(t, sawRequest, "upstream endpoint should have been called")
}

func TestToolSpecDefinitions_HandlerMapsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	f := newToolSpecTestFusion(t, srv.URL)
	tool := findToolSpec(f.ToolSpecDefinitions("tenant-xyz"), "echo_get")
	require.NotNil(t, tool)

	res, err := tool.Handler(&toolspec.ToolCall{
		Ctx:  context.Background(),
		Args: map[string]any{"id": "abc"},
	})
	require.Error(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
	assert.Equal(t, err, res.Err)
}

func TestToolSpecDefinitions_HandlerNilCtx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f := newToolSpecTestFusion(t, srv.URL)
	tool := findToolSpec(f.ToolSpecDefinitions("tenant-xyz"), "echo_get")
	require.NotNil(t, tool)

	// A nil call.Ctx must be tolerated (wrapper falls back to context.Background).
	res, err := tool.Handler(&toolspec.ToolCall{Args: map[string]any{"id": "abc"}})
	require.NoError(t, err)
	assert.Equal(t, "ok", res.ForLLM)
}

func TestServiceForToolName(t *testing.T) {
	names := []string{"microsoft365", "google"}
	sortByLengthDesc(names)

	tests := []struct {
		toolName string
		want     string
	}{
		{"google_calendar_events_list", "google"},
		{"microsoft365_mail_read_inbox", "microsoft365"},
		{"google_auth_setup", "google"},
		{"command_deploy", "command"},
		{"standalone", "standalone"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, serviceForToolName(tt.toolName, names), tt.toolName)
	}
}

func TestConvertHints_Nil(t *testing.T) {
	assert.Nil(t, convertHints(nil))

	h := convertHints(&global.ToolHints{ReadOnly: global.BoolPtr(true)})
	require.NotNil(t, h)
	require.NotNil(t, h.ReadOnly)
	assert.True(t, *h.ReadOnly)
}
