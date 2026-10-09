/******************************************************************************
 * Copyright (c) 2026 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import "github.com/PivotLLM/MCPFusion/global"

// ComponentOption configures a fusion component created by one of the package's
// other constructors, such as NewValidator, NewRetryExecutor or the auth
// strategy constructors.
type ComponentOption interface {
	applyToComponent(*componentOptions)
}

// componentOptions holds the settings shared by the component constructors.
type componentOptions struct {
	logger global.Logger
}

// newComponentOptions applies opts and returns the resulting settings.
func newComponentOptions(opts []ComponentOption) componentOptions {
	var o componentOptions
	for _, opt := range opts {
		opt.applyToComponent(&o)
	}
	return o
}

// LoggerOption sets the logger. It is accepted both by New and by every
// component constructor in this package.
type LoggerOption struct {
	logger global.Logger
}

func (o LoggerOption) applyToFusion(f *Fusion) { f.logger = o.logger }

func (o LoggerOption) applyToComponent(c *componentOptions) { c.logger = o.logger }

// WithLogger sets the logger. Without it, or with a nil logger, nothing is
// logged.
func WithLogger(logger global.Logger) LoggerOption {
	return LoggerOption{logger: logger}
}
