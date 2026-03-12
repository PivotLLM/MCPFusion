/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package global

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
	IsList    bool   // true for list operations — Bytes is an item count, not a byte count
}

// requestRecordKeyType is the unexported context key type for *RequestRecord.
type requestRecordKeyType struct{}

// RequestRecordKey is the context key for *RequestRecord.
var RequestRecordKey = requestRecordKeyType{}

// Fields returns the record as alternating key-value pairs suitable for
// structured logging (e.g. logger.InfoFields(record.Fields()...)).
func (r *RequestRecord) Fields() []any {
	fields := []any{
		"method", r.Method,
		"path", r.Path,
		"request_id", r.RequestID,
		"ip", r.IP,
		"tenant", r.Tenant,
	}
	if r.MCPMethod == "" {
		return fields
	}
	op := r.MCPMethod
	if r.ToolName != "" {
		op += ":" + r.ToolName
	}
	status := r.Status
	if status == "" {
		status = "ok"
	}
	fields = append(fields, "op", op, "status", status)
	if r.IsList {
		fields = append(fields, "items", r.Bytes)
	} else {
		fields = append(fields, "bytes", r.Bytes)
	}
	return fields
}
