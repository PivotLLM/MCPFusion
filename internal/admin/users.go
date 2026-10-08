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

// UserCommand holds the user management flags. At most one action runs, in the
// order add, list, delete, link, unlink.
type UserCommand struct {
	Add    string // -user-add: description of the user to create
	Token  string // -user-token: also create a token with this description
	List   bool   // -user-list
	Delete string // -user-delete: user ID
	Link   string // -user-link: user_id:key_hash
	Unlink string // -user-unlink: key hash
}

// Requested reports whether a user management action was asked for.
func (c UserCommand) Requested() bool {
	return c.Add != "" || c.List || c.Delete != "" || c.Link != "" || c.Unlink != ""
}

// Run performs the requested user management action.
func (c UserCommand) Run(database db.Database) error {
	if c.Add != "" {
		return userAdd(database, c.Add, c.Token)
	}
	if c.List {
		return userList(database)
	}
	if c.Delete != "" {
		return userDelete(database, c.Delete)
	}
	if c.Link != "" {
		return userLink(database, c.Link)
	}
	if c.Unlink != "" {
		return userUnlink(database, c.Unlink)
	}
	return nil
}

// userAdd creates a new user
func userAdd(database db.Database, description string, tokenDesc string) error {
	user, err := database.CreateUser(description)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Printf("\nUser created successfully\n\n")
	fmt.Printf("User ID:     %s\n", user.UserID)
	fmt.Printf("Description: %s\n", user.Description)
	fmt.Printf("Created:     %s\n\n", user.CreatedAt.Format("2006-01-02 15:04:05"))

	// Create and link API token if requested
	if tokenDesc != "" {
		token, hash, err := database.AddAPIToken(tokenDesc)
		if err != nil {
			fmt.Printf("WARNING: User created but failed to create API token: %v\n", err)
			return nil
		}

		if err := database.LinkAPIKey(user.UserID, hash); err != nil {
			fmt.Printf("WARNING: Token created but failed to link to user: %v\n", err)
		}

		fmt.Printf("SECURITY WARNING: This token will only be displayed once!\n")
		fmt.Printf("   Copy it now and store it securely.\n")
		fmt.Printf("\n")
		fmt.Printf("Token:       %s\n", token)
		fmt.Printf("Hash:        %s\n", hash[:12])
		fmt.Printf("\n")
		fmt.Printf("Use this token in the Authorization header:\n")
		fmt.Printf("  Authorization: Bearer %s\n", token)
		fmt.Printf("\n")
	}

	return nil
}

// userList displays all users
func userList(database db.Database) error {
	users, err := database.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if len(users) == 0 {
		fmt.Printf("No users found.\n")
		fmt.Printf("Create one with: %s -user-add \"Description\"\n", os.Args[0])
		return nil
	}

	fmt.Printf("Users:\n")
	fmt.Printf("%-38s %-20s %-20s %s\n", "USER ID", "CREATED", "UPDATED", "DESCRIPTION")
	fmt.Printf("%-38s %-20s %-20s %s\n", "-------", "-------", "-------", "-----------")

	for _, user := range users {
		description := user.Description
		if len(description) > 40 {
			description = description[:37] + "..."
		}

		fmt.Printf("%-38s %-20s %-20s %s\n",
			user.UserID,
			user.CreatedAt.Format("2006-01-02 15:04:05"),
			user.UpdatedAt.Format("2006-01-02 15:04:05"),
			description)
	}

	fmt.Printf("\nTotal: %d users\n", len(users))
	return nil
}

// userDelete removes a user
func userDelete(database db.Database, userID string) error {
	// Verify user exists
	user, err := database.GetUser(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	fmt.Printf("User Details:\n")
	fmt.Printf("  ID:          %s\n", user.UserID)
	fmt.Printf("  Description: %s\n", user.Description)
	fmt.Printf("  Created:     %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Printf("\nAre you sure you want to delete this user and all associated data? (y/N): ")
	var response string
	_, err = fmt.Scanln(&response)
	if err != nil {
		return err
	}

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Printf("User deletion cancelled.\n")
		return nil
	}

	if err := database.DeleteUser(userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	fmt.Printf("User deleted successfully.\n")
	return nil
}

// userLink links an API key to a user
func userLink(database db.Database, linkSpec string) error {
	parts := strings.SplitN(linkSpec, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid format. Use: -user-link user_id:key_hash")
	}

	userID := parts[0]
	keyHash := parts[1]

	if err := database.LinkAPIKey(userID, keyHash); err != nil {
		return fmt.Errorf("failed to link API key: %w", err)
	}

	displayHash := keyHash
	if len(displayHash) > 12 {
		displayHash = displayHash[:12]
	}
	fmt.Printf("API key %s linked to user %s\n", displayHash, userID)
	return nil
}

// userUnlink unlinks an API key from its user
func userUnlink(database db.Database, keyHash string) error {
	if err := database.UnlinkAPIKey(keyHash); err != nil {
		return fmt.Errorf("failed to unlink API key: %w", err)
	}

	displayHash := keyHash
	if len(displayHash) > 12 {
		displayHash = displayHash[:12]
	}
	fmt.Printf("API key %s unlinked from user\n", displayHash)
	return nil
}
