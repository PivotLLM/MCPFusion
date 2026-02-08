/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package global

import "strings"

// ToolHints contains MCP tool hint annotations that provide metadata about tool behavior.
// All fields are optional pointers to support three states: true, false, and unset (nil).
// These hints help AI clients understand tool characteristics for better decision-making.
type ToolHints struct {
	// ReadOnly indicates the tool does not modify any state
	ReadOnly *bool

	// Destructive indicates the tool may perform destructive operations
	// (e.g., deleting data, overwriting files)
	Destructive *bool

	// Idempotent indicates calling the tool multiple times with the same
	// arguments produces the same result
	Idempotent *bool

	// OpenWorld indicates the tool interacts with external systems
	// (e.g., APIs, databases, network resources)
	OpenWorld *bool
}

// BoolPtr returns a pointer to the provided boolean value.
// This helper function simplifies creating *bool values for ToolHints fields.
func BoolPtr(b bool) *bool {
	return &b
}

// ComputeDefaultHints returns computed default hints based on the HTTP method.
// These defaults provide reasonable starting values that can be overridden by
// explicit configuration.
//
// Default mappings:
//   - GET/HEAD: ReadOnly=true, Destructive=false, Idempotent=true, OpenWorld=true
//   - PUT: ReadOnly=false, Destructive=false, Idempotent=true, OpenWorld=true
//   - DELETE: ReadOnly=false, Destructive=true, Idempotent=false, OpenWorld=true
//   - POST/PATCH (default): ReadOnly=false, Destructive=false, Idempotent=false, OpenWorld=true
func ComputeDefaultHints(httpMethod string) ToolHints {
	method := strings.ToUpper(httpMethod)

	switch method {
	case "GET", "HEAD":
		return ToolHints{
			ReadOnly:    BoolPtr(true),
			Destructive: BoolPtr(false),
			Idempotent:  BoolPtr(true),
			OpenWorld:   BoolPtr(true),
		}

	case "PUT":
		return ToolHints{
			ReadOnly:    BoolPtr(false),
			Destructive: BoolPtr(false),
			Idempotent:  BoolPtr(true),
			OpenWorld:   BoolPtr(true),
		}

	case "DELETE":
		return ToolHints{
			ReadOnly:    BoolPtr(false),
			Destructive: BoolPtr(true),
			Idempotent:  BoolPtr(false),
			OpenWorld:   BoolPtr(true),
		}

	default: // POST, PATCH, and others
		return ToolHints{
			ReadOnly:    BoolPtr(false),
			Destructive: BoolPtr(false),
			Idempotent:  BoolPtr(false),
			OpenWorld:   BoolPtr(true),
		}
	}
}
