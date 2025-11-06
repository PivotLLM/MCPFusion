/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import (
	"context"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/mark3labs/mcp-go/mcp"
)

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterListPrompts(ctx context.Context, id any, request *mcp.ListPromptsRequest, result *mcp.ListPromptsResult) {
	if s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.Prompts)
	} else {
		s.logger.Infof("%s: %s items returned", request.Request.Method, len(result.Prompts))
	}
}

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterListResources(ctx context.Context, id any, request *mcp.ListResourcesRequest, result *mcp.ListResourcesResult) {
	if s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.Resources)
	} else {
		s.logger.Infof("%s: %s items returned", request.Request.Method, len(result.Resources))
	}
}

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterListResourceTemplates(ctx context.Context, id any, request *mcp.ListResourceTemplatesRequest, result *mcp.ListResourceTemplatesResult) {
	if s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.ResourceTemplates)
	} else {
		s.logger.Infof("%s: %s items returned", request.Request.Method, len(result.ResourceTemplates))
	}
}

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterListTools(ctx context.Context, id any, request *mcp.ListToolsRequest, result *mcp.ListToolsResult) {
	if //goland:noinspection GoBoolExpressions
	global.DumpTools && s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.Tools)
	} else {
		s.logger.Infof("%s: %d tools returned", request.Request.Method, len(result.Tools))
	}
}

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterCallTool(ctx context.Context, id any, request *mcp.CallToolRequest, result *mcp.CallToolResult) {
	// Calculate response size
	var responseSize int
	if result != nil {
		// Count content items and their sizes
		for _, content := range result.Content {
			// Use type assertion to check content type (note: these are value types, not pointers)
			switch c := content.(type) {
			case mcp.TextContent:
				responseSize += len(c.Text)
			case *mcp.TextContent:
				responseSize += len(c.Text)
			case mcp.ImageContent:
				responseSize += len(c.Data)
			case *mcp.ImageContent:
				responseSize += len(c.Data)
			case mcp.EmbeddedResource:
				// Count embedded resource content
				if textResource, ok := c.Resource.(*mcp.TextResourceContents); ok {
					responseSize += len(textResource.Text)
				}
			case *mcp.EmbeddedResource:
				// Count embedded resource content
				if textResource, ok := c.Resource.(*mcp.TextResourceContents); ok {
					responseSize += len(textResource.Text)
				}
			}
		}
	}

	toolName := request.Params.Name
	if s.debug {
		s.logger.Debugf("tools/call: %s completed, response size: %d bytes", toolName, responseSize)
	} else {
		s.logger.Infof("tools/call: %s completed (%d bytes)", toolName, responseSize)
	}
}
