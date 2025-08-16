/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package db

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"github.com/PivotLLM/MCPFusion/global"
	"go.etcd.io/bbolt"
)

// Database interface defines the contract for the MCPFusion database
type Database interface {
	// API Token Management
	AddAPIToken(description string) (string, string, error)
	ValidateAPIToken(token string) (bool, string, error)
	DeleteAPIToken(hash string) error
	ListAPITokens() ([]APITokenMetadata, error)
	GetAPITokenMetadata(hash string) (*APITokenMetadata, error)
	ResolveAPIToken(identifier string) (string, error)
	
	// OAuth Token Management
	StoreOAuthToken(tenantHash, serviceName string, tokenData *OAuthTokenData) error
	GetOAuthToken(tenantHash, serviceName string) (*OAuthTokenData, error)
	DeleteOAuthToken(tenantHash, serviceName string) error
	ListOAuthTokens(tenantHash string) (map[string]*OAuthTokenData, error)
	
	// Service Credentials Management
	StoreCredentials(tenantHash, serviceName string, credentials *ServiceCredentials) error
	GetCredentials(tenantHash, serviceName string) (*ServiceCredentials, error)
	DeleteCredentials(tenantHash, serviceName string) error
	ListCredentials(tenantHash string) (map[string]*ServiceCredentials, error)
	
	// Tenant Management
	GetTenantInfo(hash string) (*TenantInfo, error)
	ListTenants() ([]TenantInfo, error)
	
	// Statistics and Health
	GetStats() (*TokenStats, error)
	
	// Database Management
	Close() error
	Backup(path string) error
}

// DB implements the Database interface using BoltDB
type DB struct {
	db      *bbolt.DB
	logger  global.Logger
	dataDir string
	mutex   sync.RWMutex
	closed  bool
}

// Config holds configuration options for the database
type Config struct {
	DataDir string
	Logger  global.Logger
}

// Option defines a configuration option for the database
type Option func(*Config)

// WithDataDir sets the data directory for the database
func WithDataDir(dataDir string) Option {
	return func(c *Config) {
		c.DataDir = dataDir
	}
}

// WithLogger sets the logger for the database
func WithLogger(logger global.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// New creates a new database instance with functional options
func New(opts ...Option) (Database, error) {
	config := &Config{}
	
	// Apply options
	for _, opt := range opts {
		opt(config)
	}
	
	// Validate required options
	if config.Logger == nil {
		return nil, NewValidationError("logger", nil, "logger is required")
	}
	
	d := &DB{
		logger: config.Logger,
	}
	
	// Determine data directory
	if config.DataDir == "" {
		config.DataDir = d.determineDataDirectory()
	}
	d.dataDir = config.DataDir
	
	// Ensure data directory exists
	if err := os.MkdirAll(d.dataDir, 0755); err != nil {
		return nil, NewDatabaseError("create_data_dir", 
			fmt.Errorf("failed to create data directory %s: %w", d.dataDir, err))
	}
	
	// Open database
	if err := d.openDatabase(); err != nil {
		return nil, err
	}
	
	// Initialize schema
	if err := d.initializeSchema(); err != nil {
		d.db.Close()
		return nil, NewDatabaseError("init_schema", err)
	}
	
	d.logger.Infof("Database initialized at %s", filepath.Join(d.dataDir, "mcpfusion.db"))
	return d, nil
}

// openDatabase opens the BoltDB database file
func (d *DB) openDatabase() error {
	dbPath := filepath.Join(d.dataDir, "mcpfusion.db")
	options := &bbolt.Options{
		Timeout: 5 * time.Second,
	}
	
	db, err := bbolt.Open(dbPath, 0600, options)
	if err != nil {
		return NewDatabaseError("open_db", 
			fmt.Errorf("failed to open database at %s: %w", dbPath, err))
	}
	
	d.db = db
	return nil
}

// determineDataDirectory determines the best data directory to use
func (d *DB) determineDataDirectory() string {
	// Try /opt/mcpfusion first (system-wide)
	systemDir := "/opt/mcpfusion"
	if d.isDirectoryWritable(systemDir) {
		d.logger.Debugf("Using system data directory: %s", systemDir)
		return systemDir
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

// isDirectoryWritable checks if a directory is writable
func (d *DB) isDirectoryWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	
	testFile := filepath.Join(dir, ".test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return false
	}
	
	os.Remove(testFile)
	return true
}

// initializeSchema creates the initial bucket structure
func (d *DB) initializeSchema() error {
	return d.db.Update(func(tx *bbolt.Tx) error {
		// Create root buckets
		rootBuckets := []string{
			internal.BucketAPITokens,
			internal.BucketTenants,
			internal.BucketTokenIndex,
			internal.BucketSystem,
		}
		
		for _, bucketName := range rootBuckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
			}
		}
		
		// Create index sub-buckets
		indexBucket := tx.Bucket([]byte(internal.BucketTokenIndex))
		indexSubBuckets := []string{
			internal.BucketIndexByHash,
			internal.BucketIndexByPrefix,
		}
		for _, subBucket := range indexSubBuckets {
			if _, err := indexBucket.CreateBucketIfNotExists([]byte(subBucket)); err != nil {
				return fmt.Errorf("failed to create index sub-bucket %s: %w", subBucket, err)
			}
		}
		
		// Set schema version
		systemBucket := tx.Bucket([]byte(internal.BucketSystem))
		if err := systemBucket.Put([]byte(internal.KeySchemaVersion), []byte(internal.SchemaVersion)); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
		
		return nil
	})
}

// Close closes the database connection
func (d *DB) Close() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if d.closed {
		return nil
	}
	
	if d.db == nil {
		d.closed = true
		return nil
	}
	
	d.logger.Info("Closing database connection")
	err := d.db.Close()
	d.closed = true
	
	if err != nil {
		return NewDatabaseError("close_db", err)
	}
	
	d.logger.Info("Database connection closed successfully")
	return nil
}

// Backup creates a backup of the database to the specified path
func (d *DB) Backup(path string) error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	if d.closed {
		return ErrDatabaseClosed
	}
	
	// Ensure backup directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return NewDatabaseError("create_backup_dir", err)
	}
	
	return d.db.View(func(tx *bbolt.Tx) error {
		return tx.CopyFile(path, 0600)
	})
}

// hashAPIToken creates a SHA-256 hash of the API token
func (d *DB) hashAPIToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// generatePrefix creates a prefix for API token identification
func (d *DB) generatePrefix(token string) string {
	if len(token) >= internal.PrefixLength {
		return token[:internal.PrefixLength]
	}
	return token
}

// generateSecureToken generates a cryptographically secure random token
func (d *DB) generateSecureToken() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", NewDatabaseError("generate_token", 
			fmt.Errorf("failed to generate random bytes: %w", err))
	}
	return hex.EncodeToString(bytes), nil
}

// checkClosed verifies the database is not closed
func (d *DB) checkClosed() error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	if d.closed {
		return ErrDatabaseClosed
	}
	return nil
}