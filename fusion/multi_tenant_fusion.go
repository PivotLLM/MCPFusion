/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
)

// MultiTenantFusion wraps the regular Fusion to provide multi-tenant capabilities
type MultiTenantFusion struct {
	authManager     *MultiTenantAuthManager
	serviceResolver *ServiceConfigResolver
	databaseCache   *DatabaseCache
	httpClient      *http.Client
	logger          global.Logger
	db              *db.DB

	// Fusion instances per tenant (for caching configurations)
	tenantFusions map[string]*Fusion
	mu            sync.RWMutex

	// Default configuration
	defaultConfig *Config
}

// MultiTenantFusionOption represents configuration options for MultiTenantFusion
type MultiTenantFusionOption func(*MultiTenantFusion)

// WithMTDatabase sets the database for multi-tenant storage
func WithMTDatabase(database *db.DB) MultiTenantFusionOption {
	return func(mtf *MultiTenantFusion) {
		mtf.db = database
	}
}

// WithMTHTTPClient sets the HTTP client for all fusion instances
func WithMTHTTPClient(client *http.Client) MultiTenantFusionOption {
	return func(mtf *MultiTenantFusion) {
		mtf.httpClient = client
	}
}

// WithMTLogger sets the logger for multi-tenant fusion
func WithMTLogger(logger global.Logger) MultiTenantFusionOption {
	return func(mtf *MultiTenantFusion) {
		mtf.logger = logger
	}
}

// WithMTDefaultConfig sets a default configuration that will be used as a fallback
func WithMTDefaultConfig(config *Config) MultiTenantFusionOption {
	return func(mtf *MultiTenantFusion) {
		mtf.defaultConfig = config
	}
}

// NewMultiTenantFusion creates a new multi-tenant fusion instance
func NewMultiTenantFusion(options ...MultiTenantFusionOption) (*MultiTenantFusion, error) {
	mtf := &MultiTenantFusion{
		tenantFusions: make(map[string]*Fusion),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}

	// Apply options
	for _, option := range options {
		option(mtf)
	}

	// Initialize database cache if database is available
	if mtf.db != nil {
		mtf.databaseCache = NewDatabaseCache(mtf.db, mtf.logger)
	}

	// Initialize service resolver
	mtf.serviceResolver = NewServiceConfigResolver(
		WithSRLogger(mtf.logger),
		WithAutoReload(5*time.Minute), // Auto-reload configs every 5 minutes
	)

	// Load all available services
	if err := mtf.serviceResolver.LoadAllServices(); err != nil {
		if mtf.logger != nil {
			mtf.logger.Warningf("Failed to load all services during initialization: %v", err)
		}
	}

	// Initialize auth manager
	mtf.authManager = NewMultiTenantAuthManager(mtf.db, mtf.databaseCache, mtf.logger)

	// Register default authentication strategies
	mtf.registerDefaultAuthStrategies()

	if mtf.logger != nil {
		mtf.logger.Info("Initialized multi-tenant fusion")
		availableServices := mtf.serviceResolver.GetAvailableServices()
		mtf.logger.Infof("Available services: %v", availableServices)
	}

	return mtf, nil
}

// GetFusionForTenant returns a Fusion instance configured for a specific tenant and service
func (mtf *MultiTenantFusion) GetFusionForTenant(tenantContext *TenantContext) (*Fusion, error) {
	if tenantContext == nil {
		return nil, fmt.Errorf("tenant context is required")
	}

	if mtf.logger != nil {
		mtf.logger.Debugf("Getting fusion instance for tenant %s service %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	// Create a cache key for this tenant-service combination
	cacheKey := fmt.Sprintf("%s:%s", tenantContext.TenantHash, tenantContext.ServiceName)

	// Check if we already have a fusion instance for this tenant-service
	mtf.mu.RLock()
	fusion, exists := mtf.tenantFusions[cacheKey]
	mtf.mu.RUnlock()

	if exists {
		if mtf.logger != nil {
			mtf.logger.Debugf("Using cached fusion instance for tenant %s service %s",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
		}
		return fusion, nil
	}

	// Create a new fusion instance for this tenant-service
	fusion, err := mtf.createTenantFusion(tenantContext)
	if err != nil {
		if mtf.logger != nil {
			mtf.logger.Errorf("Failed to create fusion instance for tenant %s service %s: %v",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
		}
		return nil, fmt.Errorf("failed to create fusion for tenant: %w", err)
	}

	// Cache the fusion instance
	mtf.mu.Lock()
	mtf.tenantFusions[cacheKey] = fusion
	mtf.mu.Unlock()

	if mtf.logger != nil {
		mtf.logger.Infof("Created and cached new fusion instance for tenant %s service %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	return fusion, nil
}

// CallTool calls a tool for a specific tenant
func (mtf *MultiTenantFusion) CallTool(ctx context.Context, tenantContext *TenantContext,
	toolName string, args map[string]interface{}) (string, error) {

	if mtf.logger != nil {
		mtf.logger.Debugf("Calling tool %s for tenant %s service %s",
			toolName, tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	fusion, err := mtf.GetFusionForTenant(tenantContext)
	if err != nil {
		return "", fmt.Errorf("failed to get fusion for tenant: %w", err)
	}

	// Since fusion doesn't have CallTool method, we need to call the tool handler directly
	tools := fusion.RegisterTools()
	for _, tool := range tools {
		if tool.Name == toolName {
			return tool.Handler(args)
		}
	}
	return "", fmt.Errorf("tool not found: %s", toolName)
}

// GetResource gets a resource for a specific tenant
func (mtf *MultiTenantFusion) GetResource(ctx context.Context, tenantContext *TenantContext,
	resourceURI string) (string, error) {

	if mtf.logger != nil {
		mtf.logger.Debugf("Getting resource %s for tenant %s service %s",
			resourceURI, tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	fusion, err := mtf.GetFusionForTenant(tenantContext)
	if err != nil {
		return "", fmt.Errorf("failed to get fusion for tenant: %w", err)
	}

	// Since fusion doesn't have GetResource method, we need to call the resource handler directly
	resources := fusion.RegisterResources()
	for _, resource := range resources {
		if resource.URI == resourceURI {
			response, err := resource.Handler(resourceURI, make(map[string]interface{}))
			return response.Content, err
		}
	}
	// Try resource templates
	templates := fusion.RegisterResourceTemplates()
	for _, template := range templates {
		// Simple URI matching - in a real implementation you'd do proper template matching
		if strings.Contains(resourceURI, template.Name) {
			response, err := template.Handler(resourceURI, make(map[string]interface{}))
			return response.Content, err
		}
	}
	return "", fmt.Errorf("resource not found: %s", resourceURI)
}

// GetPrompt gets a prompt for a specific tenant
func (mtf *MultiTenantFusion) GetPrompt(ctx context.Context, tenantContext *TenantContext,
	promptName string, args map[string]interface{}) (string, global.Messages, error) {

	if mtf.logger != nil {
		mtf.logger.Debugf("Getting prompt %s for tenant %s service %s",
			promptName, tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	fusion, err := mtf.GetFusionForTenant(tenantContext)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get fusion for tenant: %w", err)
	}

	// Since fusion doesn't have GetPrompt method, we need to call the prompt handler directly
	prompts := fusion.RegisterPrompts()
	for _, prompt := range prompts {
		if prompt.Name == promptName {
			return prompt.Handler(args)
		}
	}
	return "", nil, fmt.Errorf("prompt not found: %s", promptName)
}

// InvalidateTenantCache removes a tenant's cached fusion instance
func (mtf *MultiTenantFusion) InvalidateTenantCache(tenantContext *TenantContext) {
	if tenantContext == nil {
		return
	}

	cacheKey := fmt.Sprintf("%s:%s", tenantContext.TenantHash, tenantContext.ServiceName)

	mtf.mu.Lock()
	delete(mtf.tenantFusions, cacheKey)
	mtf.mu.Unlock()

	// Also invalidate authentication tokens
	mtf.authManager.InvalidateToken(tenantContext)

	if mtf.logger != nil {
		mtf.logger.Infof("Invalidated cache for tenant %s service %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}
}

// GetAuthManager returns the multi-tenant authentication manager
func (mtf *MultiTenantFusion) GetAuthManager() *MultiTenantAuthManager {
	return mtf.authManager
}

// GetServiceResolver returns the service configuration resolver
func (mtf *MultiTenantFusion) GetServiceResolver() *ServiceConfigResolver {
	return mtf.serviceResolver
}

// GetDatabaseCache returns the database cache
func (mtf *MultiTenantFusion) GetDatabaseCache() *DatabaseCache {
	return mtf.databaseCache
}

// createTenantFusion creates a new Fusion instance for a specific tenant and service
func (mtf *MultiTenantFusion) createTenantFusion(tenantContext *TenantContext) (*Fusion, error) {
	// Resolve service configuration
	serviceConfig, err := mtf.serviceResolver.ResolveServiceForTenant(
		tenantContext.TenantHash, tenantContext.ServiceName)
	if err != nil {
		if mtf.logger != nil {
			mtf.logger.Warningf("Failed to resolve service config for tenant %s service %s, using default: %v",
				tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName, err)
		}

		// Fall back to default configuration if available
		if mtf.defaultConfig != nil {
			serviceConfig = mtf.defaultConfig.GetServiceByName(tenantContext.ServiceName)
			if serviceConfig == nil && len(mtf.defaultConfig.Services) > 0 {
				// Use the first available service as fallback
				for _, service := range mtf.defaultConfig.Services {
					serviceConfig = service
					break
				}
			}
		}

		if serviceConfig == nil {
			return nil, fmt.Errorf("no service configuration available for %s", tenantContext.ServiceName)
		}
	}

	// Create a tenant-specific configuration
	config := &Config{
		Services: map[string]*ServiceConfig{
			tenantContext.ServiceName: serviceConfig,
		},
		Logger:     mtf.logger,
		HTTPClient: mtf.httpClient,
		Cache:      mtf.databaseCache,
	}

	// Note: We can't easily set a custom auth manager due to type constraints
	// The fusion instance will use its own auth manager instead of tenant-specific wrapper

	// Create the fusion instance
	fusion := New(
		WithConfig(config),
	)
	if fusion == nil {
		return nil, fmt.Errorf("failed to create fusion instance")
	}

	if mtf.logger != nil {
		mtf.logger.Debugf("Created fusion instance for tenant %s service %s",
			tenantContext.TenantHash[:12]+"...", tenantContext.ServiceName)
	}

	return fusion, nil
}

// registerDefaultAuthStrategies registers the default authentication strategies
func (mtf *MultiTenantFusion) registerDefaultAuthStrategies() {
	// Register OAuth2 device flow strategy
	oauth2Strategy := NewOAuth2DeviceFlowStrategy(mtf.httpClient, mtf.logger)
	mtf.authManager.RegisterStrategy(oauth2Strategy)

	// Register bearer token strategy
	bearerStrategy := NewBearerTokenStrategy(mtf.logger)
	mtf.authManager.RegisterStrategy(bearerStrategy)

	// Register API key strategy
	apiKeyStrategy := NewAPIKeyStrategy(mtf.logger)
	mtf.authManager.RegisterStrategy(apiKeyStrategy)

	// Register basic auth strategy
	basicAuthStrategy := NewBasicAuthStrategy(mtf.logger)
	mtf.authManager.RegisterStrategy(basicAuthStrategy)

	if mtf.logger != nil {
		strategies := mtf.authManager.GetRegisteredStrategies()
		mtf.logger.Infof("Registered authentication strategies: %v", strategies)
	}
}

// Close closes the multi-tenant fusion and cleans up resources
func (mtf *MultiTenantFusion) Close() error {
	if mtf.logger != nil {
		mtf.logger.Info("Closing multi-tenant fusion")
	}

	// Close service resolver
	if mtf.serviceResolver != nil {
		if err := mtf.serviceResolver.Close(); err != nil {
			if mtf.logger != nil {
				mtf.logger.Errorf("Error closing service resolver: %v", err)
			}
		}
	}

	// Clear tenant fusion cache
	mtf.mu.Lock()
	for key := range mtf.tenantFusions {
		// Fusion doesn't have a Close method, just remove from cache
		delete(mtf.tenantFusions, key)
	}
	mtf.mu.Unlock()

	if mtf.logger != nil {
		mtf.logger.Info("Multi-tenant fusion closed successfully")
	}

	return nil
}

// GetStats returns statistics about the multi-tenant fusion
func (mtf *MultiTenantFusion) GetStats() map[string]interface{} {
	mtf.mu.RLock()
	tenantCount := len(mtf.tenantFusions)
	mtf.mu.RUnlock()

	stats := map[string]interface{}{
		"active_tenants":             tenantCount,
		"database_cache_available":   mtf.databaseCache != nil,
		"service_resolver_available": mtf.serviceResolver != nil,
		"auth_strategies":            mtf.authManager.GetRegisteredStrategies(),
	}

	if mtf.serviceResolver != nil {
		stats["service_resolver"] = mtf.serviceResolver.GetStats()
	}

	if mtf.databaseCache != nil {
		stats["database_cache"] = mtf.databaseCache.GetStats()
	}

	return stats
}

// tenantAuthManagerWrapper wraps the MultiTenantAuthManager to provide a standard AuthManager interface
// for use with individual Fusion instances
type tenantAuthManagerWrapper struct {
	multiTenantAuthManager *MultiTenantAuthManager
	tenantContext          *TenantContext
	logger                 global.Logger
}

// RegisterStrategy is not supported for tenant-specific auth managers
func (tam *tenantAuthManagerWrapper) RegisterStrategy(strategy AuthStrategy) {
	if tam.logger != nil {
		tam.logger.Warning("RegisterStrategy called on tenant auth manager wrapper - operation ignored")
	}
}

// GetToken gets a token for the wrapped tenant context
func (tam *tenantAuthManagerWrapper) GetToken(ctx context.Context, serviceName string, authConfig AuthConfig) (*TokenInfo, error) {
	// Update the tenant context with the correct service name
	tenantContext := *tam.tenantContext // Copy the context
	tenantContext.ServiceName = serviceName

	return tam.multiTenantAuthManager.GetToken(ctx, &tenantContext, authConfig)
}

// ApplyAuthentication applies authentication for the wrapped tenant context
func (tam *tenantAuthManagerWrapper) ApplyAuthentication(ctx context.Context, req *http.Request,
	serviceName string, authConfig AuthConfig) error {

	// Update the tenant context with the correct service name
	tenantContext := *tam.tenantContext // Copy the context
	tenantContext.ServiceName = serviceName

	return tam.multiTenantAuthManager.ApplyAuthentication(ctx, req, &tenantContext, authConfig)
}

// InvalidateToken invalidates a token for the wrapped tenant context
func (tam *tenantAuthManagerWrapper) InvalidateToken(serviceName string) {
	// Update the tenant context with the correct service name
	tenantContext := *tam.tenantContext // Copy the context
	tenantContext.ServiceName = serviceName

	tam.multiTenantAuthManager.InvalidateToken(&tenantContext)
}

// GetRegisteredStrategies returns the registered strategies from the underlying auth manager
func (tam *tenantAuthManagerWrapper) GetRegisteredStrategies() []AuthType {
	return tam.multiTenantAuthManager.GetRegisteredStrategies()
}

// HasStrategy checks if a strategy is registered
func (tam *tenantAuthManagerWrapper) HasStrategy(authType AuthType) bool {
	return tam.multiTenantAuthManager.HasStrategy(authType)
}
