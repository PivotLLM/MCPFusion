# MCPFusion Multi-Tenant Token Management with BoltDB
## Implementation Plan

This document outlines the complete implementation plan for refactoring MCPFusion to support multi-tenant token management using BoltDB (`go.etcd.io/bbolt`), based on the proven patterns from PivotChat.

## Executive Summary

MCPFusion will be refactored to support multi-tenant token management with the following features:

1. **BoltDB-based token storage** (`go.etcd.io/bbolt`) for persistence and performance
2. **Multi-tenant architecture** where each API token represents a separate namespace
3. **Secure token management** with auto-generated tokens, hashed storage, and cascade deletion
4. **CLI token management** for operational tasks

## 1. Database Schema Design

### Bucket Structure

```
mcpfusion.db
├── api_tokens/                    # Root bucket for API token management
│   ├── {hashed_api_token}/       # Bucket per API token (tenant namespace)
│   │   ├── metadata              # API token metadata (created_at, last_used, etc.)
│   │   ├── oauth_tokens/         # OAuth tokens for this tenant
│   │   │   ├── microsoft365      # Service-specific OAuth token
│   │   │   ├── google           # Service-specific OAuth token
│   │   │   └── ...              # Other services
│   │   └── service_credentials/  # Other service credentials for this tenant
│   │       ├── api_keys/        # API key credentials
│   │       ├── bearer_tokens/   # Bearer token credentials
│   │       └── basic_auth/      # Basic auth credentials
├── token_index/                  # For efficient lookups
│   ├── by_hash                  # hash -> metadata mapping
│   └── by_prefix                # partial hash -> full hash mapping (for CLI)
└── system/                      # System-wide settings
    ├── schema_version           # Database schema version for migrations
    └── stats                    # Usage statistics
```

### Data Structures

```go
// APITokenMetadata represents metadata for an API token
type APITokenMetadata struct {
    Hash        string    `json:"hash"`         // SHA-256 hash of the original token
    CreatedAt   time.Time `json:"created_at"`   // When the token was created
    LastUsed    time.Time `json:"last_used"`    // When the token was last used
    Description string    `json:"description"`  // Optional description
    Prefix      string    `json:"prefix"`       // First 8 chars for identification
}

// OAuthTokenData represents stored OAuth token information
type OAuthTokenData struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token,omitempty"`
    TokenType    string    `json:"token_type"`
    ExpiresAt    *time.Time `json:"expires_at,omitempty"`
    Scope        []string  `json:"scope,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// ServiceCredentials represents other service authentication data
type ServiceCredentials struct {
    Type      string                 `json:"type"`        // "api_key", "bearer", "basic"
    Data      map[string]interface{} `json:"data"`        // Service-specific credential data
    CreatedAt time.Time              `json:"created_at"`
    UpdatedAt time.Time              `json:"updated_at"`
}
```

## 2. Database Package Implementation

### Package Structure

```
db/
├── db.go              # Main database interface and constructor
├── api_tokens.go      # API token management operations
├── oauth_tokens.go    # OAuth token operations
├── credentials.go     # Service credentials operations
├── migration.go       # Migration utilities from file cache
├── types.go          # Data structure definitions
└── db_test.go        # Comprehensive test suite
```

### Core Database Interface

```go
package db

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    "path/filepath"
    "time"
    
    "github.com/PivotLLM/MCPFusion/global"
    "go.etcd.io/bbolt"
)

// DB represents the MCPFusion database
type DB struct {
    db      *bbolt.DB
    logger  global.Logger
    dataDir string
}

// Option defines a configuration option for the database
type Option func(*DB)

// New creates a new database instance with functional options
func New(opts ...Option) (*DB, error) {
    d := &DB{}
    
    // Apply options
    for _, opt := range opts {
        opt(d)
    }
    
    // Validate required options
    if d.logger == nil {
        return nil, fmt.Errorf("logger is required")
    }
    
    if d.dataDir == "" {
        // Determine data directory priority:
        // 1. /opt/mcpfusion (if writable)
        // 2. ~/.mcpfusion (user directory)
        d.dataDir = d.determineDataDirectory()
    }
    
    // Ensure data directory exists
    if err := os.MkdirAll(d.dataDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create data directory %s: %w", d.dataDir, err)
    }
    
    // Open database
    dbPath := filepath.Join(d.dataDir, "mcpfusion.db")
    options := &bbolt.Options{
        Timeout: 5 * time.Second,
    }
    
    db, err := bbolt.Open(dbPath, 0600, options)
    if err != nil {
        return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
    }
    d.db = db
    
    d.logger.Infof("Database opened at %s", dbPath)
    
    // Initialize schema
    if err := d.initializeSchema(); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to initialize database schema: %w", err)
    }
    
    return d, nil
}

// WithLogger sets the logger for the database
func WithLogger(logger global.Logger) Option {
    return func(d *DB) {
        d.logger = logger
    }
}

// WithDataDir sets the data directory for the database
func WithDataDir(dataDir string) Option {
    return func(d *DB) {
        d.dataDir = dataDir
    }
}

// determineDataDirectory determines the best data directory to use
func (d *DB) determineDataDirectory() string {
    // Try /opt/mcpfusion first
    systemDir := "/opt/mcpfusion"
    if err := os.MkdirAll(systemDir, 0755); err == nil {
        testFile := filepath.Join(systemDir, ".test")
        if err := ioutil.WriteFile(testFile, []byte("test"), 0644); err == nil {
            os.Remove(testFile)
            d.logger.Debugf("Using system data directory: %s", systemDir)
            return systemDir
        }
    }
    
    // Fall back to user directory
    homeDir, err := os.UserHomeDir()
    if err != nil {
        d.logger.Warningf("Cannot determine home directory: %v", err)
        return "/tmp/mcpfusion"
    }
    
    userDir := filepath.Join(homeDir, ".mcpfusion")
    d.logger.Debugf("Using user data directory: %s", userDir)
    return userDir
}

// initializeSchema creates the initial bucket structure
func (d *DB) initializeSchema() error {
    return d.db.Update(func(tx *bbolt.Tx) error {
        // Create root buckets
        buckets := []string{
            "api_tokens",
            "token_index", 
            "system",
        }
        
        for _, bucketName := range buckets {
            if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
                return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
            }
        }
        
        // Create index sub-buckets
        indexBucket := tx.Bucket([]byte("token_index"))
        indexSubBuckets := []string{"by_hash", "by_prefix"}
        for _, subBucket := range indexSubBuckets {
            if _, err := indexBucket.CreateBucketIfNotExists([]byte(subBucket)); err != nil {
                return fmt.Errorf("failed to create index sub-bucket %s: %w", subBucket, err)
            }
        }
        
        // Set schema version
        systemBucket := tx.Bucket([]byte("system"))
        return systemBucket.Put([]byte("schema_version"), []byte("1.0"))
    })
}

// Close closes the database connection
func (d *DB) Close() error {
    if d.db == nil {
        return nil
    }
    
    d.logger.Info("Closing database connection")
    err := d.db.Close()
    if err != nil {
        return fmt.Errorf("failed to close database: %w", err)
    }
    
    d.logger.Info("Database connection closed successfully")
    return nil
}
```

### API Token Management

```go
// api_tokens.go

// AddAPIToken generates a new API token and returns the token and its hash
func (d *DB) AddAPIToken(description string) (string, string, error) {
    // Generate a secure random token
    token, err := d.generateSecureToken()
    if err != nil {
        return "", "", fmt.Errorf("failed to generate token: %w", err)
    }
    
    // Generate hash
    hash := d.hashAPIToken(token)
    prefix := d.generatePrefix(token)
    
    metadata := APITokenMetadata{
        Hash:        hash,
        CreatedAt:   time.Now(),
        LastUsed:    time.Now(),
        Description: description,
        Prefix:      prefix,
    }
    
    return token, hash, d.db.Update(func(tx *bbolt.Tx) error {
        // Create tenant bucket
        apiTokensBucket := tx.Bucket([]byte("api_tokens"))
        tenantBucket, err := apiTokensBucket.CreateBucketIfNotExists([]byte(hash))
        if err != nil {
            return fmt.Errorf("failed to create tenant bucket: %w", err)
        }
        
        // Create sub-buckets for this tenant
        subBuckets := []string{"oauth_tokens", "service_credentials"}
        for _, subBucket := range subBuckets {
            if _, err := tenantBucket.CreateBucketIfNotExists([]byte(subBucket)); err != nil {
                return fmt.Errorf("failed to create sub-bucket %s: %w", subBucket, err)
            }
        }
        
        // Store metadata
        metadataData, err := json.Marshal(metadata)
        if err != nil {
            return fmt.Errorf("failed to marshal metadata: %w", err)
        }
        
        if err := tenantBucket.Put([]byte("metadata"), metadataData); err != nil {
            return fmt.Errorf("failed to store metadata: %w", err)
        }
        
        // Update indexes
        indexBucket := tx.Bucket([]byte("token_index"))
        
        // by_hash index
        byHashBucket := indexBucket.Bucket([]byte("by_hash"))
        if err := byHashBucket.Put([]byte(hash), metadataData); err != nil {
            return fmt.Errorf("failed to update hash index: %w", err)
        }
        
        // by_prefix index
        byPrefixBucket := indexBucket.Bucket([]byte("by_prefix"))
        if err := byPrefixBucket.Put([]byte(prefix), []byte(hash)); err != nil {
            return fmt.Errorf("failed to update prefix index: %w", err)
        }
        
        d.logger.Infof("Added API token with hash: %s (prefix: %s)", hash, prefix)
        return nil
    })
}

// ListAPITokens returns all API tokens with their metadata
func (d *DB) ListAPITokens() ([]APITokenMetadata, error) {
    var tokens []APITokenMetadata
    
    err := d.db.View(func(tx *bbolt.Tx) error {
        indexBucket := tx.Bucket([]byte("token_index"))
        byHashBucket := indexBucket.Bucket([]byte("by_hash"))
        
        return byHashBucket.ForEach(func(k, v []byte) error {
            var metadata APITokenMetadata
            if err := json.Unmarshal(v, &metadata); err != nil {
                d.logger.Warningf("Failed to unmarshal token metadata for hash %s: %v", string(k), err)
                return nil // Continue iteration
            }
            tokens = append(tokens, metadata)
            return nil
        })
    })
    
    return tokens, err
}

// DeleteAPIToken deletes an API token and all associated data
func (d *DB) DeleteAPIToken(hash string) error {
    return d.db.Update(func(tx *bbolt.Tx) error {
        // Get metadata first for logging
        var metadata APITokenMetadata
        apiTokensBucket := tx.Bucket([]byte("api_tokens"))
        tenantBucket := apiTokensBucket.Bucket([]byte(hash))
        if tenantBucket != nil {
            if metadataData := tenantBucket.Get([]byte("metadata")); metadataData != nil {
                json.Unmarshal(metadataData, &metadata)
            }
        }
        
        // Delete tenant bucket (cascade deletes all nested data)
        if err := apiTokensBucket.DeleteBucket([]byte(hash)); err != nil {
            return fmt.Errorf("failed to delete tenant bucket: %w", err)
        }
        
        // Update indexes
        indexBucket := tx.Bucket([]byte("token_index"))
        
        // Remove from by_hash index
        byHashBucket := indexBucket.Bucket([]byte("by_hash"))
        byHashBucket.Delete([]byte(hash))
        
        // Remove from by_prefix index
        if metadata.Prefix != "" {
            byPrefixBucket := indexBucket.Bucket([]byte("by_prefix"))
            byPrefixBucket.Delete([]byte(metadata.Prefix))
        }
        
        d.logger.Infof("Deleted API token with hash: %s (prefix: %s)", hash, metadata.Prefix)
        return nil
    })
}

// ValidateAPIToken checks if an API token is valid and updates last_used
func (d *DB) ValidateAPIToken(token string) (bool, string, error) {
    hash := d.hashAPIToken(token)
    
    var valid bool
    err := d.db.Update(func(tx *bbolt.Tx) error {
        apiTokensBucket := tx.Bucket([]byte("api_tokens"))
        tenantBucket := apiTokensBucket.Bucket([]byte(hash))
        
        if tenantBucket == nil {
            return nil // Token not found
        }
        
        // Token exists, update last_used
        metadataData := tenantBucket.Get([]byte("metadata"))
        if metadataData == nil {
            return nil // Corrupted data
        }
        
        var metadata APITokenMetadata
        if err := json.Unmarshal(metadataData, &metadata); err != nil {
            return fmt.Errorf("failed to unmarshal metadata: %w", err)
        }
        
        // Update last_used
        metadata.LastUsed = time.Now()
        updatedData, err := json.Marshal(metadata)
        if err != nil {
            return fmt.Errorf("failed to marshal updated metadata: %w", err)
        }
        
        if err := tenantBucket.Put([]byte("metadata"), updatedData); err != nil {
            return fmt.Errorf("failed to update metadata: %w", err)
        }
        
        // Update index
        indexBucket := tx.Bucket([]byte("token_index"))
        byHashBucket := indexBucket.Bucket([]byte("by_hash"))
        byHashBucket.Put([]byte(hash), updatedData)
        
        valid = true
        return nil
    })
    
    return valid, hash, err
}

// hashAPIToken creates a SHA-256 hash of the API token
func (d *DB) hashAPIToken(token string) string {
    h := sha256.Sum256([]byte(token))
    return hex.EncodeToString(h[:])
}

// generatePrefix creates a prefix for API token identification
func (d *DB) generatePrefix(token string) string {
    if len(token) >= 8 {
        return token[:8]
    }
    return token
}

// generateSecureToken generates a cryptographically secure random token
func (d *DB) generateSecureToken() (string, error) {
    bytes := make([]byte, 32)
    if _, err := rand.Read(bytes); err != nil {
        return "", fmt.Errorf("failed to generate random bytes: %w", err)
    }
    return hex.EncodeToString(bytes), nil
}
```

### OAuth Token Management

```go
// oauth_tokens.go

// StoreOAuthToken stores an OAuth token for a specific tenant and service
func (d *DB) StoreOAuthToken(tenantHash, serviceName string, tokenData *OAuthTokenData) error {
    tokenData.UpdatedAt = time.Now()
    if tokenData.CreatedAt.IsZero() {
        tokenData.CreatedAt = time.Now()
    }
    
    return d.db.Update(func(tx *bbolt.Tx) error {
        // Navigate to tenant's oauth_tokens bucket
        apiTokensBucket := tx.Bucket([]byte("api_tokens"))
        tenantBucket := apiTokensBucket.Bucket([]byte(tenantHash))
        if tenantBucket == nil {
            return fmt.Errorf("tenant not found: %s", tenantHash)
        }
        
        oauthBucket := tenantBucket.Bucket([]byte("oauth_tokens"))
        if oauthBucket == nil {
            return fmt.Errorf("oauth_tokens bucket not found for tenant: %s", tenantHash)
        }
        
        // Serialize token data
        tokenBytes, err := json.Marshal(tokenData)
        if err != nil {
            return fmt.Errorf("failed to marshal OAuth token: %w", err)
        }
        
        // Store token
        if err := oauthBucket.Put([]byte(serviceName), tokenBytes); err != nil {
            return fmt.Errorf("failed to store OAuth token: %w", err)
        }
        
        d.logger.Debugf("Stored OAuth token for tenant %s, service %s", tenantHash, serviceName)
        return nil
    })
}

// GetOAuthToken retrieves an OAuth token for a specific tenant and service
func (d *DB) GetOAuthToken(tenantHash, serviceName string) (*OAuthTokenData, error) {
    var tokenData *OAuthTokenData
    
    err := d.db.View(func(tx *bbolt.Tx) error {
        // Navigate to tenant's oauth_tokens bucket
        apiTokensBucket := tx.Bucket([]byte("api_tokens"))
        tenantBucket := apiTokensBucket.Bucket([]byte(tenantHash))
        if tenantBucket == nil {
            return fmt.Errorf("tenant not found: %s", tenantHash)
        }
        
        oauthBucket := tenantBucket.Bucket([]byte("oauth_tokens"))
        if oauthBucket == nil {
            return fmt.Errorf("oauth_tokens bucket not found for tenant: %s", tenantHash)
        }
        
        // Get token data
        tokenBytes := oauthBucket.Get([]byte(serviceName))
        if tokenBytes == nil {
            return fmt.Errorf("OAuth token not found for service: %s", serviceName)
        }
        
        // Deserialize token data
        tokenData = &OAuthTokenData{}
        if err := json.Unmarshal(tokenBytes, tokenData); err != nil {
            return fmt.Errorf("failed to unmarshal OAuth token: %w", err)
        }
        
        return nil
    })
    
    return tokenData, err
}

// DeleteOAuthToken deletes an OAuth token for a specific tenant and service
func (d *DB) DeleteOAuthToken(tenantHash, serviceName string) error {
    return d.db.Update(func(tx *bbolt.Tx) error {
        // Navigate to tenant's oauth_tokens bucket
        apiTokensBucket := tx.Bucket([]byte("api_tokens"))
        tenantBucket := apiTokensBucket.Bucket([]byte(tenantHash))
        if tenantBucket == nil {
            return fmt.Errorf("tenant not found: %s", tenantHash)
        }
        
        oauthBucket := tenantBucket.Bucket([]byte("oauth_tokens"))
        if oauthBucket == nil {
            return fmt.Errorf("oauth_tokens bucket not found for tenant: %s", tenantHash)
        }
        
        // Delete token
        if err := oauthBucket.Delete([]byte(serviceName)); err != nil {
            return fmt.Errorf("failed to delete OAuth token: %w", err)
        }
        
        d.logger.Debugf("Deleted OAuth token for tenant %s, service %s", tenantHash, serviceName)
        return nil
    })
}

// ListOAuthTokens lists all OAuth tokens for a specific tenant
func (d *DB) ListOAuthTokens(tenantHash string) (map[string]*OAuthTokenData, error) {
    tokens := make(map[string]*OAuthTokenData)
    
    err := d.db.View(func(tx *bbolt.Tx) error {
        // Navigate to tenant's oauth_tokens bucket
        apiTokensBucket := tx.Bucket([]byte("api_tokens"))
        tenantBucket := apiTokensBucket.Bucket([]byte(tenantHash))
        if tenantBucket == nil {
            return fmt.Errorf("tenant not found: %s", tenantHash)
        }
        
        oauthBucket := tenantBucket.Bucket([]byte("oauth_tokens"))
        if oauthBucket == nil {
            return nil // No OAuth tokens yet
        }
        
        // Iterate through all OAuth tokens
        return oauthBucket.ForEach(func(k, v []byte) error {
            var tokenData OAuthTokenData
            if err := json.Unmarshal(v, &tokenData); err != nil {
                d.logger.Warningf("Failed to unmarshal OAuth token for service %s: %v", string(k), err)
                return nil // Continue iteration
            }
            tokens[string(k)] = &tokenData
            return nil
        })
    })
    
    return tokens, err
}
```

## 3. CLI Implementation

### Token Management Commands

```go
// cmd/token/main.go

package main

import (
    "flag"
    "fmt"
    "os"
    "crypto/rand"
    "encoding/hex"
    "text/tabwriter"
    "github.com/PivotLLM/MCPFusion/db"
    "github.com/PivotLLM/MCPFusion/mlogger"
)

func main() {
    var (
        dataDir    = flag.String("data-dir", "", "Database data directory")
        debug      = flag.Bool("debug", false, "Enable debug logging")
    )
    flag.Parse()
    
    if len(flag.Args()) == 0 {
        printUsage()
        os.Exit(1)
    }
    
    // Initialize logger
    logger, err := mlogger.New(
        mlogger.WithPrefix("TOKEN"),
        mlogger.WithLogStdout(true),
        mlogger.WithDebug(*debug),
    )
    if err != nil {
        fmt.Printf("Failed to create logger: %v\n", err)
        os.Exit(1)
    }
    defer logger.Close()
    
    // Initialize database
    opts := []db.Option{db.WithLogger(logger)}
    if *dataDir != "" {
        opts = append(opts, db.WithDataDir(*dataDir))
    }
    
    database, err := db.New(opts...)
    if err != nil {
        fmt.Printf("Failed to open database: %v\n", err)
        os.Exit(1)
    }
    defer database.Close()
    
    command := flag.Args()[0]
    switch command {
    case "add":
        cmdAdd(database)
    case "list":
        cmdList(database)
    case "delete":
        cmdDelete(database)
    default:
        fmt.Printf("Unknown command: %s\n", command)
        printUsage()
        os.Exit(1)
    }
}

func cmdAdd(database *db.DB) {
    description := ""
    if len(flag.Args()) > 1 {
        description = flag.Args()[1]
    }
    
    token, hash, err := database.AddAPIToken(description)
    if err != nil {
        fmt.Printf("Failed to add token: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Token generated successfully!\n")
    fmt.Printf("Hash: %s\n", hash)
    fmt.Printf("IMPORTANT: Save this token securely. It cannot be retrieved later.\n")
    fmt.Printf("Token: %s\n", token)
}

func cmdList(database *db.DB) {
    tokens, err := database.ListAPITokens()
    if err != nil {
        fmt.Printf("Failed to list tokens: %v\n", err)
        os.Exit(1)
    }
    
    if len(tokens) == 0 {
        fmt.Println("No API tokens found.")
        return
    }
    
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
    fmt.Fprintln(w, "PREFIX\tHASH\tCREATED\tLAST USED\tDESCRIPTION")
    
    for _, token := range tokens {
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
            token.Prefix,
            token.Hash[:16]+"...",
            token.CreatedAt.Format("2006-01-02 15:04:05"),
            token.LastUsed.Format("2006-01-02 15:04:05"),
            token.Description,
        )
    }
    
    w.Flush()
}

func cmdDelete(database *db.DB) {
    if len(flag.Args()) < 2 {
        fmt.Println("Usage: token delete <HASH_OR_PREFIX>")
        os.Exit(1)
    }
    
    identifier := flag.Args()[1]
    
    // Find token by hash or prefix
    tokens, err := database.ListAPITokens()
    if err != nil {
        fmt.Printf("Failed to list tokens: %v\n", err)
        os.Exit(1)
    }
    
    var targetHash string
    for _, token := range tokens {
        if token.Hash == identifier || token.Prefix == identifier {
            targetHash = token.Hash
            break
        }
    }
    
    if targetHash == "" {
        fmt.Printf("Token not found: %s\n", identifier)
        os.Exit(1)
    }
    
    if err := database.DeleteAPIToken(targetHash); err != nil {
        fmt.Printf("Failed to delete token: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Token deleted successfully: %s\n", targetHash)
}


func printUsage() {
    fmt.Println("Usage: token <command> [options]")
    fmt.Println("")
    fmt.Println("Commands:")
    fmt.Println("  add [description]          Generate and add a new API token")
    fmt.Println("  list                       List all API tokens")
    fmt.Println("  delete <HASH_OR_PREFIX>    Delete an API token")
    fmt.Println("")
    fmt.Println("Options:")
    fmt.Println("  -data-dir string          Database data directory")
    fmt.Println("  -debug                    Enable debug logging")
}
```

## 4. Integration with Existing Auth System

### Modified AuthManager

```go
// fusion/auth_multi_tenant.go

// MultiTenantAuthManager extends AuthManager with tenant support
type MultiTenantAuthManager struct {
    *AuthManager
    db         *db.DB
    tenantHash string // Current tenant context
}

// NewMultiTenantAuthManager creates a new multi-tenant auth manager
func NewMultiTenantAuthManager(database *db.DB, logger global.Logger) *MultiTenantAuthManager {
    // Create base auth manager with database-backed cache
    baseManager := NewAuthManager(NewDatabaseCache(database), logger)
    
    return &MultiTenantAuthManager{
        AuthManager: baseManager,
        db:          database,
    }
}

// SetTenant sets the current tenant context
func (am *MultiTenantAuthManager) SetTenant(apiToken string) error {
    valid, hash, err := am.db.ValidateAPIToken(apiToken)
    if err != nil {
        return fmt.Errorf("failed to validate API token: %w", err)
    }
    
    if !valid {
        return fmt.Errorf("invalid API token")
    }
    
    am.tenantHash = hash
    am.logger.Debugf("Set tenant context: %s", hash)
    return nil
}

// GetToken overrides base GetToken to use tenant-specific storage
func (am *MultiTenantAuthManager) GetToken(ctx context.Context, serviceName string, authConfig AuthConfig) (*TokenInfo, error) {
    if am.tenantHash == "" {
        return nil, fmt.Errorf("no tenant context set")
    }
    
    am.logger.Debugf("Getting token for tenant %s, service %s", am.tenantHash, serviceName)
    
    // Check database for existing OAuth token
    if authConfig.Type == AuthTypeOAuth2Device {
        if tokenData, err := am.db.GetOAuthToken(am.tenantHash, serviceName); err == nil {
            // Convert database token to TokenInfo
            tokenInfo := &TokenInfo{
                AccessToken:  tokenData.AccessToken,
                RefreshToken: tokenData.RefreshToken,
                TokenType:    tokenData.TokenType,
                ExpiresAt:    tokenData.ExpiresAt,
                Scope:        tokenData.Scope,
            }
            
            // Check if token is still valid
            if !tokenInfo.IsExpiredWithBuffer(5 * time.Minute) {
                am.logger.Debugf("Using valid tenant-specific token for service %s", serviceName)
                return tokenInfo, nil
            }
            
            // Try to refresh if possible
            if tokenInfo.HasRefreshToken() {
                if strategy, exists := am.strategies[authConfig.Type]; exists && strategy.SupportsRefresh() {
                    if refreshedToken, err := strategy.RefreshToken(ctx, tokenInfo); err == nil {
                        // Store refreshed token
                        refreshedData := &db.OAuthTokenData{
                            AccessToken:  refreshedToken.AccessToken,
                            RefreshToken: refreshedToken.RefreshToken,
                            TokenType:    refreshedToken.TokenType,
                            ExpiresAt:    refreshedToken.ExpiresAt,
                            Scope:        refreshedToken.Scope,
                        }
                        
                        if err := am.db.StoreOAuthToken(am.tenantHash, serviceName, refreshedData); err != nil {
                            am.logger.Warningf("Failed to store refreshed token: %v", err)
                        } else {
                            am.logger.Infof("Successfully refreshed and stored token for service: %s", serviceName)
                        }
                        
                        return refreshedToken, nil
                    }
                }
            }
        }
    }
    
    // Fall back to new authentication
    return am.performNewAuthentication(ctx, serviceName, authConfig)
}

// performNewAuthentication handles new authentication and stores the result
func (am *MultiTenantAuthManager) performNewAuthentication(ctx context.Context, serviceName string, authConfig AuthConfig) (*TokenInfo, error) {
    strategy, exists := am.strategies[authConfig.Type]
    if !exists {
        return nil, NewAuthenticationError(authConfig.Type, serviceName, "unsupported authentication type", nil)
    }
    
    am.logger.Infof("Performing new authentication for tenant %s, service %s", am.tenantHash, serviceName)
    
    tokenInfo, err := strategy.Authenticate(ctx, authConfig.Config)
    if err != nil {
        return nil, err
    }
    
    // Store OAuth tokens in database
    if authConfig.Type == AuthTypeOAuth2Device {
        tokenData := &db.OAuthTokenData{
            AccessToken:  tokenInfo.AccessToken,
            RefreshToken: tokenInfo.RefreshToken,
            TokenType:    tokenInfo.TokenType,
            ExpiresAt:    tokenInfo.ExpiresAt,
            Scope:        tokenInfo.Scope,
        }
        
        if err := am.db.StoreOAuthToken(am.tenantHash, serviceName, tokenData); err != nil {
            am.logger.Warningf("Failed to store OAuth token for service %s: %v", serviceName, err)
        } else {
            am.logger.Infof("Successfully stored OAuth token for service: %s", serviceName)
        }
    }
    
    return tokenInfo, nil
}

// InvalidateToken invalidates a token for the current tenant
func (am *MultiTenantAuthManager) InvalidateToken(serviceName string) {
    if am.tenantHash == "" {
        am.logger.Warning("No tenant context set for token invalidation")
        return
    }
    
    if err := am.db.DeleteOAuthToken(am.tenantHash, serviceName); err != nil {
        am.logger.Warningf("Failed to delete OAuth token: %v", err)
    } else {
        am.logger.Infof("Invalidated token for tenant %s, service %s", am.tenantHash, serviceName)
    }
}

// DatabaseCache implements Cache interface using the database
type DatabaseCache struct {
    db *db.DB
}

func NewDatabaseCache(database *db.DB) *DatabaseCache {
    return &DatabaseCache{db: database}
}

func (c *DatabaseCache) Get(key string) (interface{}, error) {
    // For non-OAuth tokens, use database storage
    // This would need to be implemented based on specific needs
    return nil, fmt.Errorf("key not found")
}

func (c *DatabaseCache) Set(key string, value interface{}, ttl time.Duration) error {
    // For non-OAuth tokens, store in database
    // This would need to be implemented based on specific needs
    return nil
}

func (c *DatabaseCache) Delete(key string) error {
    // Delete from database
    return nil
}

func (c *DatabaseCache) Clear() error {
    // Clear cache data
    return nil
}

func (c *DatabaseCache) Has(key string) bool {
    _, err := c.Get(key)
    return err == nil
}
```

## 5. Main Application Integration

### Modified main.go

```go
// Updated main.go to support API token validation

func main() {
    // ... existing flag parsing ...
    
    // Initialize database
    database, err := db.New(
        db.WithLogger(tempLogger),
    )
    if err != nil {
        tempLogger.Fatalf("Failed to initialize database: %v", err)
    }
    defer database.Close()
    
    // API token validation middleware
    APIAuthKey := os.Getenv("API_AUTH_KEY")
    if APIAuthKey != "" {
        // Validate the API token exists in database
        valid, hash, err := database.ValidateAPIToken(APIAuthKey)
        if err != nil {
            tempLogger.Fatalf("Failed to validate API token: %v", err)
        }
        if !valid {
            tempLogger.Fatalf("Invalid API token provided")
        }
        tempLogger.Infof("Validated API token: %s", hash[:16]+"...")
    } else {
        tempLogger.Warning("No API token provided - running in legacy mode")
    }
    
    // ... rest of existing main.go logic ...
    
    // Create fusion provider with multi-tenant support
    if fusionConfig != "" {
        logger.Infof("Loading fusion provider with config file: %s", fusionConfig)
        fusionProvider = fusion.NewWithDatabase(
            fusion.WithLogger(logger),
            fusion.WithJSONConfig(fusionConfig),
            fusion.WithDatabase(database),
        )
        providers = append(providers, fusionProvider)
    }
    
    // ... rest of existing logic ...
}
```

### HTTP Middleware for Bearer Token Validation

```go
// mcpserver/auth_middleware.go

func (s *MCPServer) validateBearerToken(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if s.database == nil {
            // Legacy mode - use existing validation
            next(w, r)
            return
        }
        
        // Extract API token from Authorization header
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Authorization header required", http.StatusUnauthorized)
            return
        }
        
        // Parse Bearer token
        const bearerPrefix = "Bearer "
        if !strings.HasPrefix(authHeader, bearerPrefix) {
            http.Error(w, "Authorization must be Bearer token", http.StatusUnauthorized)
            return
        }
        
        apiToken := strings.TrimPrefix(authHeader, bearerPrefix)
        if apiToken == "" {
            http.Error(w, "Bearer token is empty", http.StatusUnauthorized)
            return
        }
        
        // Validate token
        valid, hash, err := s.database.ValidateAPIToken(apiToken)
        if err != nil {
            s.logger.Errorf("Token validation error: %v", err)
            http.Error(w, "Internal server error", http.StatusInternalServerError)
            return
        }
        
        if !valid {
            http.Error(w, "Invalid API token", http.StatusUnauthorized)
            return
        }
        
        // Set tenant context in request
        ctx := context.WithValue(r.Context(), "tenant_hash", hash)
        next(w, r.WithContext(ctx))
    }
}
```

## 6. Testing Strategy

### Test Structure

```go
// db/db_test.go

func TestAPITokenManagement(t *testing.T) {
    // Test token addition, validation, and deletion
    tempDir, _ := ioutil.TempDir("", "mcpfusion_test")
    defer os.RemoveAll(tempDir)
    
    logger := &testLogger{}
    db, err := New(WithLogger(logger), WithDataDir(tempDir))
    assert.NoError(t, err)
    defer db.Close()
    
    // Test adding token
    token, hash, err := db.AddAPIToken("Test token")
    assert.NoError(t, err)
    assert.NotEmpty(t, token)
    assert.NotEmpty(t, hash)
    
    // Test validation
    valid, returnedHash, err := db.ValidateAPIToken(token)
    assert.NoError(t, err)
    assert.True(t, valid)
    assert.Equal(t, hash, returnedHash)
    
    // Test invalid token
    valid, _, err = db.ValidateAPIToken("invalid-token")
    assert.NoError(t, err)
    assert.False(t, valid)
    
    // Test deletion
    err = db.DeleteAPIToken(hash)
    assert.NoError(t, err)
    
    // Verify deletion
    valid, _, err = db.ValidateAPIToken(token)
    assert.NoError(t, err)
    assert.False(t, valid)
}

func TestOAuthTokenManagement(t *testing.T) {
    // Test OAuth token storage and retrieval
    tempDir, _ := ioutil.TempDir("", "mcpfusion_test")
    defer os.RemoveAll(tempDir)
    
    logger := &testLogger{}
    db, err := New(WithLogger(logger), WithDataDir(tempDir))
    assert.NoError(t, err)
    defer db.Close()
    
    // Create tenant
    tenantToken, tenantHash, err := db.AddAPIToken("Test tenant")
    assert.NoError(t, err)
    
    // Store OAuth token
    tokenData := &OAuthTokenData{
        AccessToken:  "access-token-123",
        RefreshToken: "refresh-token-123",
        TokenType:    "Bearer",
        ExpiresAt:    &time.Time{},
        Scope:        []string{"read", "write"},
    }
    
    err = db.StoreOAuthToken(tenantHash, "microsoft365", tokenData)
    assert.NoError(t, err)
    
    // Retrieve OAuth token
    retrieved, err := db.GetOAuthToken(tenantHash, "microsoft365")
    assert.NoError(t, err)
    assert.Equal(t, tokenData.AccessToken, retrieved.AccessToken)
    assert.Equal(t, tokenData.RefreshToken, retrieved.RefreshToken)
    
    // Test cascade deletion
    err = db.DeleteAPIToken(tenantHash)
    assert.NoError(t, err)
    
    // Verify OAuth token is also deleted
    _, err = db.GetOAuthToken(tenantHash, "microsoft365")
    assert.Error(t, err)
}

func TestConcurrentAccess(t *testing.T) {
    // Test concurrent token operations
    tempDir, _ := ioutil.TempDir("", "mcpfusion_test")
    defer os.RemoveAll(tempDir)
    
    logger := &testLogger{}
    db, err := New(WithLogger(logger), WithDataDir(tempDir))
    assert.NoError(t, err)
    defer db.Close()
    
    var wg sync.WaitGroup
    numGoroutines := 10
    
    // Concurrent token additions
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            _, _, err := db.AddAPIToken(fmt.Sprintf("Token %d", id))
            assert.NoError(t, err)
        }(i)
    }
    
    wg.Wait()
    
    // Verify all tokens were added
    tokens, err := db.ListAPITokens()
    assert.NoError(t, err)
    assert.Len(t, tokens, numGoroutines)
}

type testLogger struct{}

func (l *testLogger) Debug(msg string)               {}
func (l *testLogger) Info(msg string)                {}
func (l *testLogger) Notice(msg string)              {}
func (l *testLogger) Warning(msg string)             {}
func (l *testLogger) Error(msg string)               {}
func (l *testLogger) Fatal(msg string)               {}
func (l *testLogger) Debugf(format string, v ...any) {}
func (l *testLogger) Infof(format string, v ...any)  {}
func (l *testLogger) Noticef(format string, v ...any) {}
func (l *testLogger) Warningf(format string, v ...any) {}
func (l *testLogger) Errorf(format string, v ...any)  {}
func (l *testLogger) Fatalf(format string, v ...any)  {}
func (l *testLogger) Close()                          {}
```

## 8. Performance Considerations

### Database Optimization

1. **Indexing Strategy**: Use separate index buckets for efficient lookups
2. **Connection Pooling**: Single database connection with proper locking
3. **Batch Operations**: Group related operations in single transactions
4. **Cache Layer**: In-memory cache for frequently accessed tokens

### Memory Management

1. **Resource Cleanup**: Proper database connection closing
2. **Goroutine Safety**: Thread-safe operations with appropriate locking
3. **Buffer Reuse**: Efficient JSON marshaling/unmarshaling

## 9. Security Considerations

### Token Security

1. **Hashing**: SHA-256 hashing of API tokens (never store plaintext)
2. **Auto-Generation**: Cryptographically secure random token generation
3. **Access Control**: Database file permissions (0600)
4. **Audit Trail**: Logging of all token operations

### Data Protection

1. **Encryption at Rest**: BoltDB file-level encryption (optional)
2. **Secure Deletion**: Proper cleanup of sensitive data
3. **Access Logging**: Track all authentication attempts

## 10. Deployment Considerations

### System Integration

1. **Service Discovery**: Automatic detection of available services
2. **Health Checks**: Database connectivity monitoring
3. **Backup Strategy**: Regular database backups
4. **Monitoring**: Metrics collection for performance tracking

### Operational Procedures

1. **Token Rotation**: Regular API token rotation procedures
2. **Emergency Procedures**: Token revocation and system recovery
3. **Capacity Planning**: Database size monitoring and management

## 11. Implementation Timeline

### Phase 1: Core Database Layer
- [ ] Implement core database package with BoltDB (`go.etcd.io/bbolt`)
- [ ] Create API token management functions with auto-generation
- [ ] Implement OAuth token storage/retrieval
- [ ] Add comprehensive test suite

### Phase 2: CLI Tools
- [ ] Build token management CLI with auto-generation
- [ ] Implement token validation and listing
- [ ] Add delete operations
- [ ] Create user documentation

### Phase 3: Integration
- [ ] Integrate database with existing auth system
- [ ] Modify Fusion provider for multi-tenant support
- [ ] Update HTTP middleware for Bearer token validation
- [ ] Remove file cache dependencies

### Phase 4: Testing and Documentation
- [ ] Complete integration testing
- [ ] Performance testing and optimization
- [ ] Update documentation
- [ ] Production deployment preparation

## 12. Conclusion

This implementation plan provides a comprehensive approach to adding multi-tenant token management to MCPFusion using BoltDB. The design follows proven patterns from PivotChat while addressing the specific needs of MCPFusion's multi-service API integration architecture.

Key benefits of this approach:

1. **Scalability**: Efficient storage and retrieval of tokens for multiple tenants
2. **Security**: Auto-generated tokens with proper hashing and secure storage
3. **Reliability**: Proven BoltDB (`go.etcd.io/bbolt`) technology with ACID transactions
4. **Maintainability**: Clean separation of concerns and comprehensive testing
5. **Operational Excellence**: Complete CLI tools and monitoring capabilities

The phased implementation approach ensures a clean migration to a robust multi-tenant architecture without legacy compatibility concerns.