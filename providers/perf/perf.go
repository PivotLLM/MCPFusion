/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package perf provides performance and stress-testing MCP tools.
// WARNING: These tools are intended for development and testing only.
// Never enable this provider in production.
package perf

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// Provider implements global.ToolProvider for the perf tools.
type Provider struct {
	logger  global.Logger
	counter atomic.Int64
}

// Option is a functional option for configuring a Provider.
type Option func(*Provider)

// WithLogger sets the logger.
func WithLogger(l global.Logger) Option {
	return func(p *Provider) { p.logger = l }
}

// New creates a new perf Provider with the given options.
func New(opts ...Option) *Provider {
	p := &Provider{}
	for _, o := range opts {
		o(p)
	}
	return p
}

// RegisterTools implements global.ToolProvider and returns all perf tools.
func (p *Provider) RegisterTools() []global.ToolDefinition {
	tools := []global.ToolDefinition{
		p.echoTool(),
		p.delayTool(),
		p.randomDataTool(),
		p.errorTool(),
		p.counterTool(),
	}

	if p.logger != nil {
		p.logger.Infof("Perf provider registered %d tools", len(tools))
	}

	return tools
}

// extractContext pulls the MCP context from the args map.  If no context is
// present, it returns context.Background().
func extractContext(args map[string]interface{}) context.Context {
	if v, ok := args["__mcp_context"]; ok {
		if ctx, ok := v.(context.Context); ok {
			return ctx
		}
	}
	return context.Background()
}

// echoTool returns the perf_echo tool definition.
func (p *Provider) echoTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name:        "perf_echo",
		Description: "Echoes the provided message back as JSON. Useful for round-trip latency testing.",
		Parameters: []global.Parameter{
			{
				Name:        "message",
				Description: "The message to echo",
				Required:    true,
				Type:        "string",
			},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			_ = extractContext(args)
			message, _ := args["message"].(string)

			if p.logger != nil {
				p.logger.Debugf("perf_echo: message length %d", len(message))
			}

			out, err := json.Marshal(map[string]string{"message": message})
			if err != nil {
				return "", fmt.Errorf("perf_echo: failed to marshal response: %w", err)
			}
			return string(out), nil
		},
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(true),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(true),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// delayTool returns the perf_delay tool definition.
func (p *Provider) delayTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name:        "perf_delay",
		Description: "Sleeps for the given number of seconds (capped at 60) before returning. Useful for testing timeout behaviour.",
		Parameters: []global.Parameter{
			{
				Name:        "seconds",
				Description: "Number of seconds to sleep (capped at 60)",
				Required:    true,
				Type:        "number",
			},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			ctx := extractContext(args)

			rawSeconds, _ := args["seconds"].(float64)
			if rawSeconds > 60 {
				rawSeconds = 60
			}
			if rawSeconds < 0 {
				rawSeconds = 0
			}

			if p.logger != nil {
				p.logger.Debugf("perf_delay: sleeping for %.1f seconds", rawSeconds)
			}

			duration := time.Duration(rawSeconds * float64(time.Second))
			select {
			case <-time.After(duration):
			case <-ctx.Done():
				return "", fmt.Errorf("perf_delay: context cancelled: %w", ctx.Err())
			}

			if p.logger != nil {
				p.logger.Debugf("perf_delay: finished sleeping %.1f seconds", rawSeconds)
			}

			out, err := json.Marshal(map[string]float64{"slept_seconds": rawSeconds})
			if err != nil {
				return "", fmt.Errorf("perf_delay: failed to marshal response: %w", err)
			}
			return string(out), nil
		},
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(true),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(true),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// randomDataTool returns the perf_random_data tool definition.
func (p *Provider) randomDataTool() global.ToolDefinition {
	const maxBytes = 1_048_576 // 1 MiB
	return global.ToolDefinition{
		Name:        "perf_random_data",
		Description: "Generates n random bytes (capped at 1 MiB) and returns them hex-encoded. Useful for testing large-payload handling.",
		Parameters: []global.Parameter{
			{
				Name:        "bytes",
				Description: "Number of bytes to generate (capped at 1,048,576)",
				Required:    true,
				Type:        "integer",
			},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			_ = extractContext(args)

			var n int
			switch v := args["bytes"].(type) {
			case float64:
				n = int(v)
			case int:
				n = v
			case int64:
				n = int(v)
			}

			if n > maxBytes {
				n = maxBytes
			}
			if n < 0 {
				n = 0
			}

			buf := make([]byte, n)
			// math/rand is acceptable here — no security requirement.
			//nolint:gosec
			_, _ = rand.New(rand.NewSource(time.Now().UnixNano())).Read(buf)

			out, err := json.Marshal(map[string]interface{}{
				"bytes": n,
				"data":  hex.EncodeToString(buf),
			})
			if err != nil {
				return "", fmt.Errorf("perf_random_data: failed to marshal response: %w", err)
			}
			return string(out), nil
		},
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(true),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(false),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// errorTool returns the perf_error tool definition.
func (p *Provider) errorTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name:        "perf_error",
		Description: "Returns an error with the provided message. Useful for testing error-handling paths.",
		Parameters: []global.Parameter{
			{
				Name:        "message",
				Description: "Error message to return (default: \"perf provider error\")",
				Required:    false,
				Type:        "string",
			},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			_ = extractContext(args)

			message, _ := args["message"].(string)
			if message == "" {
				message = "perf provider error"
			}
			return "", fmt.Errorf("%s", message) //nolint:goerr113
		},
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(true),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(true),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// counterTool returns the perf_counter tool definition.
func (p *Provider) counterTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name:        "perf_counter",
		Description: "Atomically increments and returns a monotonically increasing counter. Useful for testing concurrency.",
		Parameters:  []global.Parameter{},
		Handler: func(args map[string]interface{}) (string, error) {
			_ = extractContext(args)

			n := p.counter.Add(1)
			out, err := json.Marshal(map[string]int64{"count": n})
			if err != nil {
				return "", fmt.Errorf("perf_counter: failed to marshal response: %w", err)
			}
			return string(out), nil
		},
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(false),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(false),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}
