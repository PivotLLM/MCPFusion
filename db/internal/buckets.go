/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package internal

// Bucket names for consistent database organization
//
//goland:noinspection GoCommentStart
const (
	// Root buckets
	BucketAPITokens  = "api_tokens"
	BucketTenants    = "tenants"
	BucketTokenIndex = "token_index"
	BucketSystem     = "system"

	// Sub-buckets under tenants/{tenant_hash}/
	BucketOAuthTokens        = "oauth_tokens"
	BucketServiceCredentials = "service_credentials"

	// Sub-buckets under token_index/
	BucketIndexByHash   = "by_hash"
	BucketIndexByPrefix = "by_prefix"

	// System keys
	KeySchemaVersion = "schema_version"
	KeyStats         = "stats"
	KeyMetadata      = "metadata"
)

// SchemaVersion Current schema version
const SchemaVersion = "1.0"

// BucketPath represents a path to a bucket in the database
type BucketPath []string

// NewBucketPath creates a new bucket path from string segments
func NewBucketPath(segments ...string) BucketPath {
	return segments
}

// String returns the bucket path as a slash-separated string
func (bp BucketPath) String() string {
	result := ""
	for i, segment := range bp {
		if i > 0 {
			result += "/"
		}
		result += segment
	}
	return result
}

// Append adds segments to the bucket path
func (bp BucketPath) Append(segments ...string) BucketPath {
	newPath := make(BucketPath, len(bp)+len(segments))
	copy(newPath, bp)
	copy(newPath[len(bp):], segments)
	return newPath
}

// Predefined bucket paths for common operations
//
//goland:noinspection GoCommentStart
var (
	// Root bucket paths
	PathAPITokens  = NewBucketPath(BucketAPITokens)
	PathTenants    = NewBucketPath(BucketTenants)
	PathTokenIndex = NewBucketPath(BucketTokenIndex)
	PathSystem     = NewBucketPath(BucketSystem)

	// Index paths
	PathIndexByHash   = PathTokenIndex.Append(BucketIndexByHash)
	PathIndexByPrefix = PathTokenIndex.Append(BucketIndexByPrefix)
)

// GetTenantPath returns the bucket path for a specific tenant
func GetTenantPath(tenantHash string) BucketPath {
	return PathTenants.Append(tenantHash)
}

// GetTenantOAuthPath returns the OAuth tokens bucket path for a tenant
//
//goland:noinspection GoUnusedExportedFunction
func GetTenantOAuthPath(tenantHash string) BucketPath {
	return GetTenantPath(tenantHash).Append(BucketOAuthTokens)
}

// GetTenantCredentialsPath returns the credentials bucket path for a tenant
func GetTenantCredentialsPath(tenantHash string) BucketPath {
	return GetTenantPath(tenantHash).Append(BucketServiceCredentials)
}

// GetCredentialTypePath returns the path for a specific credential type
//
//goland:noinspection GoUnusedExportedFunction
func GetCredentialTypePath(tenantHash string, credType string) BucketPath {
	return GetTenantCredentialsPath(tenantHash).Append(credType)
}
