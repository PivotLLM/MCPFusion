/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MCPServer) AddTools() {

	// Iterate over tool providers and register their tools
	for _, provider := range s.toolProviders {

		// Call the Register function of the provider to get tool definitions
		toolDefinitions := provider.RegisterTools()

		// Iterate over the tool definitions and register each tool
		for _, toolDef := range toolDefinitions {

			// Combine description and parameters into a slice of options
			toolOptions := []mcp.ToolOption{
				mcp.WithDescription(toolDef.Description),
			}
			for _, param := range toolDef.Parameters {
				options := []mcp.PropertyOption{mcp.Description(param.Description)}
				if param.Required {
					options = append(options, mcp.Required())
				}

				// Use appropriate MCP parameter type based on param.Type
				var toolOption mcp.ToolOption
				switch param.Type {
				case "string":
					toolOption = mcp.WithString(param.Name, options...)
				case "number":
					toolOption = mcp.WithNumber(param.Name, options...)
				case "boolean":
					toolOption = mcp.WithBoolean(param.Name, options...)
				case "array":
					toolOption = mcp.WithArray(param.Name, options...)
				case "object":
					toolOption = mcp.WithObject(param.Name, options...)
				default:
					// Fallback to string for unknown types
					toolOption = mcp.WithString(param.Name, options...)
				}

				toolOptions = append(toolOptions, toolOption)
			}

			// Create the tool with all options
			tool := mcp.NewTool(toolDef.Name, toolOptions...)

			// Register the tool with the MCP server, creating a handler compatible with the MCP server
			// that wraps the tool's handler function with the provided options
			s.srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

				// Copy the MCP arguments to a map
				options := req.GetArguments()

				// Always pass context through options for fusion handlers to use
				// This allows fusion providers to access tenant context
				ctxOptions := make(map[string]any)
				for k, v := range options {
					ctxOptions[k] = v
				}
				// Store the context for fusion handlers to extract tenant context
				ctxOptions["__mcp_context"] = ctx

				// Debug: Log that we're passing context
				if s.logger != nil {
					s.logger.Debugf("MCP server passing context to tool %s", toolDef.Name)
				}

				result, err := toolDef.Handler(ctxOptions)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}
				return mcp.NewToolResultText(result), nil
			})
		}
	}
}
