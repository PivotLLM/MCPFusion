/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// MCPAuthConfiguration holds configuration for MCP-level authentication
type MCPAuthConfiguration struct {
	authManager     *fusion.MultiTenantAuthManager
	serviceProvider ServiceProvider
	authorizer      global.Authorizer
	logger          global.Logger
}

// MCPAuthOption represents configuration options for MCP authentication middleware
type MCPAuthOption func(*MCPAuthConfiguration)

// WithMCPAuthManager sets the authentication manager
func WithMCPAuthManager(authManager *fusion.MultiTenantAuthManager) MCPAuthOption {
	return func(config *MCPAuthConfiguration) {
		config.authManager = authManager
	}
}

// WithMCPServiceProvider sets the service provider for validation
func WithMCPServiceProvider(serviceProvider ServiceProvider) MCPAuthOption {
	return func(config *MCPAuthConfiguration) {
		config.serviceProvider = serviceProvider
	}
}

// WithMCPLogger sets the logger for MCP authentication middleware
func WithMCPLogger(logger global.Logger) MCPAuthOption {
	return func(config *MCPAuthConfiguration) {
		config.logger = logger
	}
}

// WithMCPAuthorizer sets the authorizer for MCP tool-level authorization
func WithMCPAuthorizer(authorizer global.Authorizer) MCPAuthOption {
	return func(config *MCPAuthConfiguration) {
		config.authorizer = authorizer
	}
}

// WithMCPAuthentication creates a server option that adds MCP-level authentication middleware
// This middleware validates tenant access to specific tools and logs at the MCP protocol level
func WithMCPAuthentication(options ...MCPAuthOption) server.ServerOption {
	config := &MCPAuthConfiguration{}

	// Apply options
	for _, option := range options {
		option(config)
	}

	// Default to AllowAllAuthorizer if none is configured
	if config.authorizer == nil {
		config.authorizer = &global.AllowAllAuthorizer{}
	}

	if config.logger != nil {
		config.logger.Info("Initialized MCP-level authentication middleware")
	}

	return server.WithToolHandlerMiddleware(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Extract tenant context from the request context
			tenantContext, ok := ctx.Value(global.TenantContextKey).(*fusion.TenantContext)
			if !ok || tenantContext == nil {
				if config.logger != nil {
					config.logger.Errorf("MCP Auth: No tenant context found for tool call: %s", request.Params.Name)
				}
				return nil, fmt.Errorf("authentication required")
			}

			if config.logger != nil {
				config.logger.Debugf("MCP Auth: Processing tool call %s for tenant %s",
					request.Params.Name, tenantContext.ShortHash())
			}

			// Extract service name from tool name
			serviceName, err := global.ExtractServiceFromToolName(request.Params.Name)
			if err != nil {
				if config.logger != nil {
					config.logger.Errorf("MCP Auth: Failed to extract service from tool name %s: %v",
						request.Params.Name, err)
				}
				return nil, fmt.Errorf("invalid tool name: %s", request.Params.Name)
			}

			// Check if this is a command tool (not a service tool)
			// Command tools follow the pattern: command_{commandId}
			// They don't require service validation or tenant access checks
			if serviceName == "command" {
				if config.logger != nil {
					config.logger.Debugf("MCP Auth: Tool %s is a command tool, skipping service validation",
						request.Params.Name)
				}
				// Command tools are available to all authenticated tenants
				// Continue to tool handler without service-specific validation
				return next(ctx, request)
			}

			// Validate that this service exists in our configuration
			if config.serviceProvider != nil {
				availableServices := config.serviceProvider.GetAvailableServices()
				serviceFound := false
				for _, availableService := range availableServices {
					if availableService == serviceName {
						serviceFound = true
						break
					}
				}
				if !serviceFound {
					if config.logger != nil {
						config.logger.Errorf("MCP Auth: Service '%s' from tool '%s' not found in available services: %v",
							serviceName, request.Params.Name, availableServices)
					}
					return nil, fmt.Errorf("service '%s' not configured", serviceName)
				}
			}

			// Validate tenant access to the service
			if config.authManager != nil {
				if err := config.authManager.ValidateTenantAccess(tenantContext, serviceName); err != nil {
					if config.logger != nil {
						config.logger.Errorf("MCP Auth: Tenant access validation failed for %s service %s: %v",
							tenantContext.ShortHash(), serviceName, err)
					}
					return nil, fmt.Errorf("access denied to service: %s", serviceName)
				}
			}

			// Run tool-level authorization
			toolRequest := global.ToolRequest{
				TenantHash:  tenantContext.TenantHash,
				ServiceName: serviceName,
				ToolName:    request.Params.Name,
			}
			if err := config.authorizer.Authorize(ctx, toolRequest); err != nil {
				if config.logger != nil {
					config.logger.Errorf("MCP Auth: Authorization denied for tenant %s tool %s: %v",
						tenantContext.ShortHash(), request.Params.Name, err)
				}
				return nil, fmt.Errorf("authorization denied: %v", err)
			}

			if config.logger != nil {
				config.logger.Debugf("MCP Auth: Successfully validated tenant %s access to service %s for tool %s",
					tenantContext.ShortHash(), serviceName, request.Params.Name)
			}

			// Add service name to context for downstream handlers
			enrichedCtx := context.WithValue(ctx, global.ServiceNameKey, serviceName)

			// Continue to next handler with enriched context
			return next(enrichedCtx, request)
		}
	})
}
