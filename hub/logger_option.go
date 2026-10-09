/******************************************************************************
 * Copyright (c) 2026 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import "github.com/PivotLLM/MCPFusion/global"

// ClientOption configures a client created by NewSSEClient, NewHTTPClient,
// NewStdioClient or NewMCPClientManager.
type ClientOption interface {
	applyToClient(*clientOptions)
}

// clientOptions holds the settings shared by the client constructors.
type clientOptions struct {
	logger global.Logger
}

// newClientOptions applies opts and returns the resulting settings.
func newClientOptions(opts []ClientOption) clientOptions {
	var o clientOptions
	for _, opt := range opts {
		opt.applyToClient(&o)
	}
	return o
}

// LoggerOption sets the logger. It is accepted both by NewHubProvider and by
// the client constructors.
type LoggerOption struct {
	logger global.Logger
}

func (o LoggerOption) applyToHub(h *HubProvider) { h.logger = o.logger }

func (o LoggerOption) applyToClient(c *clientOptions) { c.logger = o.logger }

// WithLogger sets the logger. Without it, or with a nil logger, nothing is
// logged.
func WithLogger(logger global.Logger) LoggerOption {
	return LoggerOption{logger: logger}
}
