/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package global

import (
	"fmt"
	"strings"
)

// ParseToolName extracts the service name and endpoint ID from a tool name.
// Tool names follow the format: {serviceName}_{endpointId}
// For example: "microsoft365_calendar_read_summary" -> ("microsoft365", "calendar_read_summary")
func ParseToolName(toolName string) (serviceName string, endpointID string, err error) {
	if toolName == "" {
		return "", "", fmt.Errorf("tool name cannot be empty")
	}

	// Find the first underscore to split service name from endpoint ID
	firstUnderscore := strings.Index(toolName, "_")
	if firstUnderscore == -1 {
		return "", "", fmt.Errorf("invalid tool name format: %s (expected format: serviceName_endpointId)", toolName)
	}

	serviceName = toolName[:firstUnderscore]
	endpointID = toolName[firstUnderscore+1:]

	if serviceName == "" {
		return "", "", fmt.Errorf("service name cannot be empty in tool: %s", toolName)
	}

	if endpointID == "" {
		return "", "", fmt.Errorf("endpoint ID cannot be empty in tool: %s", toolName)
	}

	return serviceName, endpointID, nil
}

// ExtractServiceFromToolName is a convenience function that only returns the service name
func ExtractServiceFromToolName(toolName string) (string, error) {
	serviceName, _, err := ParseToolName(toolName)
	return serviceName, err
}

// ValidateToolName checks if a tool name follows the expected format
func ValidateToolName(toolName string) error {
	_, _, err := ParseToolName(toolName)
	return err
}

// BuildToolName constructs a tool name from service name and endpoint ID
func BuildToolName(serviceName, endpointID string) string {
	return fmt.Sprintf("%s_%s", serviceName, endpointID)
}
