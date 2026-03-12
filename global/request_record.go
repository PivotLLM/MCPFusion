/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package global

import "fmt"

// RequestRecord is a mutable struct stored as a pointer in the request context.
// SimpleMiddleware creates and populates it; MCP hooks fill in the MCP-layer fields.
// SimpleMiddleware logs the combined line after the handler returns.
type RequestRecord struct {
	IP        string
	Method    string
	Path      string
	Tenant    string
	RequestID string
	MCPMethod string // "tools/call", "tools/list", etc. — empty if no MCP hook fired
	ToolName  string // only for tools/call
	Status    string // "ok" or "error"
	Bytes     int    // response bytes (tools/call) or item count (list operations)
	IsList    bool   // true for list operations — format Bytes as "N items"
}

// requestRecordKeyType is the unexported context key type for *RequestRecord.
type requestRecordKeyType struct{}

// RequestRecordKey is the context key for *RequestRecord.
var RequestRecordKey = requestRecordKeyType{}

// Format returns a single-line log string combining HTTP and MCP fields.
func (r *RequestRecord) Format() string {
	base := fmt.Sprintf("%s %s [%s] ip=%s tenant=%s",
		r.Method, r.Path, r.RequestID, r.IP, r.Tenant)
	if r.MCPMethod == "" {
		return base
	}
	op := r.MCPMethod
	if r.ToolName != "" {
		op += ":" + r.ToolName
	}
	status := r.Status
	if status == "" {
		status = "ok"
	}
	unit := "bytes"
	if r.IsList {
		unit = "items"
	}
	return fmt.Sprintf("%s %s %s %d %s", base, op, status, r.Bytes, unit)
}
