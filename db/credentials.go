/*=============================================================================
= Copyright (c) 2025 Tenebris Technologies Inc.                              =
= All rights reserved.                                                       =
=============================================================================*/

package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"go.etcd.io/bbolt"
)

// StoreCredentials stores service credentials for a tenant and service
func (d *DB) StoreCredentials(tenantHash, serviceName string, credentials *ServiceCredentials) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return NewValidationError("service_name", serviceName, err.Error())
	}

	if credentials == nil {
		return NewValidationError("credentials", nil, "credentials cannot be nil")
	}

	if credentials.Type == "" {
		return NewValidationError("credential_type", "", "credential type cannot be empty")
	}

	if credentials.Data == nil || len(credentials.Data) == 0 {
		return NewValidationError("credential_data", nil, "credential data cannot be empty")
	}

	// Validate credential type
	validTypes := map[CredentialType]bool{
		CredentialTypeAPIKey:    true,
		CredentialTypeBearer:    true,
		CredentialTypeBasicAuth: true,
		CredentialTypeCustom:    true,
	}

	if !validTypes[credentials.Type] {
		return NewValidationError("credential_type", credentials.Type, "invalid credential type")
	}

	// Set timestamps
	now := time.Now()
	credentials.UpdatedAt = now
	if credentials.CreatedAt.IsZero() {
		credentials.CreatedAt = now
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get or create tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("store_credentials", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket, err := tenantsBucket.CreateBucketIfNotExists([]byte(tenantHash))
		if err != nil {
			return NewDatabaseErrorWithContext("store_credentials", fmt.Errorf("failed to create tenant bucket: %w", err), tenantHash, serviceName)
		}

		// Get or create service credentials bucket for this tenant
		credentialsBucket, err := tenantBucket.CreateBucketIfNotExists([]byte(internal.BucketServiceCredentials))
		if err != nil {
			return NewDatabaseErrorWithContext("store_credentials", fmt.Errorf("failed to create credentials bucket: %w", err), tenantHash, serviceName)
		}

		// Marshal credentials data
		credentialsBytes, err := json.Marshal(credentials)
		if err != nil {
			return NewDatabaseErrorWithContext("store_credentials", fmt.Errorf("failed to marshal credentials: %w", err), tenantHash, serviceName)
		}

		// Store credentials data
		if err := credentialsBucket.Put([]byte(serviceName), credentialsBytes); err != nil {
			return NewDatabaseErrorWithContext("store_credentials", fmt.Errorf("failed to store credentials: %w", err), tenantHash, serviceName)
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Stored %s credentials for tenant %s service %s", credentials.Type, tenantHash[:12]+"...", serviceName)
	return nil
}

// GetCredentials retrieves service credentials for a tenant and service
func (d *DB) GetCredentials(tenantHash, serviceName string) (*ServiceCredentials, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return nil, NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return nil, NewValidationError("service_name", serviceName, err.Error())
	}

	var credentials *ServiceCredentials

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("get_credentials", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			return NewDatabaseErrorWithContext("get_credentials", ErrTenantNotFound, tenantHash, serviceName)
		}

		// Get service credentials bucket
		credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
		if credentialsBucket == nil {
			return NewDatabaseErrorWithContext("get_credentials", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Get credentials data
		credentialsBytes := credentialsBucket.Get([]byte(serviceName))
		if credentialsBytes == nil {
			return NewDatabaseErrorWithContext("get_credentials", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Unmarshal credentials data
		credentials = &ServiceCredentials{}
		if err := json.Unmarshal(credentialsBytes, credentials); err != nil {
			return NewDatabaseErrorWithContext("get_credentials", fmt.Errorf("failed to unmarshal credentials: %w", err), tenantHash, serviceName)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Retrieved %s credentials for tenant %s service %s", credentials.Type, tenantHash[:12]+"...", serviceName)
	return credentials, nil
}

// DeleteCredentials removes service credentials for a tenant and service
func (d *DB) DeleteCredentials(tenantHash, serviceName string) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return NewValidationError("service_name", serviceName, err.Error())
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("delete_credentials", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			return NewDatabaseErrorWithContext("delete_credentials", ErrTenantNotFound, tenantHash, serviceName)
		}

		// Get service credentials bucket
		credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
		if credentialsBucket == nil {
			return NewDatabaseErrorWithContext("delete_credentials", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Check if credentials exist
		if credentialsBucket.Get([]byte(serviceName)) == nil {
			return NewDatabaseErrorWithContext("delete_credentials", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Delete credentials
		if err := credentialsBucket.Delete([]byte(serviceName)); err != nil {
			return NewDatabaseErrorWithContext("delete_credentials", fmt.Errorf("failed to delete credentials: %w", err), tenantHash, serviceName)
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Deleted credentials for tenant %s service %s", tenantHash[:12]+"...", serviceName)
	return nil
}

// ListCredentials returns all service credentials for a tenant
func (d *DB) ListCredentials(tenantHash string) (map[string]*ServiceCredentials, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate input
	if err := internal.ValidateHash(tenantHash); err != nil {
		return nil, NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	credentials := make(map[string]*ServiceCredentials)

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("list_credentials", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			// No tenant bucket means no credentials - return empty map
			return nil
		}

		// Get service credentials bucket
		credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
		if credentialsBucket == nil {
			// No credentials bucket means no credentials - return empty map
			return nil
		}

		// Iterate through all service credentials
		return credentialsBucket.ForEach(func(k, v []byte) error {
			serviceName := string(k)
			
			var serviceCredentials ServiceCredentials
			if err := json.Unmarshal(v, &serviceCredentials); err != nil {
				d.logger.Warningf("Failed to unmarshal credentials for service %s: %v", serviceName, err)
				return nil // Continue iteration
			}

			credentials[serviceName] = &serviceCredentials
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	d.logger.Debugf("Listed %d service credentials for tenant %s", len(credentials), tenantHash[:12]+"...")
	return credentials, nil
}

// UpdateCredentials updates existing service credentials for a tenant and service
// This is a convenience method that preserves the original creation timestamp
func (d *DB) UpdateCredentials(tenantHash, serviceName string, credentials *ServiceCredentials) error {
	if err := d.checkClosed(); err != nil {
		return err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return NewValidationError("service_name", serviceName, err.Error())
	}

	if credentials == nil {
		return NewValidationError("credentials", nil, "credentials cannot be nil")
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("update_credentials", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			return NewDatabaseErrorWithContext("update_credentials", ErrTenantNotFound, tenantHash, serviceName)
		}

		// Get service credentials bucket
		credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
		if credentialsBucket == nil {
			return NewDatabaseErrorWithContext("update_credentials", ErrServiceNotFound, tenantHash, serviceName)
		}

		// Get existing credentials to preserve creation time
		existingBytes := credentialsBucket.Get([]byte(serviceName))
		if existingBytes == nil {
			return NewDatabaseErrorWithContext("update_credentials", ErrServiceNotFound, tenantHash, serviceName)
		}

		var existing ServiceCredentials
		if err := json.Unmarshal(existingBytes, &existing); err != nil {
			return NewDatabaseErrorWithContext("update_credentials", fmt.Errorf("failed to unmarshal existing credentials: %w", err), tenantHash, serviceName)
		}

		// Update credentials while preserving creation time
		credentials.CreatedAt = existing.CreatedAt
		credentials.UpdatedAt = time.Now()

		// Marshal and store updated credentials
		updatedBytes, err := json.Marshal(credentials)
		if err != nil {
			return NewDatabaseErrorWithContext("update_credentials", fmt.Errorf("failed to marshal updated credentials: %w", err), tenantHash, serviceName)
		}

		if err := credentialsBucket.Put([]byte(serviceName), updatedBytes); err != nil {
			return NewDatabaseErrorWithContext("update_credentials", fmt.Errorf("failed to store updated credentials: %w", err), tenantHash, serviceName)
		}

		return nil
	})

	if err != nil {
		return err
	}

	d.logger.Infof("Updated %s credentials for tenant %s service %s", credentials.Type, tenantHash[:12]+"...", serviceName)
	return nil
}

// GetCredentialsByType returns all credentials of a specific type for a tenant
func (d *DB) GetCredentialsByType(tenantHash string, credType CredentialType) (map[string]*ServiceCredentials, error) {
	if err := d.checkClosed(); err != nil {
		return nil, err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return nil, NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	// Get all credentials first
	allCredentials, err := d.ListCredentials(tenantHash)
	if err != nil {
		return nil, err
	}

	// Filter by type
	filteredCredentials := make(map[string]*ServiceCredentials)
	for serviceName, creds := range allCredentials {
		if creds.Type == credType {
			filteredCredentials[serviceName] = creds
		}
	}

	d.logger.Debugf("Found %d %s credentials for tenant %s", len(filteredCredentials), credType, tenantHash[:12]+"...")
	return filteredCredentials, nil
}

// HasCredentials checks if a tenant has credentials for a specific service
func (d *DB) HasCredentials(tenantHash, serviceName string) (bool, error) {
	if err := d.checkClosed(); err != nil {
		return false, err
	}

	// Validate inputs
	if err := internal.ValidateHash(tenantHash); err != nil {
		return false, NewValidationError("tenant_hash", tenantHash, err.Error())
	}

	if err := internal.ValidateServiceName(serviceName); err != nil {
		return false, NewValidationError("service_name", serviceName, err.Error())
	}

	var hasCredentials bool

	err := d.db.View(func(tx *bbolt.Tx) error {
		// Get tenant bucket
		tenantsBucket := tx.Bucket([]byte(internal.BucketTenants))
		if tenantsBucket == nil {
			return NewDatabaseError("has_credentials", fmt.Errorf("tenants bucket not found"))
		}

		tenantBucket := tenantsBucket.Bucket([]byte(tenantHash))
		if tenantBucket == nil {
			hasCredentials = false
			return nil
		}

		// Get service credentials bucket
		credentialsBucket := tenantBucket.Bucket([]byte(internal.BucketServiceCredentials))
		if credentialsBucket == nil {
			hasCredentials = false
			return nil
		}

		// Check if credentials exist
		hasCredentials = credentialsBucket.Get([]byte(serviceName)) != nil
		return nil
	})

	if err != nil {
		return false, err
	}

	return hasCredentials, nil
}