/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"testing"
)

func TestApplyParameterTransforms(t *testing.T) {
	tests := []struct {
		name       string
		params     []ParameterConfig
		inputArgs  map[string]interface{}
		wantArgs   map[string]interface{}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of inputArgs so we can compare mutations independently.
			argsCopy := make(map[string]interface{}, len(tt.inputArgs))
			for k, v := range tt.inputArgs {
				argsCopy[k] = v
			}

			applyParameterTransforms(tt.params, argsCopy, nil)

			for k, want := range tt.wantArgs {
				got, ok := argsCopy[k]
				if !ok {
					t.Errorf("key %q missing from args after transform", k)
					continue
				}
				if got != want {
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
