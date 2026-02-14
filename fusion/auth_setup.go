/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// AuthCodeBlob contains the data needed by the fusion-auth CLI tool to initiate
// an OAuth2 authorization code flow on behalf of a tenant.
type AuthCodeBlob struct {
	URL     string `json:"u"`
	Code    string `json:"c"`
	Service string `json:"s"`
}

// createAuthSetupToolDefinition creates an MCP tool definition that generates auth
// instructions when API calls fail due to missing authentication for a service.
func (f *Fusion) createAuthSetupToolDefinition(serviceName string, service *ServiceConfig) global.ToolDefinition {
	toolName := fmt.Sprintf("%s_auth_setup", serviceName)
	authType := service.Auth.Type

	var description string
	switch authType {
	case AuthTypeUserCredentials:
		description = fmt.Sprintf(
			"Generates authentication instructions for %s when API calls fail due to missing credentials. "+
				"Returns a fusion-auth command the user can run to provide required credentials.",
			service.Name,
		)
	default:
		description = fmt.Sprintf(
			"Generates authentication instructions for %s when API calls fail due to missing authentication. "+
				"Returns a fusion-auth command the user can run on a machine with a web browser to complete OAuth setup.",
			service.Name,
		)
	}

	return global.ToolDefinition{
		Name:        toolName,
		Description: description,
		Parameters:  []global.Parameter{},
		Handler:     f.createAuthSetupHandler(serviceName, authType),
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(false),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(true),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// createAuthSetupHandler returns a tool handler that generates a time-limited auth code
// and returns instructions for the user to authenticate via the fusion-auth CLI.
func (f *Fusion) createAuthSetupHandler(serviceName string, authType AuthType) global.ToolHandler {
	return func(options map[string]any) (string, error) {
		// Extract context from options using the __mcp_context key pattern
		ctx := context.Background()
		if ctxValue, exists := options["__mcp_context"]; exists {
			if contextFromMCP, ok := ctxValue.(context.Context); ok {
				ctx = contextFromMCP
			}
		}

		// Extract TenantContext from context
		tenantContextValue := ctx.Value(global.TenantContextKey)
		if tenantContextValue == nil {
			return "", fmt.Errorf("no tenant context found - authentication required")
		}
		tenantContext, ok := tenantContextValue.(*TenantContext)
		if !ok {
			return "", fmt.Errorf("invalid tenant context type")
		}

		// Verify multi-tenant auth is available
		if f.multiTenantAuth == nil {
			return "", fmt.Errorf("multi-tenant authentication is not configured")
		}

		// Verify external URL is configured
		if f.externalURL == "" {
			return "", fmt.Errorf("external URL is not configured - set MCP_FUSION_EXTERNAL_URL")
		}

		// Look up the service config for the display name
		service := f.config.Services[serviceName]
		if service == nil {
			return "", fmt.Errorf("service %s not found in configuration", serviceName)
		}

		// Invalidate any existing token for this tenant+service so stale
		// tokens don't persist after re-authentication
		serviceContext := &TenantContext{
			TenantHash:  tenantContext.TenantHash,
			ServiceName: serviceName,
		}
		f.multiTenantAuth.InvalidateToken(serviceContext)

		if f.logger != nil {
			f.logger.Infof("Invalidated existing token for tenant %s service %s before re-auth",
				tenantContext.ShortHash(), serviceName)
		}

		// Create a time-limited auth code (15 minutes)
		code, err := f.multiTenantAuth.CreateAuthCode(tenantContext.TenantHash, serviceName, 15*time.Minute)
		if err != nil {
			return "", fmt.Errorf("failed to create auth code: %w", err)
		}

		// Build and encode the auth code blob
		blob := AuthCodeBlob{
			URL:     f.externalURL,
			Code:    code,
			Service: serviceName,
		}
		blobJSON, _ := json.Marshal(blob)
		encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(blobJSON)

		if f.logger != nil {
			f.logger.Infof("Generated auth setup code for tenant %s service %s",
				tenantContext.ShortHash(), serviceName)
		}

		var message string
		switch authType {
		case AuthTypeUserCredentials:
			// Check for optional setup instructions in the service auth config
			var instructionsBlock string
			if instructions, ok := service.Auth.Config["instructions"].(string); ok && instructions != "" {
				instructionsBlock = "\n\n" + instructions + "\n"
			}
			message = fmt.Sprintf(
				"Credentials are required for %s.%s\n\nPlease run the following command. IMPORTANT: Present this command in a markdown code block so the user can copy it without line breaks.\n\n```\nfusion-auth %s\n```\n\n"+
					"This auth code expires in 15 minutes. After authenticating, retry your previous request.",
				service.Name, instructionsBlock, encoded,
			)
		default:
			message = fmt.Sprintf(
				"Authentication is required for %s. Please run the following command on a machine with a web browser. IMPORTANT: Present this command in a markdown code block so the user can copy it without line breaks.\n\n```\nfusion-auth %s\n```\n\n"+
					"This auth code expires in 15 minutes. After authenticating, retry your previous request.",
				service.Name, encoded,
			)
		}

		return message, nil
	}
}
