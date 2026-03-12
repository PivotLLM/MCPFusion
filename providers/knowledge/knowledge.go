/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package knowledge provides knowledge-store MCP tools as a standalone ToolProvider.
package knowledge

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/metrics"
)

//go:embed knowledge_readme.md
var knowledgeReadme string

// UserIDExtractor extracts a user ID from a context.  It returns an error when
// no authenticated user can be identified.
type UserIDExtractor func(ctx context.Context) (string, error)

// Provider implements global.ToolProvider for the knowledge-store tools.
type Provider struct {
	logger    global.Logger
	database  db.Database
	collector *metrics.Collector
	extractor UserIDExtractor
}

// Option is a functional option for configuring a Provider.
type Option func(*Provider)

// WithLogger sets the logger.
func WithLogger(l global.Logger) Option {
	return func(p *Provider) { p.logger = l }
}

// WithDatabase sets the database used to persist knowledge entries.
func WithDatabase(d db.Database) Option {
	return func(p *Provider) { p.database = d }
}

// WithCollector sets the shared metrics collector.  When set, every tool call
// records a request via collector.RecordRequest("knowledge", isError).
func WithCollector(c *metrics.Collector) Option {
	return func(p *Provider) { p.collector = c }
}

// WithUserIDExtractor sets the function used to extract the calling user's ID
// from the MCP request context.
func WithUserIDExtractor(e UserIDExtractor) Option {
	return func(p *Provider) { p.extractor = e }
}

// New creates a new knowledge Provider with the given options.
func New(opts ...Option) *Provider {
	p := &Provider{}
	for _, o := range opts {
		o(p)
	}
	return p
}

// ToolCount returns the number of tools this provider registers without
// triggering any logging side effects.  Used to pre-register metrics before
// the MCP server calls RegisterTools.
func (p *Provider) ToolCount() int {
	if p.database == nil {
		return 0
	}
	return 5
}

// RegisterTools implements global.ToolProvider.  It returns nil (with a warning
// log) when no database is configured.
func (p *Provider) RegisterTools() []global.ToolDefinition {
	if p.database == nil {
		if p.logger != nil {
			p.logger.Warning("No database available, skipping knowledge tool registration")
		}
		return nil
	}

	tools := []global.ToolDefinition{
		p.knowledgeSetTool(),
		p.knowledgeGetTool(),
		p.knowledgeDeleteTool(),
		p.knowledgeRenameTool(),
		p.knowledgeSearchTool(),
	}

	if p.logger != nil {
		p.logger.Infof("Registered %d knowledge management tools", len(tools))
	}

	return tools
}

// toolHandler wraps a knowledge tool handler with context extraction and
// tenant resolution.
type toolHandler struct {
	provider *Provider
	handler  func(ctx context.Context, userID string, args map[string]interface{}) (string, error)
}

// call implements the global.ToolHandler signature.
func (h *toolHandler) call(args map[string]interface{}) (string, error) {
	ctx := context.Background()
	filteredArgs := make(map[string]interface{})

	for k, v := range args {
		if k == "__mcp_context" {
			if c, ok := v.(context.Context); ok {
				ctx = c
			}
			continue
		}
		filteredArgs[k] = v
	}

	// Resolve user ID via the injected extractor.
	var userID string
	if h.provider.extractor != nil {
		var err error
		userID, err = h.provider.extractor(ctx)
		if err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("no user ID extractor configured")
	}

	result, err := h.handler(ctx, userID, filteredArgs)

	// Record to shared collector for cross-package health reporting.
	if h.provider.collector != nil {
		h.provider.collector.RecordRequest("knowledge", err != nil)
	}

	return result, err
}

// knowledgeSetTool returns the tool definition for knowledge_set.
func (p *Provider) knowledgeSetTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name: "knowledge_set",
		Description: "Store or update a knowledge entry. Use this to remember user preferences, " +
			"rules, and context that should persist across sessions. Organize entries by domain " +
			"(e.g., 'email', 'calendar', 'general') and a descriptive key. " +
			"When the user asks you to remember new domains or instructions, update the " +
			"'system' domain 'readme' entry to include a pointer so future sessions know to consult it.",
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
		Handler: (&toolHandler{
			provider: p,
			handler: func(_ context.Context, userID string, args map[string]interface{}) (string, error) {
				domain, _ := args["domain"].(string)
				key, _ := args["key"].(string)
				content, _ := args["content"].(string)

				entry := &db.KnowledgeEntry{
					Domain:  domain,
					Key:     key,
					Content: content,
				}

				if err := p.database.SetKnowledge(userID, entry); err != nil {
					return "", fmt.Errorf("failed to store knowledge: %w", err)
				}

				return fmt.Sprintf("Knowledge entry stored: domain=%s, key=%s", domain, key), nil
			},
		}).call,
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(false),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(true),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// knowledgeGetTool returns the tool definition for knowledge_get.
func (p *Provider) knowledgeGetTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name: "knowledge_get",
		Description: "Retrieve knowledge entries. Call with both domain and key to get a specific entry, " +
			"with only domain to list all entries in that domain, or with neither to list all knowledge " +
			"entries across all domains. " +
			"IMPORTANT: At the start of a session, read domain='system' key='readme' for user preferences " +
			"and instructions on which knowledge domains to consult before performing tasks.",
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
		Handler: (&toolHandler{
			provider: p,
			handler: func(_ context.Context, userID string, args map[string]interface{}) (string, error) {
				domain, _ := args["domain"].(string)
				key, _ := args["key"].(string)

				// Validate: key requires domain.
				if key != "" && domain == "" {
					return "", fmt.Errorf("'key' requires 'domain' to be set")
				}

				// Specific entry requested.
				if domain != "" && key != "" {
					// system/readme: always prepend the embedded header.
					if domain == "system" && key == "readme" {
						entry, err := p.database.GetKnowledge(userID, domain, key)
						if err != nil {
							// No user content yet — return just the embedded header.
							return knowledgeReadme, nil
						}
						// Prepend embedded header to the user's stored content.
						return knowledgeReadme + entry.Content, nil
					}

					entry, err := p.database.GetKnowledge(userID, domain, key)
					if err != nil {
						return "", fmt.Errorf("failed to get knowledge: %w", err)
					}
					result, err := json.MarshalIndent(entry, "", "  ")
					if err != nil {
						return "", fmt.Errorf("failed to serialize knowledge entry: %w", err)
					}
					return string(result), nil
				}

				// List entries (all domains or a specific domain).
				entries, err := p.database.ListKnowledge(userID, domain)
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
		}).call,
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(true),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(true),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// knowledgeDeleteTool returns the tool definition for knowledge_delete.
func (p *Provider) knowledgeDeleteTool() global.ToolDefinition {
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
		Handler: (&toolHandler{
			provider: p,
			handler: func(_ context.Context, userID string, args map[string]interface{}) (string, error) {
				domain, _ := args["domain"].(string)
				key, _ := args["key"].(string)

				if err := p.database.DeleteKnowledge(userID, domain, key); err != nil {
					return "", fmt.Errorf("failed to delete knowledge: %w", err)
				}

				return fmt.Sprintf("Knowledge entry deleted: domain=%s, key=%s", domain, key), nil
			},
		}).call,
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(false),
			Destructive: global.BoolPtr(true),
			Idempotent:  global.BoolPtr(false),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// knowledgeRenameTool returns the tool definition for knowledge_rename.
func (p *Provider) knowledgeRenameTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name:        "knowledge_rename",
		Description: "Rename the key of an existing knowledge entry within a domain, preserving its content and metadata.",
		Parameters: []global.Parameter{
			{
				Name:        "domain",
				Description: "Category of the entry to rename",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "old_key",
				Description: "Current key of the entry",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "new_key",
				Description: "New key for the entry",
				Required:    true,
				Type:        "string",
			},
		},
		Handler: (&toolHandler{
			provider: p,
			handler: func(_ context.Context, userID string, args map[string]interface{}) (string, error) {
				domain, _ := args["domain"].(string)
				oldKey, _ := args["old_key"].(string)
				newKey, _ := args["new_key"].(string)

				if err := p.database.RenameKnowledge(userID, domain, oldKey, newKey); err != nil {
					return "", fmt.Errorf("failed to rename knowledge: %w", err)
				}

				return fmt.Sprintf("Knowledge entry renamed: domain=%s, %s -> %s", domain, oldKey, newKey), nil
			},
		}).call,
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(false),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(false),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// knowledgeSearchTool returns the tool definition for knowledge_search.
func (p *Provider) knowledgeSearchTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name: "knowledge_search",
		Description: "Search knowledge entries by keyword. Performs a case-insensitive search across " +
			"domain names, keys, and content. Use this when you don't know the exact domain or key for an entry.",
		Parameters: []global.Parameter{
			{
				Name:        "query",
				Description: "Search term to match against domain, key, and content",
				Required:    true,
				Type:        "string",
			},
		},
		Handler: (&toolHandler{
			provider: p,
			handler: func(_ context.Context, userID string, args map[string]interface{}) (string, error) {
				query, _ := args["query"].(string)

				entries, err := p.database.SearchKnowledge(userID, query)
				if err != nil {
					return "", fmt.Errorf("failed to search knowledge: %w", err)
				}

				if len(entries) == 0 {
					return fmt.Sprintf("No knowledge entries matching '%s'", query), nil
				}

				result, err := json.MarshalIndent(entries, "", "  ")
				if err != nil {
					return "", fmt.Errorf("failed to serialize knowledge entries: %w", err)
				}
				return string(result), nil
			},
		}).call,
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(true),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(true),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}
