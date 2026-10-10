/******************************************************************************
 * Copyright (c) 2026 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"testing"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/stretchr/testify/assert"
	"github.com/tenebris-tech/mlogger"
)

func TestWithLogger_HubProviderUsesLogger(t *testing.T) {
	mem := mlogger.NewMemoryLogger()
	configs := map[string]*fusion.ServiceConfig{
		"bad": {ServiceKey: "bad", Transport: "carrier-pigeon"},
	}

	h := NewHubProvider(configs, WithLogger(mem))
	h.Start(context.Background())
	h.Shutdown()

	assert.NotEmpty(t, mem.Logs(), "hub provider should log through WithLogger")
}

func TestNewHubProvider_WithoutLogger(t *testing.T) {
	configs := map[string]*fusion.ServiceConfig{
		"bad": {ServiceKey: "bad", Transport: "carrier-pigeon"},
	}

	h := NewHubProvider(configs)
	assert.NotPanics(t, func() {
		h.Start(context.Background())
		h.Shutdown()
	})
}

func TestClientConstructors_WithoutLogger(t *testing.T) {
	cfg := &fusion.ServiceConfig{ServiceKey: "svc", Command: "/bin/true"}

	assert.NotNil(t, NewSSEClient(cfg))
	assert.NotNil(t, NewHTTPClient(cfg))
	assert.NotNil(t, NewStdioClient(cfg))
	assert.NotNil(t, NewMCPClientManager("svc"))
}
