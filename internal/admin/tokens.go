/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package admin

import (
	"fmt"
	"os"
	"strings"

	"github.com/PivotLLM/MCPFusion/db"
)

// TokenCommand holds the token management flags. At most one action runs, in
// the order add, list, delete.
type TokenCommand struct {
	Add    string // -token-add: description of the token to create
	User   string // -token-user: user ID to link a new token to
	List   bool   // -token-list
	Delete string // -token-del: token prefix or hash
}

// Requested reports whether a token management action was asked for.
func (c TokenCommand) Requested() bool {
	return c.Add != "" || c.List || c.Delete != ""
}

// Run performs the requested token management action.
func (c TokenCommand) Run(database db.Database) error {
	if c.Add != "" {
		return tokenAdd(database, c.Add, c.User)
	}

	if c.List {
		return tokenList(database)
	}

	if c.Delete != "" {
		return tokenDelete(database, c.Delete)
	}

	return nil
}

// tokenAdd creates a new API token
func tokenAdd(database db.Database, description string, userID string) error {
	if description == "" {
		description = "API Token"
	}

	// Validate description length
	if len(description) > 255 {
		return fmt.Errorf("description too long (max 255 characters)")
	}

	fmt.Printf("Generating new API token...\n")

	token, hash, err := database.AddAPIToken(description)
	if err != nil {
		return fmt.Errorf("failed to create API token: %w", err)
	}

	// Show the token only once with security warning
	fmt.Printf("\n")
	fmt.Printf("API Token created successfully\n")
	fmt.Printf("\n")
	fmt.Printf("SECURITY WARNING: This token will only be displayed once!\n")
	fmt.Printf("   Copy it now and store it securely.\n")
	fmt.Printf("\n")
	fmt.Printf("Token:       %s\n", token)
	fmt.Printf("Hash:        %s\n", hash[:12])
	fmt.Printf("Description: %s\n", description)
	fmt.Printf("\n")
	fmt.Printf("Use this token in the Authorization header:\n")
	fmt.Printf("  Authorization: Bearer %s\n", token)
	fmt.Printf("\n")

	// Link token to user if specified
	if userID != "" {
		if err := database.LinkAPIKey(userID, hash); err != nil {
			fmt.Printf("WARNING: Token created but failed to link to user %s: %v\n", userID, err)
		} else {
			fmt.Printf("Token linked to user %s\n", userID)
		}
	}

	return nil
}

// tokenList displays all API tokens
func tokenList(database db.Database) error {
	tokens, err := database.ListAPITokens()
	if err != nil {
		return fmt.Errorf("failed to list API tokens: %w", err)
	}

	if len(tokens) == 0 {
		fmt.Printf("No API tokens found.\n")
		fmt.Printf("Create one with: %s -token-add \"Description\"\n", os.Args[0])
		return nil
	}

	fmt.Printf("API Tokens:\n")
	fmt.Printf("%-10s %-20s %-20s %-20s %s\n", "PREFIX", "HASH", "CREATED", "LAST USED", "DESCRIPTION")
	fmt.Printf("%-10s %-20s %-20s %-20s %s\n", "------", "----", "-------", "---------", "-----------")

	for _, token := range tokens {
		prefix := token.Hash[:8]
		shortHash := token.Hash[:12]

		createdAt := token.CreatedAt.Format("2006-01-02 15:04:05")

		lastUsed := "Never used"
		if !token.LastUsed.IsZero() {
			lastUsed = token.LastUsed.Format("2006-01-02 15:04:05")
		}

		description := token.Description
		if len(description) > 30 {
			description = description[:27] + "..."
		}

		fmt.Printf("%-10s %-20s %-20s %-20s %s\n", prefix, shortHash, createdAt, lastUsed, description)
	}

	fmt.Printf("\nTotal: %d tokens\n", len(tokens))
	return nil
}

// tokenDelete removes an API token
func tokenDelete(database db.Database, identifier string) error {
	if identifier == "" {
		return fmt.Errorf("token identifier is required")
	}

	// List tokens to find matching one
	tokens, err := database.ListAPITokens()
	if err != nil {
		return fmt.Errorf("failed to list API tokens: %w", err)
	}

	var matchedToken *db.APITokenMetadata
	for _, token := range tokens {
		if token.Hash == identifier || strings.HasPrefix(token.Hash, identifier) {
			if matchedToken != nil {
				return fmt.Errorf("multiple tokens match '%s'. Please use a longer prefix", identifier)
			}
			matchedToken = &token
		}
	}

	if matchedToken == nil {
		return fmt.Errorf("no API token found matching '%s'", identifier)
	}

	// Show token details and confirm deletion
	fmt.Printf("Token Details:\n")
	fmt.Printf("  Hash: %s\n", matchedToken.Hash[:12])
	fmt.Printf("  Description: %s\n", matchedToken.Description)
	fmt.Printf("  Created: %s\n", matchedToken.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Printf("Are you sure you want to delete this token? (y/N): ")
	var response string
	_, err = fmt.Scanln(&response)
	if err != nil {
		return err
	}

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Printf("Token deletion cancelled.\n")
		return nil
	}

	if err := database.DeleteAPIToken(matchedToken.Hash); err != nil {
		return fmt.Errorf("failed to delete API token: %w", err)
	}

	fmt.Printf("Token deleted successfully.\n")
	return nil
}
