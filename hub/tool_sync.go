/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolDiff represents the difference between two tool sets, tracking which
// tools were added and which were removed.
type ToolDiff struct {
	Added   []string
	Removed []string
}

// ConvertDownstreamTool converts a downstream MCP tool into a global.ToolDefinition
// suitable for registration with the MCPFusion server. The tool name is prefixed
// with the service name, and a handler closure is created that forwards calls to
// the downstream service using the original unprefixed tool name.
func ConvertDownstreamTool(
	serviceName string,
	tool mcp.Tool,
	callFunc func(ctx context.Context, toolName string, args map[string]interface{}, meta *mcp.Meta) (*mcp.CallToolResult, error),
) global.ToolDefinition {

	prefixedName := fmt.Sprintf("%s_%s", serviceName, tool.Name)

	// Convert input schema properties to global.Parameter slice
	params := convertProperties(tool.InputSchema.Properties, tool.InputSchema.Required)

	// Convert annotations to hints
	hints := convertAnnotations(tool.Annotations)

	// Capture the original tool name for the handler closure
	originalName := tool.Name

	handler := func(options map[string]any) (string, error) {
		// Extract context from options if present
		ctx, _ := options["__mcp_context"].(context.Context)
		if ctx == nil {
			ctx = context.Background()
		}

		// Extract downstream meta (set by convertToServerTool for progress forwarding)
		meta, _ := options["__meta"].(*mcp.Meta)

		// Remove internal keys before forwarding
		args := make(map[string]interface{}, len(options))
		for k, v := range options {
			if k == "__mcp_context" || k == "__meta" {
				continue
			}
			args[k] = v
		}

		result, err := callFunc(ctx, originalName, args, meta)
		if err != nil {
			return "", fmt.Errorf("downstream tool call failed: %w", err)
		}

		return FormatCallToolResult(result), nil
	}

	return global.ToolDefinition{
		Name:        prefixedName,
		Description: tool.Description,
		Parameters:  params,
		Handler:     handler,
		Hints:       hints,
	}
}

// FormatCallToolResult converts a CallToolResult into a string representation.
// For single text results, the text is returned directly. For error results, the
// output is prefixed with "Error: ". Multiple content items or non-text content
// are serialized to JSON.
func FormatCallToolResult(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}

	var texts []string
	for _, content := range result.Content {
		// Marshal and re-parse to extract the concrete type fields
		data, err := json.Marshal(content)
		if err != nil {
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		if raw["type"] == "text" {
			if text, ok := raw["text"].(string); ok {
				texts = append(texts, text)
			}
		} else {
			// For non-text content, include the JSON representation
			texts = append(texts, string(data))
		}
	}

	output := strings.Join(texts, "\n")
	if result.IsError {
		return "Error: " + output
	}
	return output
}

// DiffTools compares two tool maps and returns which tools were added and removed.
// Both maps are keyed by tool name. The returned slices are sorted for deterministic output.
func DiffTools(oldTools, newTools map[string]mcp.Tool) ToolDiff {
	var diff ToolDiff

	// Find removed tools (present in old but not in new)
	for name := range oldTools {
		if _, exists := newTools[name]; !exists {
			diff.Removed = append(diff.Removed, name)
		}
	}

	// Find added tools (present in new but not in old)
	for name := range newTools {
		if _, exists := oldTools[name]; !exists {
			diff.Added = append(diff.Added, name)
		}
	}

	// Sort for deterministic output
	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)

	return diff
}

// convertProperties converts an MCP input schema properties map to a slice of
// global.Parameter. Each property key becomes the parameter name, and type and
// description are extracted from the property value map.
func convertProperties(properties map[string]any, required []string) []global.Parameter {
	if len(properties) == 0 {
		return nil
	}

	// Build a set of required parameter names for fast lookup
	requiredSet := make(map[string]bool, len(required))
	for _, r := range required {
		requiredSet[r] = true
	}

	params := make([]global.Parameter, 0, len(properties))
	for name, propVal := range properties {
		param := global.Parameter{
			Name:     name,
			Required: requiredSet[name],
		}

		// Property values are typically map[string]any with "type", "description", etc.
		propMap, ok := propVal.(map[string]any)
		if ok {
			if t, ok := propMap["type"].(string); ok {
				param.Type = t
			}
			if d, ok := propMap["description"].(string); ok {
				param.Description = d
			}
			if def, ok := propMap["default"]; ok {
				param.Default = def
			}
			if pattern, ok := propMap["pattern"].(string); ok {
				param.Pattern = pattern
			}
			if format, ok := propMap["format"].(string); ok {
				param.Format = format
			}
			if enum, ok := propMap["enum"].([]interface{}); ok {
				param.Enum = enum
			}
			if param.Type == "array" {
				if itemsVal, ok := propMap["items"].(map[string]any); ok {
					if itemType, ok := itemsVal["type"].(string); ok {
						param.Items = itemType
					}
				}
			}
		}

		params = append(params, param)
	}

	// Sort by name for deterministic ordering
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	return params
}

// convertAnnotations maps MCP ToolAnnotation fields to global.ToolHints.
// Returns nil if all annotation hint fields are nil (unset).
func convertAnnotations(annotations mcp.ToolAnnotation) *global.ToolHints {
	if annotations.ReadOnlyHint == nil &&
		annotations.DestructiveHint == nil &&
		annotations.IdempotentHint == nil &&
		annotations.OpenWorldHint == nil {
		return nil
	}

	hints := &global.ToolHints{}

	if annotations.ReadOnlyHint != nil {
		hints.ReadOnly = global.BoolPtr(*annotations.ReadOnlyHint)
	}
	if annotations.DestructiveHint != nil {
		hints.Destructive = global.BoolPtr(*annotations.DestructiveHint)
	}
	if annotations.IdempotentHint != nil {
		hints.Idempotent = global.BoolPtr(*annotations.IdempotentHint)
	}
	if annotations.OpenWorldHint != nil {
		hints.OpenWorld = global.BoolPtr(*annotations.OpenWorldHint)
	}

	return hints
}
