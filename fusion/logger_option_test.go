/******************************************************************************
 * Copyright (c) 2026 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"strings"
	"testing"
	"time"

	"github.com/tenebris-tech/mlogger"
)

func TestWithLogger_ComponentUsesLogger(t *testing.T) {
	mem := mlogger.NewMemoryLogger()
	NewDatabaseCacheWithDefaultTTL(nil, time.Hour, WithLogger(mem))

	for _, line := range mem.Logs() {
		if strings.Contains(line, "Initialized database cache") {
			return
		}
	}
	t.Fatalf("component did not log through WithLogger; got %v", mem.Logs())
}

func TestWithLogger_FusionUsesLogger(t *testing.T) {
	mem := mlogger.NewMemoryLogger()
	f, err := New(WithLogger(mem))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if f.Logger() != mem {
		t.Fatalf("Logger() = %v, want the WithLogger logger", f.Logger())
	}
}

func TestComponentConstructors_WithoutLogger(t *testing.T) {
	tests := []struct {
		name string
		run  func()
	}{
		{"validator", func() {
			params := []ParameterConfig{{Name: "q", Type: ParameterTypeString, Required: true}}
			_ = NewValidator().ValidateParameters(params, map[string]interface{}{})
		}},
		{"database cache", func() {
			c := NewDatabaseCache(nil)
			_ = c.Set("k", "v", time.Minute)
			_, _ = c.Get("k")
		}},
		{"time tokens", func() { _ = NewTimeTokenProcessor().ProcessValue("#DAYS-1") }},
		{"mapper", func() { _, _ = NewMapper().BuildURL("https://example.com", "/x", nil, nil) }},
		{"auth manager", func() { _ = NewMultiTenantAuthManager(nil, NewDatabaseCache(nil)).RegisteredStrategies() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run()
		})
	}
}
