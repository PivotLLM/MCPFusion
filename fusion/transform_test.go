/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"fmt"
	"testing"
)

func TestApplyParameterTransforms(t *testing.T) {
	tests := []struct {
		name      string
		params    []ParameterConfig
		inputArgs map[string]interface{}
		wantArgs  map[string]interface{}
		wantErr   bool
	}{
		{
			name: "empty string input — unchanged",
			params: []ParameterConfig{
				{Name: "content", Transforms: []string{"html_compact"}},
			},
			inputArgs: map[string]interface{}{"content": ""},
			wantArgs:  map[string]interface{}{"content": ""},
		},
		{
			name: "non-string value in args — skipped",
			params: []ParameterConfig{
				{Name: "count", Transforms: []string{"html_compact"}},
			},
			inputArgs: map[string]interface{}{"count": 42},
			wantArgs:  map[string]interface{}{"count": 42},
		},
		{
			name: "parameter absent from args — skipped",
			params: []ParameterConfig{
				{Name: "missing", Transforms: []string{"html_compact"}},
			},
			inputArgs: map[string]interface{}{},
			wantArgs:  map[string]interface{}{},
		},
		{
			name: "input with no inter-element whitespace — unchanged, no mutation",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{"html_compact"}},
			},
			inputArgs: map[string]interface{}{"html": "<p>Hello world</p>"},
			wantArgs:  map[string]interface{}{"html": "<p>Hello world</p>"},
		},
		{
			name: "input with </p>\\n<p> pattern — compacted to </p><p>",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{"html_compact"}},
			},
			inputArgs: map[string]interface{}{"html": "<p>First</p>\n<p>Second</p>"},
			wantArgs:  map[string]interface{}{"html": "<p>First</p><p>Second</p>"},
		},
		{
			name: "input with <ol>\\n<li> pattern — compacted to <ol><li>",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{"html_compact"}},
			},
			inputArgs: map[string]interface{}{"html": "<ol>\n<li>Item</li>\n</ol>"},
			wantArgs:  map[string]interface{}{"html": "<ol><li>Item</li></ol>"},
		},
		{
			name: "multiple newlines between tags — compacted",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{"html_compact"}},
			},
			inputArgs: map[string]interface{}{"html": "<p>A</p>\n\n\n<p>B</p>"},
			wantArgs:  map[string]interface{}{"html": "<p>A</p><p>B</p>"},
		},
		{
			name: "CRLF between tags — compacted",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{"html_compact"}},
			},
			inputArgs: map[string]interface{}{"html": "<p>A</p>\r\n<p>B</p>"},
			wantArgs:  map[string]interface{}{"html": "<p>A</p><p>B</p>"},
		},
		{
			name: "content inside <pre><code> blocks (newlines in text content) — preserved",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{"html_compact"}},
			},
			// The newlines here appear between text characters and tag boundaries,
			// not between two tag boundaries, so the regex does not match.
			inputArgs: map[string]interface{}{
				"html": "<pre><code>line1\nline2\nline3</code></pre>",
			},
			wantArgs: map[string]interface{}{
				"html": "<pre><code>line1\nline2\nline3</code></pre>",
			},
		},
		{
			name: "unknown transform name — value unchanged",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{"unknown_transform"}},
			},
			inputArgs: map[string]interface{}{"html": "<p>Hello</p>\n<p>World</p>"},
			wantArgs:  map[string]interface{}{"html": "<p>Hello</p>\n<p>World</p>"},
		},
		{
			name: "multiple transforms listed — applied in order (html_compact twice, idempotent)",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{"html_compact", "html_compact"}},
			},
			inputArgs: map[string]interface{}{"html": "<p>A</p>\n<p>B</p>"},
			wantArgs:  map[string]interface{}{"html": "<p>A</p><p>B</p>"},
		},
		{
			name: "transforms is empty slice — no change",
			params: []ParameterConfig{
				{Name: "html", Transforms: []string{}},
			},
			inputArgs: map[string]interface{}{"html": "<p>A</p>\n<p>B</p>"},
			wantArgs:  map[string]interface{}{"html": "<p>A</p>\n<p>B</p>"},
		},

		// --- html_compact_fields ---

		{
			name: "html_compact_fields: array of objects with HTML fields — whitespace stripped",
			params: []ParameterConfig{
				{
					Name:       "items",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"html_compact_fields:description,observation"},
				},
			},
			inputArgs: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"description": "<p>A</p>\n<p>B</p>",
						"observation": "<ol>\n<li>X</li>\n</ol>",
						"title":       "unchanged",
					},
				},
			},
			wantArgs: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"description": "<p>A</p><p>B</p>",
						"observation": "<ol><li>X</li></ol>",
						"title":       "unchanged",
					},
				},
			},
		},
		{
			name: "html_compact_fields: array of objects without whitespace — unchanged",
			params: []ParameterConfig{
				{
					Name:       "items",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"html_compact_fields:description"},
				},
			},
			inputArgs: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"description": "<p>No whitespace between tags</p>",
					},
				},
			},
			wantArgs: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"description": "<p>No whitespace between tags</p>",
					},
				},
			},
		},
		{
			name: "html_compact_fields: named field not present in object — skipped gracefully",
			params: []ParameterConfig{
				{
					Name:       "items",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"html_compact_fields:missing_field"},
				},
			},
			inputArgs: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"other": "<p>A</p>\n<p>B</p>",
					},
				},
			},
			wantArgs: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"other": "<p>A</p>\n<p>B</p>",
					},
				},
			},
		},
		{
			name: "html_compact_fields: non-array value — skipped gracefully (no error)",
			params: []ParameterConfig{
				{
					Name:       "items",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"html_compact_fields:description"},
				},
			},
			inputArgs: map[string]interface{}{
				"items": "not an array",
			},
			wantArgs: map[string]interface{}{
				"items": "not an array",
			},
		},

		// --- validate_object_fields ---

		{
			name: "validate_object_fields: valid objects with all required fields present — no error",
			params: []ParameterConfig{
				{
					Name:       "fields",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"validate_object_fields:label,customField._id"},
				},
			},
			inputArgs: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"label":       "Some label",
						"customField": map[string]interface{}{"_id": "abc123"},
					},
				},
			},
			wantArgs: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"label":       "Some label",
						"customField": map[string]interface{}{"_id": "abc123"},
					},
				},
			},
		},
		{
			name:    "validate_object_fields: object missing top-level field — error returned",
			wantErr: true,
			params: []ParameterConfig{
				{
					Name:       "fields",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"validate_object_fields:label"},
				},
			},
			inputArgs: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"other": "value",
					},
				},
			},
			wantArgs: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"other": "value",
					},
				},
			},
		},
		{
			name:    "validate_object_fields: object missing nested field — error returned",
			wantErr: true,
			params: []ParameterConfig{
				{
					Name:       "fields",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"validate_object_fields:customField._id"},
				},
			},
			inputArgs: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"customField": map[string]interface{}{"label": "no id here"},
					},
				},
			},
			wantArgs: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"customField": map[string]interface{}{"label": "no id here"},
					},
				},
			},
		},
		{
			name:    "validate_object_fields: object with empty string in required field — error returned",
			wantErr: true,
			params: []ParameterConfig{
				{
					Name:       "fields",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"validate_object_fields:label"},
				},
			},
			inputArgs: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"label": "",
					},
				},
			},
			wantArgs: map[string]interface{}{
				"fields": []interface{}{
					map[string]interface{}{
						"label": "",
					},
				},
			},
		},
		{
			name: "validate_object_fields: non-array value — skipped gracefully (no error)",
			params: []ParameterConfig{
				{
					Name:       "fields",
					Type:       ParameterTypeArray,
					Items:      "object",
					Transforms: []string{"validate_object_fields:label"},
				},
			},
			inputArgs: map[string]interface{}{
				"fields": "not an array",
			},
			wantArgs: map[string]interface{}{
				"fields": "not an array",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of inputArgs so we can compare mutations independently.
			argsCopy := make(map[string]interface{}, len(tt.inputArgs))
			for k, v := range tt.inputArgs {
				argsCopy[k] = v
			}

			err := applyParameterTransforms(tt.params, argsCopy, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, want := range tt.wantArgs {
				got, ok := argsCopy[k]
				if !ok {
					t.Errorf("key %q missing from args after transform", k)
					continue
				}
				// Deep comparison for slices of maps
				if wantSlice, ok := want.([]interface{}); ok {
					gotSlice, ok := got.([]interface{})
					if !ok {
						t.Errorf("args[%q] is not []interface{}", k)
						continue
					}
					if len(gotSlice) != len(wantSlice) {
						t.Errorf("args[%q] length = %d; want %d", k, len(gotSlice), len(wantSlice))
						continue
					}
					for i, wantElem := range wantSlice {
						gotElem := gotSlice[i]
						wantMap, wok := wantElem.(map[string]interface{})
						gotMap, gok := gotElem.(map[string]interface{})
						if wok && gok {
							for field, wantVal := range wantMap {
								gotVal := gotMap[field]
								if fmt.Sprintf("%v", gotVal) != fmt.Sprintf("%v", wantVal) {
									t.Errorf("args[%q][%d][%q] = %v; want %v", k, i, field, gotVal, wantVal)
								}
							}
						} else if gotElem != wantElem {
							t.Errorf("args[%q][%d] = %v; want %v", k, i, gotElem, wantElem)
						}
					}
				} else if got != want {
					t.Errorf("args[%q] = %q; want %q", k, got, want)
				}
			}

			// Ensure no unexpected keys were added.
			for k := range argsCopy {
				if _, ok := tt.wantArgs[k]; !ok {
					t.Errorf("unexpected key %q found in args after transform", k)
				}
			}
		})
	}
}
