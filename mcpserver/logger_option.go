/******************************************************************************
 * Copyright (c) 2026 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import "github.com/PivotLLM/MCPFusion/global"

// TransportOption configures a transport created by NewAuthenticatedTransport
// or NewExtendedTransport.
type TransportOption interface {
	applyToTransport(*transportOptions)
}

// transportOptions holds the settings shared by the transport constructors.
type transportOptions struct {
	logger global.Logger
}

// newTransportOptions applies opts and returns the resulting settings.
func newTransportOptions(opts []TransportOption) transportOptions {
	var o transportOptions
	for _, opt := range opts {
		opt.applyToTransport(&o)
	}
	return o
}

// LoggerOption sets the logger. It is accepted both by New and by the
// transport constructors.
type LoggerOption struct {
	logger global.Logger
}

func (o LoggerOption) applyToServer(m *MCPServer) { m.logger = o.logger }

func (o LoggerOption) applyToTransport(t *transportOptions) { t.logger = o.logger }

// WithLogger sets the logger. New requires one; the transport constructors log
// nothing without it.
func WithLogger(logger global.Logger) LoggerOption {
	return LoggerOption{logger: logger}
}
