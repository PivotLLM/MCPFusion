/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
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
	if global.DumpTools && s.debug {
		s.logger.Debugf("%s: %v", request.Request.Method, result.Tools)
	} else {
		s.logger.Infof("%s: %d tools returned", request.Request.Method, len(result.Tools))
	}
}
