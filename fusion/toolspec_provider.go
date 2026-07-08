/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/toolspec"
)

// ToolSpecDefinitions converts every tool RegisterTools produces (endpoint tools,
// <service>_auth_setup tools, and command_* tools) into transport-neutral
// toolspec.ToolDefinition values for an embedded host (e.g. ClawEh).
//
// tenant is the caller's tenant hash: the underlying fusion handlers require a
// *TenantContext in the call context even for auth:none endpoints (handler.go
// rejects a missing tenant), so each generated handler injects one keyed to this
// tenant. Namespacing is the aggregator's job (toolspec names are BARE), so tool
// names are passed through unchanged — they already carry the service prefix
// fusion assigned (e.g. "google_calendar_events_list").
//
// Security note: command_* tools execute local processes on the host; an
// embedded host must gate exposure of these accordingly.
func (f *Fusion) ToolSpecDefinitions(tenant string) []toolspec.ToolDefinition {
	tools := f.RegisterTools()
	out := make([]toolspec.ToolDefinition, 0, len(tools))
	serviceNames := f.serviceNamesLongestFirst()

	for i := range tools {
		td := tools[i]
		service := serviceForToolName(td.Name, serviceNames)
		revealTogether := false
		if svc := f.GetService(service); svc != nil {
			revealTogether = svc.RevealTogether
		}
		params := convertParameters(td.Parameters)
		out = append(out, toolspec.ToolDefinition{
			Name:        td.Name,
			Description: td.Description,
			Parameters:  params,
			// Pre-derive the JSON Schema so RawSchema is populated verbatim, matching
			// the "use the already-built schema" contract embedded hosts expect.
			RawSchema: toolspec.ParametersToSchema(params),
			Hints:     convertHints(td.Hints),
			Handler:   f.wrapToolHandler(td.Handler, tenant, service),
			// Group + RevealTogether let a discovery-aware host unlock all of a
			// service's tools in one search. Group is the resolved service name;
			// RevealTogether comes from the service's reveal_together config.
			Group:          service,
			RevealTogether: revealTogether,
		})
	}
	return out
}

// convertParameters maps fusion/global parameters onto toolspec parameters,
// preserving type, constraints, and examples so the derived JSON Schema matches
// what the parameter declared.
func convertParameters(params []global.Parameter) []toolspec.Parameter {
	if len(params) == 0 {
		return nil
	}
	out := make([]toolspec.Parameter, 0, len(params))
	for _, p := range params {
		out = append(out, toolspec.Parameter{
			Name:        p.Name,
			Description: p.Description,
			Required:    p.Required,
			Type:        p.Type,
			Items:       p.Items,
			Default:     p.Default,
			Enum:        p.Enum,
			Pattern:     p.Pattern,
			Minimum:     p.Minimum,
			Maximum:     p.Maximum,
			MinLength:   p.MinLength,
			MaxLength:   p.MaxLength,
			Format:      p.Format,
			Examples:    p.Examples,
		})
	}
	return out
}

// wrapToolHandler adapts a global.ToolHandler to a toolspec.ToolHandler. It
// injects a *TenantContext (fusion requires one even for auth:none) and threads
// the call context to fusion's contextAwareHandler via the "__mcp_context" key it
// expects, then maps the (string, error) return onto a *toolspec.Result.
func (f *Fusion) wrapToolHandler(handler global.ToolHandler, tenant, service string) toolspec.ToolHandler {
	return func(call *toolspec.ToolCall) (*toolspec.Result, error) {
		ctx := call.Ctx
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, global.TenantContextKey, &TenantContext{
			TenantHash:  tenant,
			ServiceName: service,
			CreatedAt:   time.Now(),
		})

		// Copy args and thread the context through the key the contextAwareHandler
		// (and auth_setup handler) look for, so a per-call ctx is honored without
		// changing fusion's legacy map-only handler signature.
		options := make(map[string]any, len(call.Args)+1)
		for k, v := range call.Args {
			options[k] = v
		}
		options["__mcp_context"] = ctx

		out, err := handler(options)
		result := &toolspec.Result{ForLLM: out}
		if err != nil {
			result.IsError = true
			result.Err = err
		}
		return result, err
	}
}

// serviceNamesLongestFirst returns configured service names ordered longest
// first, so serviceForToolName matches the most specific prefix.
func (f *Fusion) serviceNamesLongestFirst() []string {
	names := f.GetServiceNames()
	sortByLengthDesc(names)
	return names
}

// serviceForToolName resolves the owning service name for a tool by longest
// matching "<service>_" prefix. command_* tools and any unmatched name fall back
// to the first path segment before "_", which is a harmless label: endpoint
// handlers overwrite ServiceName from their own config, and command handlers do
// not consult it.
func serviceForToolName(toolName string, serviceNamesLongestFirst []string) string {
	for _, name := range serviceNamesLongestFirst {
		if strings.HasPrefix(toolName, name+"_") {
			return name
		}
	}
	if idx := strings.IndexByte(toolName, '_'); idx > 0 {
		return toolName[:idx]
	}
	return toolName
}

// convertHints maps fusion/global tool hints onto toolspec tool hints.
func convertHints(h *global.ToolHints) *toolspec.ToolHints {
	if h == nil {
		return nil
	}
	return &toolspec.ToolHints{
		ReadOnly:    h.ReadOnly,
		Destructive: h.Destructive,
		Idempotent:  h.Idempotent,
		OpenWorld:   h.OpenWorld,
	}
}

// sortByLengthDesc sorts strings by descending length (stable enough for prefix
// matching; ties keep input order).
func sortByLengthDesc(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && len(s[j]) > len(s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
