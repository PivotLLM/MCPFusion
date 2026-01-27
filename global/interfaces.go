/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package global

import "fmt"

// Parameter represents a parameter for a tool, resource, or prompt with rich metadata
type Parameter struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Required    bool                   `json:"required"`
	Type        string                 `json:"type"`      // "string", "number", "boolean", "array", "object"
	Default     interface{}            `json:"default"`   // Default value
	Enum        []interface{}          `json:"enum"`      // Valid values
	Pattern     string                 `json:"pattern"`   // Validation pattern
	Minimum     *float64               `json:"minimum"`   // Min value (numbers)
	Maximum     *float64               `json:"maximum"`   // Max value (numbers)
	MinLength   *int                   `json:"minLength"` // Min length (strings/arrays)
	MaxLength   *int                   `json:"maxLength"` // Max length (strings/arrays)
	Format      string                 `json:"format"`    // "date", "email", "uri", etc.
	Examples    []interface{}          `json:"examples"`  // Example values
	Metadata    map[string]interface{} `json:"metadata"`  // Extensible for future needs
}

// EnhancedDescription generates a rich description with constraint information
func (p Parameter) EnhancedDescription() string {
	desc := p.Description

	// Add default value
	if p.Default != nil {
		desc += fmt.Sprintf(" (default: %v)", p.Default)
	}

	// Add valid values (limit to reasonable length)
	if len(p.Enum) > 0 && len(p.Enum) <= 10 {
		desc += fmt.Sprintf(" (valid: %v)", p.Enum)
	} else if len(p.Enum) > 10 {
		desc += fmt.Sprintf(" (valid values: %d options available)", len(p.Enum))
	}

	// Add range constraints
	if p.Minimum != nil && p.Maximum != nil {
		desc += fmt.Sprintf(" (range: %v-%v)", *p.Minimum, *p.Maximum)
	} else if p.Minimum != nil {
		desc += fmt.Sprintf(" (min: %v)", *p.Minimum)
	} else if p.Maximum != nil {
		desc += fmt.Sprintf(" (max: %v)", *p.Maximum)
	}

	// Add length constraints
	if p.MinLength != nil && p.MaxLength != nil {
		desc += fmt.Sprintf(" (length: %d-%d)", *p.MinLength, *p.MaxLength)
	} else if p.MaxLength != nil {
		desc += fmt.Sprintf(" (max length: %d)", *p.MaxLength)
	}

	// Add format hints
	if p.Format != "" {
		desc += fmt.Sprintf(" (format: %s)", p.Format)
	}

	// Add pattern hints
	if p.Pattern != "" {
		desc += fmt.Sprintf(" (pattern: %s)", p.Pattern)
	}

	return desc
}

//
// Tools
//

// ToolDefinition represents the structure of a tool
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  []Parameter
	Handler     ToolHandler
}

// ToolHandler defines the function signature for our tool handler
type ToolHandler func(options map[string]any) (string, error)

// ToolProvider defines an interface for providing tools
type ToolProvider interface {
	RegisterTools() []ToolDefinition
}

// NewTools is a helper function that returns an empty slice of ToolDefinition
//
//goland:noinspection GoUnusedExportedFunction
func NewTools() []ToolDefinition {
	return []ToolDefinition{}
}

//
// Resources
//

// ResourceDefinition represents the structure of a resource
type ResourceDefinition struct {
	Name        string
	Description string
	MIMEType    string
	URI         string
	Handler     ResourceHandler
}

// ResourceTemplateDefinition represents the structure of a resource template
type ResourceTemplateDefinition struct {
	Name        string
	Description string
	MIMEType    string
	URITemplate string
	Handler     ResourceHandler
}

// ResourceResponse represents the structure of a resource response
type ResourceResponse struct {
	URI      string
	MIMEType string
	Content  string
}

// ResourceHandler defines the function signature for our resource handler
type ResourceHandler func(uri string, options map[string]any) (ResourceResponse, error)

// ResourceProvider defines an interface for providing resources
type ResourceProvider interface {
	RegisterResources() []ResourceDefinition
	RegisterResourceTemplates() []ResourceTemplateDefinition
}

// NewResources is a helper function that returns an empty slice of ResourceDefinition
//
//goland:noinspection GoUnusedExportedFunction
func NewResources() []ResourceDefinition {
	return []ResourceDefinition{}
}

// NewResourceTemplates is a helper function that returns an empty slice of ResourceTemplateDefinition
//
//goland:noinspection GoUnusedExportedFunction
func NewResourceTemplates() []ResourceTemplateDefinition {
	return []ResourceTemplateDefinition{}
}

//
// Prompts
//

// PromptDefinition represents the structure of a prompt
type PromptDefinition struct {
	Name        string
	Description string
	Parameters  []Parameter
	Handler     PromptHandler
}

// Messages represents a collection of messages
type Messages []Message
type Message struct {
	Role    string
	Content string
}

// PromptHandler defines the function signature for our prompt handler
type PromptHandler func(options map[string]any) (string, Messages, error)

// PromptProvider defines an interface for providing prompts
type PromptProvider interface {
	RegisterPrompts() []PromptDefinition
}

// NewPrompts is a helper function that returns an empty slice of PromptDefinition
//
//goland:noinspection GoUnusedExportedFunction
func NewPrompts() []PromptDefinition {
	return []PromptDefinition{}
}

//
// Context Keys for shared usage across packages
//

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

const (
	// TenantContextKey is the key used to store tenant context in request contexts
	TenantContextKey ContextKey = "tenant_context"
	// ServiceNameKey is the key used to store service name in request contexts
	ServiceNameKey ContextKey = "service_name"
	// ToolNameKey is the key used to store the MCP tool name in request contexts
	ToolNameKey ContextKey = "tool_name"
)
