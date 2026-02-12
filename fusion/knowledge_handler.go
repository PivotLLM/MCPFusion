/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
)

// knowledgeToolHandler wraps a knowledge tool handler with context extraction and
// tenant resolution. It extracts the MCP context from the args map, retrieves the
// TenantContext, and delegates to the inner handler with the resolved user ID.
type knowledgeToolHandler struct {
	fusion  *Fusion
	handler func(ctx context.Context, userID string, args map[string]interface{}) (string, error)
}

// Call implements the global.ToolHandler signature by extracting the MCP context,
// resolving the tenant's user ID, and forwarding to the inner handler.
func (h *knowledgeToolHandler) Call(args map[string]interface{}) (string, error) {
	ctx := context.Background()
	filteredArgs := make(map[string]interface{})

	for k, v := range args {
		if k == "__mcp_context" {
			if contextFromMCP, ok := v.(context.Context); ok {
				ctx = contextFromMCP
			}
			continue
		}
		filteredArgs[k] = v
	}

	// Extract TenantContext from the request context
	tc, ok := ctx.Value(global.TenantContextKey).(*TenantContext)
	if !ok || tc == nil {
		return "", fmt.Errorf("no tenant context available")
	}

	if tc.UserID == "" {
		return "", fmt.Errorf(
			"no user ID associated with this API key. " +
				"Link the API key to a user with: mcpfusion -user-link <user_id>:<key_hash>")
	}

	return h.handler(ctx, tc.UserID, filteredArgs)
}

// registerKnowledgeTools creates and returns knowledge management tool definitions.
// Returns nil if no database is configured.
func (f *Fusion) registerKnowledgeTools() []global.ToolDefinition {
	if f.database == nil {
		if f.logger != nil {
			f.logger.Warning("No database available, skipping knowledge tool registration")
		}
		return nil
	}

	tools := []global.ToolDefinition{
		f.knowledgeSetTool(),
		f.knowledgeGetTool(),
		f.knowledgeDeleteTool(),
	}

	if f.logger != nil {
		f.logger.Infof("Registered %d knowledge management tools", len(tools))
	}

	return tools
}

// knowledgeSetTool returns the tool definition for knowledge_set.
func (f *Fusion) knowledgeSetTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name: "knowledge_set",
		Description: "Store or update a knowledge entry. Use this to remember user preferences, " +
			"rules, and context that should persist across sessions. Organize entries by domain " +
			"(e.g., 'email', 'calendar', 'general') and a descriptive key.",
		Parameters: []global.Parameter{
			{
				Name:        "domain",
				Description: "Category for the knowledge entry (e.g., 'email', 'calendar', 'contacts', 'general')",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "key",
				Description: "Unique identifier within the domain (e.g., 'meeting-preferences', 'newsletter-rules')",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "content",
				Description: "The knowledge content in natural language (e.g., 'User prefers 30-minute meetings in Eastern time')",
				Required:    true,
				Type:        "string",
			},
		},
		Handler: (&knowledgeToolHandler{
			fusion: f,
			handler: func(_ context.Context, userID string, args map[string]interface{}) (string, error) {
				domain, _ := args["domain"].(string)
				key, _ := args["key"].(string)
				content, _ := args["content"].(string)

				entry := &db.KnowledgeEntry{
					Domain:  domain,
					Key:     key,
					Content: content,
				}

				if err := f.database.SetKnowledge(userID, entry); err != nil {
					return "", fmt.Errorf("failed to store knowledge: %w", err)
				}

				return fmt.Sprintf("Knowledge entry stored: domain=%s, key=%s", domain, key), nil
			},
		}).Call,
	}
}

// knowledgeGetTool returns the tool definition for knowledge_get.
func (f *Fusion) knowledgeGetTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name: "knowledge_get",
		Description: "Retrieve knowledge entries. Call with both domain and key to get a specific entry, " +
			"with only domain to list all entries in that domain, or with neither to list all knowledge " +
			"entries across all domains.",
		Parameters: []global.Parameter{
			{
				Name:        "domain",
				Description: "Category to filter by (e.g., 'email', 'calendar'). Omit to list all domains.",
				Required:    false,
				Type:        "string",
			},
			{
				Name:        "key",
				Description: "Specific entry key within the domain. Requires domain to be set.",
				Required:    false,
				Type:        "string",
			},
		},
		Handler: (&knowledgeToolHandler{
			fusion: f,
			handler: func(_ context.Context, userID string, args map[string]interface{}) (string, error) {
				domain, _ := args["domain"].(string)
				key, _ := args["key"].(string)

				// Validate: key requires domain
				if key != "" && domain == "" {
					return "", fmt.Errorf("'key' requires 'domain' to be set")
				}

				// Specific entry requested
				if domain != "" && key != "" {
					entry, err := f.database.GetKnowledge(userID, domain, key)
					if err != nil {
						return "", fmt.Errorf("failed to get knowledge: %w", err)
					}
					result, err := json.MarshalIndent(entry, "", "  ")
					if err != nil {
						return "", fmt.Errorf("failed to serialize knowledge entry: %w", err)
					}
					return string(result), nil
				}

				// List entries (all domains or a specific domain)
				entries, err := f.database.ListKnowledge(userID, domain)
				if err != nil {
					return "", fmt.Errorf("failed to list knowledge: %w", err)
				}

				if len(entries) == 0 {
					if domain != "" {
						return fmt.Sprintf("No knowledge entries found in domain '%s'", domain), nil
					}
					return "No knowledge entries found", nil
				}

				result, err := json.MarshalIndent(entries, "", "  ")
				if err != nil {
					return "", fmt.Errorf("failed to serialize knowledge entries: %w", err)
				}
				return string(result), nil
			},
		}).Call,
	}
}

// knowledgeDeleteTool returns the tool definition for knowledge_delete.
func (f *Fusion) knowledgeDeleteTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name:        "knowledge_delete",
		Description: "Delete a knowledge entry by domain and key.",
		Parameters: []global.Parameter{
			{
				Name:        "domain",
				Description: "Category of the entry to delete",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "key",
				Description: "Key of the entry to delete",
				Required:    true,
				Type:        "string",
			},
		},
		Handler: (&knowledgeToolHandler{
			fusion: f,
			handler: func(_ context.Context, userID string, args map[string]interface{}) (string, error) {
				domain, _ := args["domain"].(string)
				key, _ := args["key"].(string)

				if err := f.database.DeleteKnowledge(userID, domain, key); err != nil {
					return "", fmt.Errorf("failed to delete knowledge: %w", err)
				}

				return fmt.Sprintf("Knowledge entry deleted: domain=%s, key=%s", domain, key), nil
			},
		}).Call,
	}
}
