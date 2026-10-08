/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package env

import (
	"reflect"
	"testing"
)

func TestConfigFiles(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		configs string
		config  string
		want    []string
	}{
		{"none", "", "", "", []string{}},
		{"flag wins", "a.json, b.json", "c.json", "d.json", []string{"a.json", "b.json"}},
		{"configs variable", "", "c.json,,  e.json ", "d.json", []string{"c.json", "e.json"}},
		{"config variable", "", "", "d.json", []string{"d.json"}},
		{"only separators", " , ,", "", "", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MCP_FUSION_CONFIGS", tt.configs)
			t.Setenv("MCP_FUSION_CONFIG", tt.config)
			if got := ConfigFiles(tt.flag, nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConfigFiles(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestKnowledgeEnabled(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"", true},
		{"true", true},
		{"false", false},
		{" FALSE ", false},
		{"0", false},
		{"no", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("MCP_FUSION_KNOWLEDGE", tt.value)
			if got := KnowledgeEnabled(); got != tt.want {
				t.Errorf("KnowledgeEnabled() with %q = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestPerfEnabled(t *testing.T) {
	tests := []struct {
		name  string
		flag  bool
		value string
		want  bool
	}{
		{"off", false, "", false},
		{"flag", true, "", true},
		{"variable true", false, "true", true},
		{"variable yes upper", false, " YES ", true},
		{"variable 1", false, "1", true},
		{"variable false", false, "false", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MCP_FUSION_PERF", tt.value)
			if got := PerfEnabled(tt.flag); got != tt.want {
				t.Errorf("PerfEnabled(%v) with %q = %v, want %v", tt.flag, tt.value, got, tt.want)
			}
		})
	}
}

func TestListen(t *testing.T) {
	tests := []struct {
		name        string
		envListen   string
		port        int
		want        string
		wantFromEnv bool
	}{
		{"variable wins", "0.0.0.0:9000", 8081, "0.0.0.0:9000", true},
		{"port", "", 8081, "localhost:8081", false},
		{"port zero", "", 0, defaultListen, false},
		{"port too large", "", 65536, defaultListen, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MCP_FUSION_LISTEN", tt.envListen)
			got, fromEnv := Listen(tt.port)
			if got != tt.want || fromEnv != tt.wantFromEnv {
				t.Errorf("Listen(%d) = %q, %v, want %q, %v", tt.port, got, fromEnv, tt.want, tt.wantFromEnv)
			}
		})
	}
}

func TestLogFile(t *testing.T) {
	t.Setenv("MCP_FUSION_LOGFILE", "/tmp/x.log")
	if got := LogFile(); got != "/tmp/x.log" {
		t.Errorf("LogFile() = %q, want /tmp/x.log", got)
	}
	t.Setenv("MCP_FUSION_LOGFILE", "")
	if got := LogFile(); got != "" {
		t.Errorf("LogFile() with empty variable = %q, want empty", got)
	}
}
