/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"fmt"
	"regexp"
	"strings"
	
	"github.com/PivotLLM/MCPFusion/global"
)

// mcpParameterPattern defines the valid pattern for MCP parameter names
var mcpParameterPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`)

// invalidCharPattern matches characters that are not valid in MCP parameter names
var invalidCharPattern = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// SanitizeParameterName removes invalid characters from a parameter name to make it MCP-compliant
func SanitizeParameterName(name string) string {
	// Remove invalid characters
	sanitized := invalidCharPattern.ReplaceAllString(name, "")
	
	// Ensure it's not empty after sanitization
	if sanitized == "" {
		// If the original name was all invalid characters, use a placeholder
		sanitized = "param"
	}
	
	// Ensure it doesn't exceed 64 characters
	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}
	
	return sanitized
}

// GetMCPParameterName returns the MCP-compliant name for a parameter
// It uses the alias if provided, otherwise sanitizes the original name
func GetMCPParameterName(param *ParameterConfig) string {
	if param.Alias != "" {
		return param.Alias
	}
	return SanitizeParameterName(param.Name)
}

// IsValidMCPParameterName checks if a parameter name is MCP-compliant
func IsValidMCPParameterName(name string) bool {
	return mcpParameterPattern.MatchString(name)
}

// ParameterNameMapper manages the bidirectional mapping between MCP names and original API names
type ParameterNameMapper struct {
	mcpToOriginal map[string]string
	originalToMCP map[string]string
}

// NewParameterNameMapper creates a new parameter name mapper
func NewParameterNameMapper() *ParameterNameMapper {
	return &ParameterNameMapper{
		mcpToOriginal: make(map[string]string),
		originalToMCP: make(map[string]string),
	}
}

// AddMapping adds a bidirectional mapping between MCP and original parameter names
func (m *ParameterNameMapper) AddMapping(mcpName, originalName string) error {
	// Check for conflicts
	if existing, exists := m.mcpToOriginal[mcpName]; exists && existing != originalName {
		return fmt.Errorf("MCP parameter name '%s' already mapped to '%s', cannot map to '%s'", 
			mcpName, existing, originalName)
	}
	
	m.mcpToOriginal[mcpName] = originalName
	m.originalToMCP[originalName] = mcpName
	return nil
}

// GetOriginalName returns the original API parameter name for an MCP name
func (m *ParameterNameMapper) GetOriginalName(mcpName string) string {
	if original, exists := m.mcpToOriginal[mcpName]; exists {
		return original
	}
	// If no mapping exists, return the MCP name as-is
	return mcpName
}

// GetMCPName returns the MCP-compliant name for an original parameter name
func (m *ParameterNameMapper) GetMCPName(originalName string) string {
	if mcpName, exists := m.originalToMCP[originalName]; exists {
		return mcpName
	}
	// If no mapping exists, sanitize the original name
	return SanitizeParameterName(originalName)
}

// MapArgsToOriginal converts MCP parameter names in args to original API names
func (m *ParameterNameMapper) MapArgsToOriginal(args map[string]interface{}) map[string]interface{} {
	mapped := make(map[string]interface{})
	for mcpName, value := range args {
		originalName := m.GetOriginalName(mcpName)
		mapped[originalName] = value
	}
	return mapped
}

// BuildParameterMappings creates a parameter name mapper for an endpoint's parameters
func BuildParameterMappings(params []ParameterConfig, logger global.Logger) (*ParameterNameMapper, error) {
	mapper := NewParameterNameMapper()
	
	for _, param := range params {
		mcpName := GetMCPParameterName(&param)
		originalName := param.Name
		
		// Log the mapping
		if logger != nil {
			if param.Alias != "" {
				logger.Infof("Using parameter alias '%s' for '%s'", mcpName, originalName)
			} else if mcpName != originalName {
				logger.Warningf("Auto-sanitized parameter '%s' to '%s' - consider adding explicit alias", 
					originalName, mcpName)
			}
		}
		
		// Add the mapping
		if err := mapper.AddMapping(mcpName, originalName); err != nil {
			return nil, err
		}
	}
	
	return mapper, nil
}

// ValidateParameterNames checks for naming conflicts in a set of parameters
func ValidateParameterNames(params []ParameterConfig) error {
	seen := make(map[string]string) // MCP name -> original name
	
	for _, param := range params {
		mcpName := GetMCPParameterName(&param)
		
		// Check if MCP name is valid
		if !IsValidMCPParameterName(mcpName) {
			return fmt.Errorf("parameter '%s' (alias/sanitized: '%s') is not MCP-compliant", 
				param.Name, mcpName)
		}
		
		// Check for conflicts
		if originalName, exists := seen[mcpName]; exists {
			if originalName != param.Name {
				return fmt.Errorf("parameter name conflict: both '%s' and '%s' map to MCP name '%s'",
					originalName, param.Name, mcpName)
			}
		}
		
		seen[mcpName] = param.Name
	}
	
	return nil
}

// SuggestAlias suggests an alias for a parameter with invalid characters
func SuggestAlias(paramName string) string {
	// Common Microsoft Graph API parameter mappings
	suggestions := map[string]string{
		"$select":  "select",
		"$filter":  "filter",
		"$orderby": "orderby",
		"$top":     "top",
		"$skip":    "skip",
		"$expand":  "expand",
		"$search":  "search",
		"$count":   "count",
		"$format":  "format",
	}
	
	if suggestion, exists := suggestions[strings.ToLower(paramName)]; exists {
		return suggestion
	}
	
	// For other parameters, just sanitize
	return SanitizeParameterName(paramName)
}