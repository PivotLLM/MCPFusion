/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bundled UnifyEM configuration must load, register every tool with the
// agreed hints, and build request bodies in the shape the UnifyEM API expects.
func TestUnifyEMConfig_LoadsAndRegistersTools(t *testing.T) {
	logger := newTestLogger(t)
	f, err := New(WithLogger(logger), WithJSONConfig("../configs/unifyem.json"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	require.NotNil(t, f.config, "configs/unifyem.json failed to load")

	service := f.config.Services["unifyem"]
	require.NotNil(t, service)
	sc, ok := service.Auth.SessionCredentials()
	require.True(t, ok, "unifyem must use session_jwt with per-user credentials")
	assert.Equal(t, CredentialStoreCredentials, sc.Store)
	require.Len(t, sc.Fields, 2)
	assert.True(t, sc.Fields[1].Secret, "password field must be secret")

	tools := map[string]bool{}
	destructive := map[string]bool{}
	readOnly := 0
	for _, tool := range f.RegisterTools() {
		if !strings.HasPrefix(tool.Name, "unifyem_") {
			continue
		}
		tools[tool.Name] = true
		if tool.Name == "unifyem_auth_setup" {
			continue
		}
		require.NotNil(t, tool.Hints, "%s has no hints", tool.Name)
		require.NotNil(t, tool.Hints.OpenWorld, "%s missing openWorld", tool.Name)
		assert.True(t, *tool.Hints.OpenWorld, "%s must be open-world", tool.Name)
		if tool.Hints.Destructive != nil && *tool.Hints.Destructive {
			destructive[tool.Name] = true
		}
		if tool.Hints.ReadOnly != nil && *tool.Hints.ReadOnly {
			readOnly++
		}
	}
	assert.Len(t, tools, 52, "51 API tools plus unifyem_auth_setup")
	assert.True(t, tools["unifyem_auth_setup"])
	assert.Equal(t, map[string]bool{
		"unifyem_agent_delete":            true,
		"unifyem_agent_trigger_lost":      true,
		"unifyem_agent_trigger_uninstall": true,
		"unifyem_agent_trigger_wipe":      true,
		"unifyem_cmd_user_delete":         true,
		"unifyem_request_delete":          true,
		"unifyem_user_delete":             true,
	}, destructive)
	assert.Equal(t, 16, readOnly)
}

func TestUnifyEMConfig_RequestShapes(t *testing.T) {
	logger := newTestLogger(t)
	f, err := New(WithLogger(logger), WithJSONConfig("../configs/unifyem.json"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	require.NotNil(t, f.config)
	service := f.config.Services["unifyem"]
	m := NewMapper(WithLogger(logger))

	build := func(t *testing.T, id string, args map[string]any) (string, string) {
		t.Helper()
		for i := range service.Endpoints {
			e := &service.Endpoints[i]
			if e.ID != id {
				continue
			}
			u, err := m.BuildURL("https://uem.example.com", e.Path, e.Parameters, args)
			require.NoError(t, err)
			body, err := m.BuildRequestBody(e.Parameters, args, e.RequestBody)
			require.NoError(t, err)
			b, _ := json.Marshal(body)
			return u, string(b)
		}
		t.Fatalf("endpoint %s not found", id)
		return "", ""
	}

	u, body := build(t, "cmd_ping", map[string]any{"args.agent_id": "A-1"})
	assert.Equal(t, "https://uem.example.com/api/v1/cmd", u)
	assert.JSONEq(t, `{"cmd":"ping","args":{"agent_id":"A-1"}}`, body)

	_, body = build(t, "cmd_execute", map[string]any{"args.agent_id": "A-1", "args.cmd": "ls", "args.arg1": "-la"})
	assert.JSONEq(t, `{"cmd":"execute","args":{"agent_id":"A-1","cmd":"ls","arg1":"-la"}}`, body)

	u, body = build(t, "agent_trigger_wipe", map[string]any{"agent_id": "A-1"})
	assert.Equal(t, "https://uem.example.com/api/v1/agent/A-1", u)
	assert.JSONEq(t, `{"triggers":{"wipe":true}}`, body)

	_, body = build(t, "agent_tags_add", map[string]any{"agent_id": "A-1", "tags": []any{"a", "b"}})
	assert.JSONEq(t, `{"tags":["a","b"]}`, body)

	_, body = build(t, "config_agents_set", map[string]any{"parameters": map[string]any{"sync_interval": "300"}})
	assert.JSONEq(t, `{"parameters":{"sync_interval":"300"}}`, body)

	_, body = build(t, "report_get", map[string]any{"report": "agents"})
	assert.JSONEq(t, `{"report":"agents","args":{"format":"json"}}`, body)

	u, _ = build(t, "agent_list_by_tag", map[string]any{"tag": "finance"})
	assert.Equal(t, "https://uem.example.com/api/v1/agent/by-tag/finance", u)
}
