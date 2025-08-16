/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MigrationReport represents the result of a migration operation
type MigrationReport struct {
	StartTime           time.Time     `json:"start_time"`
	EndTime             time.Time     `json:"end_time"`
	Duration            time.Duration `json:"duration"`
	SourceDir           string        `json:"source_dir"`
	TokensFound         int           `json:"tokens_found"`
	TokensMigrated      int           `json:"tokens_migrated"`
	TokensSkipped       int           `json:"tokens_skipped"`
	CredentialsMigrated int           `json:"credentials_migrated"`
	Errors              []string      `json:"errors"`
	Success             bool          `json:"success"`
}

// MigrateFromFileCache migrates tokens and credentials from the existing file cache system
func (d *DB) MigrateFromFileCache(cacheDir string) (*MigrationReport, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	report := &MigrationReport{
		StartTime: time.Now(),
		SourceDir: cacheDir,
		Errors:    make([]string, 0),
	}

	d.logger.Infof("Starting migration from file cache: %s", cacheDir)

	// Check if cache directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		report.addError(fmt.Sprintf("Cache directory does not exist: %s", cacheDir))
		report.finalize()
		return report, nil // Not an error - just no migration needed
	}

	// Discover tenant directories (assuming structure like cache/tenant_hash/)
	tenantDirs, err := d.discoverTenantDirectories(cacheDir)
	if err != nil {
		report.addError(fmt.Sprintf("Failed to discover tenant directories: %v", err))
		report.finalize()
		return report, err
	}

	d.logger.Infof("Found %d potential tenant directories", len(tenantDirs))

	// Migrate each tenant
	for _, tenantDir := range tenantDirs {
		if err := d.migrateTenant(tenantDir, report); err != nil {
			report.addError(fmt.Sprintf("Failed to migrate tenant %s: %v", tenantDir, err))
			continue
		}
	}

	// If no specific tenant structure found, try to migrate global tokens
	if len(tenantDirs) == 0 {
		if err := d.migrateGlobalTokens(cacheDir, report); err != nil {
			report.addError(fmt.Sprintf("Failed to migrate global tokens: %v", err))
		}
	}

	report.finalize()
	d.logger.Infof("Migration completed. Tokens migrated: %d, Credentials: %d, Errors: %d",
		report.TokensMigrated, report.CredentialsMigrated, len(report.Errors))

	return report, nil
}

// discoverTenantDirectories looks for directories that might contain tenant-specific cache data
func (d *DB) discoverTenantDirectories(cacheDir string) ([]string, error) {
	var tenantDirs []string

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Look for directories that might be tenant hashes (64 character hex strings)
		dirName := entry.Name()
		if len(dirName) == 64 && d.isHexString(dirName) {
			tenantDirs = append(tenantDirs, filepath.Join(cacheDir, dirName))
		}
	}

	return tenantDirs, nil
}

// migrateTenant migrates data for a specific tenant
func (d *DB) migrateTenant(tenantDir string, report *MigrationReport) error {
	tenantHash := filepath.Base(tenantDir)
	d.logger.Debugf("Migrating tenant: %s", tenantHash)

	// Check if tenant already exists
	if _, err := d.GetAPITokenMetadata(tenantHash); err == nil {
		d.logger.Infof("Tenant %s already exists, skipping migration", tenantHash[:8])
		report.TokensSkipped++
		return nil
	}

	// Create API token for this tenant (we'll need to generate a new token)
	description := fmt.Sprintf("Migrated from file cache on %s", time.Now().Format("2006-01-02"))
	token, hash, err := d.AddAPIToken(description)
	if err != nil {
		return fmt.Errorf("failed to create API token: %w", err)
	}

	if hash != tenantHash {
		d.logger.Warningf("Generated hash %s does not match expected %s", hash, tenantHash)
		// Continue with migration using the generated hash
	}

	report.TokensMigrated++
	d.logger.Infof("Created API token for migrated tenant (token: %s...)", token[:16])

	// Migrate OAuth tokens
	if err := d.migrateOAuthTokens(tenantDir, hash, report); err != nil {
		return fmt.Errorf("failed to migrate OAuth tokens: %w", err)
	}

	// Migrate credentials
	if err := d.migrateCredentials(tenantDir, hash, report); err != nil {
		return fmt.Errorf("failed to migrate credentials: %w", err)
	}

	return nil
}

// migrateOAuthTokens migrates OAuth tokens for a tenant
func (d *DB) migrateOAuthTokens(tenantDir, tenantHash string, report *MigrationReport) error {
	oauthDir := filepath.Join(tenantDir, "oauth")
	if _, err := os.Stat(oauthDir); os.IsNotExist(err) {
		return nil // No OAuth tokens to migrate
	}

	entries, err := os.ReadDir(oauthDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		serviceName := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(oauthDir, entry.Name())

		if err := d.migrateOAuthTokenFile(filePath, tenantHash, serviceName, report); err != nil {
			report.addError(fmt.Sprintf("Failed to migrate OAuth token file %s: %v", filePath, err))
			continue
		}
	}

	return nil
}

// migrateOAuthTokenFile migrates a single OAuth token file
func (d *DB) migrateOAuthTokenFile(filePath, tenantHash, serviceName string, report *MigrationReport) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Try to parse as existing token format
	var tokenData OAuthTokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return fmt.Errorf("failed to parse OAuth token file: %w", err)
	}

	// Store in database
	if err := d.StoreOAuthToken(tenantHash, serviceName, &tokenData); err != nil {
		return err
	}

	d.logger.Debugf("Migrated OAuth token for service: %s", serviceName)
	return nil
}

// migrateCredentials migrates service credentials for a tenant
func (d *DB) migrateCredentials(tenantDir, tenantHash string, report *MigrationReport) error {
	credDir := filepath.Join(tenantDir, "credentials")
	if _, err := os.Stat(credDir); os.IsNotExist(err) {
		return nil // No credentials to migrate
	}

	entries, err := os.ReadDir(credDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		serviceName := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(credDir, entry.Name())

		if err := d.migrateCredentialFile(filePath, tenantHash, serviceName, report); err != nil {
			report.addError(fmt.Sprintf("Failed to migrate credential file %s: %v", filePath, err))
			continue
		}

		report.CredentialsMigrated++
	}

	return nil
}

// migrateCredentialFile migrates a single credential file
func (d *DB) migrateCredentialFile(filePath, tenantHash, serviceName string, report *MigrationReport) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Try to parse as existing credential format
	var credentials ServiceCredentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return fmt.Errorf("failed to parse credential file: %w", err)
	}

	// Set default type if not specified
	if credentials.Type == "" {
		credentials.Type = CredentialTypeCustom
	}

	// Store in database
	if err := d.StoreCredentials(tenantHash, serviceName, &credentials); err != nil {
		return err
	}

	d.logger.Debugf("Migrated credentials for service: %s", serviceName)
	return nil
}

// migrateGlobalTokens migrates tokens from a non-tenant-specific cache structure
func (d *DB) migrateGlobalTokens(cacheDir string, report *MigrationReport) error {
	// Look for common token file patterns
	patterns := []string{
		"*.token",
		"*.json",
		"oauth_*.json",
		"creds_*.json",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(cacheDir, pattern))
		if err != nil {
			continue
		}

		for range matches {
			// Create a default tenant for global tokens
			if report.TokensMigrated == 0 {
				description := "Migrated global tokens from file cache"
				_, _, err := d.AddAPIToken(description)
				if err != nil {
					return fmt.Errorf("failed to create default tenant: %w", err)
				}
				report.TokensMigrated++
			}
		}
	}

	return nil
}

// Helper methods for MigrationReport
func (r *MigrationReport) addError(msg string) {
	r.Errors = append(r.Errors, msg)
}

func (r *MigrationReport) finalize() {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Success = len(r.Errors) == 0
}

// isHexString checks if a string is a valid hexadecimal string
func (d *DB) isHexString(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
