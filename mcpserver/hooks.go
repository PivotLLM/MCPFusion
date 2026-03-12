/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
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
	count := 0
	if result != nil {
		count = len(result.Prompts)
	}
	if rec, ok := ctx.Value(global.RequestRecordKey).(*global.RequestRecord); ok && rec != nil {
		rec.MCPMethod = string(request.Request.Method)
		rec.Bytes = count
		rec.IsList = true
		rec.Status = "ok"
	} else if s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.Prompts)
	} else {
		s.logger.Infof("%s: %d items returned", request.Request.Method, count)
	}
}

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterListResources(ctx context.Context, id any, request *mcp.ListResourcesRequest, result *mcp.ListResourcesResult) {
	count := 0
	if result != nil {
		count = len(result.Resources)
	}
	if rec, ok := ctx.Value(global.RequestRecordKey).(*global.RequestRecord); ok && rec != nil {
		rec.MCPMethod = string(request.Request.Method)
		rec.Bytes = count
		rec.IsList = true
		rec.Status = "ok"
	} else if s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.Resources)
	} else {
		s.logger.Infof("%s: %d items returned", request.Request.Method, count)
	}
}

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterListResourceTemplates(ctx context.Context, id any, request *mcp.ListResourceTemplatesRequest, result *mcp.ListResourceTemplatesResult) {
	count := 0
	if result != nil {
		count = len(result.ResourceTemplates)
	}
	if rec, ok := ctx.Value(global.RequestRecordKey).(*global.RequestRecord); ok && rec != nil {
		rec.MCPMethod = string(request.Request.Method)
		rec.Bytes = count
		rec.IsList = true
		rec.Status = "ok"
	} else if s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.ResourceTemplates)
	} else {
		s.logger.Infof("%s: %d items returned", request.Request.Method, count)
	}
}

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterListTools(ctx context.Context, id any, request *mcp.ListToolsRequest, result *mcp.ListToolsResult) {
	count := 0
	if result != nil {
		count = len(result.Tools)
	}
	if rec, ok := ctx.Value(global.RequestRecordKey).(*global.RequestRecord); ok && rec != nil {
		rec.MCPMethod = string(request.Request.Method)
		rec.Bytes = count
		rec.IsList = true
		rec.Status = "ok"
	} else if //goland:noinspection GoBoolExpressions
	global.DumpTools && s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.Tools)
	} else {
		s.logger.Infof("%s: %d tools returned", request.Request.Method, count)
	}
}

//goland:noinspection GoUnusedParameter
func (s *MCPServer) hookAfterCallTool(ctx context.Context, id any, request *mcp.CallToolRequest, rawResult any) {
	// Type-assert result — mcp-go v0.45.0 changed the hook signature to any
	result, _ := rawResult.(*mcp.CallToolResult)

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
	status := "ok"
	if result != nil && result.IsError {
		status = "error"
	}

	if rec, ok := ctx.Value(global.RequestRecordKey).(*global.RequestRecord); ok && rec != nil {
		rec.MCPMethod = "tools/call"
		rec.ToolName = toolName
		rec.Status = status
		rec.Bytes = responseSize
	} else {
		s.logger.Infof("tools/call: %s %s (%d bytes)", toolName, status, responseSize)
	}
}
