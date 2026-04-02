/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package global

import "time"

// HTTP transport connection pool limits.
// These control the shared http.Transport used by all outbound API calls.
const (
	HTTPMaxIdleConns        = 100
	HTTPMaxIdleConnsPerHost = 10
	HTTPMaxConnsPerHost     = 50
	HTTPIdleConnTimeout     = 30 * time.Second
)

// HTTP client and outbound request timeouts.
const (
	HTTPDefaultClientTimeout      = 60 * time.Second
	HTTPDefaultOutboundTimeout    = 60 * time.Second
	HTTPDialTimeout               = 10 * time.Second
	HTTPKeepAliveInterval         = 30 * time.Second
	HTTPTLSHandshakeTimeout       = 10 * time.Second
	HTTPResponseHeaderTimeout     = 30 * time.Second
	HTTPExpectContinueTimeout     = 1 * time.Second
)

// Response size limits.
//
// MaxResponseBodyReadBytes caps the number of bytes read from an upstream
// HTTP response body via io.LimitReader before any transformation is applied.
// Setting this generously allows transforms to reduce large payloads before
// the final output size limit is enforced.
//
// DefaultMaxResponseBytes is the default limit on the final (post-transform)
// response string returned to MCP clients.  Operators can override this with
// WithMaxResponseBytes().
const (
	MaxResponseBodyReadBytes = 50 * 1024 * 1024 // 50 MB — upstream body read cap
	DefaultMaxResponseBytes  = 1 * 1024 * 1024  // 1 MB  — final output cap
)

// Knowledge store limits.
const (
	MaxKnowledgeQueryLength = 512
)
