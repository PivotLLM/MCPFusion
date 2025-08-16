/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// ServiceConfigResolver resolves service configurations dynamically from JSON files
type ServiceConfigResolver struct {
	configPaths  []string                  // Paths to search for config files
	configs      map[string]*ServiceConfig // Cached service configurations
	lastModified map[string]time.Time      // Last modification times for config files
	logger       global.Logger
	mu           sync.RWMutex
	autoReload   bool          // Whether to automatically reload changed configs
	reloadTicker *time.Ticker  // Ticker for periodic reload checks
	stopReload   chan struct{} // Channel to stop the reload goroutine
}

// ServiceResolverOption represents configuration options for the service resolver
type ServiceResolverOption func(*ServiceConfigResolver)

// WithConfigPaths sets the configuration file search paths
func WithConfigPaths(paths ...string) ServiceResolverOption {
	return func(scr *ServiceConfigResolver) {
		scr.configPaths = paths
	}
}

// WithAutoReload enables automatic reloading of configuration files
func WithAutoReload(interval time.Duration) ServiceResolverOption {
	return func(scr *ServiceConfigResolver) {
		scr.autoReload = true
		if scr.reloadTicker != nil {
			scr.reloadTicker.Stop()
		}
		scr.reloadTicker = time.NewTicker(interval)
	}
}

// WithSRLogger sets the logger for the service resolver
func WithSRLogger(logger global.Logger) ServiceResolverOption {
	return func(scr *ServiceConfigResolver) {
		scr.logger = logger
	}
}

// NewServiceConfigResolver creates a new service configuration resolver
func NewServiceConfigResolver(options ...ServiceResolverOption) *ServiceConfigResolver {
	scr := &ServiceConfigResolver{
		configPaths:  []string{"./configs", "~/.mcpfusion/configs", "/etc/mcpfusion/configs"},
		configs:      make(map[string]*ServiceConfig),
		lastModified: make(map[string]time.Time),
		stopReload:   make(chan struct{}),
	}

	// Apply options
	for _, option := range options {
		option(scr)
	}

	// Start auto-reload if enabled
	if scr.autoReload && scr.reloadTicker != nil {
		go scr.autoReloadLoop()
	}

	if scr.logger != nil {
		scr.logger.Infof("Initialized service config resolver with paths: %v", scr.configPaths)
	}

	return scr
}

// ResolveService resolves a service configuration by name
func (scr *ServiceConfigResolver) ResolveService(serviceName string) (*ServiceConfig, error) {
	if scr.logger != nil {
		scr.logger.Debugf("Resolving service configuration for: %s", serviceName)
	}

	scr.mu.RLock()
	config, exists := scr.configs[serviceName]
	scr.mu.RUnlock()

	if exists {
		if scr.logger != nil {
			scr.logger.Debugf("Found cached configuration for service: %s", serviceName)
		}
		return config, nil
	}

	// Service not cached, try to load it
	if scr.logger != nil {
		scr.logger.Debugf("Service %s not cached, attempting to load from files", serviceName)
	}

	config, err := scr.loadServiceConfig(serviceName)
	if err != nil {
		if scr.logger != nil {
			scr.logger.Errorf("Failed to load configuration for service %s: %v", serviceName, err)
		}
		return nil, fmt.Errorf("failed to resolve service '%s': %w", serviceName, err)
	}

	// Cache the configuration
	scr.mu.Lock()
	scr.configs[serviceName] = config
	scr.mu.Unlock()

	if scr.logger != nil {
		scr.logger.Infof("Successfully loaded and cached configuration for service: %s", serviceName)
	}

	return config, nil
}

// ResolveServiceForTenant resolves a service configuration for a specific tenant
// This allows for tenant-specific service configurations
func (scr *ServiceConfigResolver) ResolveServiceForTenant(tenantHash, serviceName string) (*ServiceConfig, error) {
	if scr.logger != nil {
		scr.logger.Debugf("Resolving service configuration for tenant %s service %s",
			tenantHash[:12]+"...", serviceName)
	}

	// First try to find a tenant-specific configuration
	tenantSpecificName := fmt.Sprintf("%s.%s", tenantHash[:12], serviceName)
	config, err := scr.ResolveService(tenantSpecificName)
	if err == nil {
		if scr.logger != nil {
			scr.logger.Debugf("Found tenant-specific configuration for %s", tenantSpecificName)
		}
		return config, nil
	}

	// Fall back to the default service configuration
	if scr.logger != nil {
		scr.logger.Debugf("No tenant-specific config found, falling back to default for service: %s", serviceName)
	}
	return scr.ResolveService(serviceName)
}

// LoadAllServices loads all available service configurations
func (scr *ServiceConfigResolver) LoadAllServices() error {
	if scr.logger != nil {
		scr.logger.Debug("Loading all available service configurations")
	}

	configFiles, err := scr.findAllConfigFiles()
	if err != nil {
		return fmt.Errorf("failed to find config files: %w", err)
	}

	loadedCount := 0
	for _, configFile := range configFiles {
		serviceName := scr.getServiceNameFromFile(configFile)
		if serviceName == "" {
			if scr.logger != nil {
				scr.logger.Warningf("Could not determine service name from file: %s", configFile)
			}
			continue
		}

		config, err := scr.loadServiceConfigFromFile(configFile)
		if err != nil {
			if scr.logger != nil {
				scr.logger.Errorf("Failed to load config from file %s: %v", configFile, err)
			}
			continue
		}

		scr.mu.Lock()
		scr.configs[serviceName] = config
		scr.mu.Unlock()

		loadedCount++
		if scr.logger != nil {
			scr.logger.Debugf("Loaded configuration for service %s from %s", serviceName, configFile)
		}
	}

	if scr.logger != nil {
		scr.logger.Infof("Loaded %d service configurations from %d files", loadedCount, len(configFiles))
	}

	return nil
}

// GetAvailableServices returns a list of all available service names
func (scr *ServiceConfigResolver) GetAvailableServices() []string {
	scr.mu.RLock()
	defer scr.mu.RUnlock()

	services := make([]string, 0, len(scr.configs))
	for serviceName := range scr.configs {
		services = append(services, serviceName)
	}

	return services
}

// ReloadService forces a reload of a specific service configuration
func (scr *ServiceConfigResolver) ReloadService(serviceName string) error {
	if scr.logger != nil {
		scr.logger.Debugf("Force reloading service configuration for: %s", serviceName)
	}

	// Remove from cache first
	scr.mu.Lock()
	delete(scr.configs, serviceName)
	scr.mu.Unlock()

	// Load fresh configuration
	_, err := scr.ResolveService(serviceName)
	return err
}

// ReloadAll forces a reload of all cached service configurations
func (scr *ServiceConfigResolver) ReloadAll() error {
	if scr.logger != nil {
		scr.logger.Debug("Force reloading all service configurations")
	}

	// Clear the cache
	scr.mu.Lock()
	scr.configs = make(map[string]*ServiceConfig)
	scr.lastModified = make(map[string]time.Time)
	scr.mu.Unlock()

	// Reload all services
	return scr.LoadAllServices()
}

// Close stops the auto-reload functionality and cleans up resources
func (scr *ServiceConfigResolver) Close() error {
	if scr.autoReload && scr.reloadTicker != nil {
		scr.reloadTicker.Stop()
		close(scr.stopReload)

		if scr.logger != nil {
			scr.logger.Debug("Stopped service config resolver auto-reload")
		}
	}

	return nil
}

// loadServiceConfig loads a service configuration by name from the configured paths
func (scr *ServiceConfigResolver) loadServiceConfig(serviceName string) (*ServiceConfig, error) {
	// Try different file patterns for the service
	patterns := []string{
		fmt.Sprintf("%s.json", serviceName),
		fmt.Sprintf("%s-config.json", serviceName),
		fmt.Sprintf("service-%s.json", serviceName),
	}

	for _, path := range scr.configPaths {
		expandedPath := scr.expandPath(path)
		for _, pattern := range patterns {
			configFile := filepath.Join(expandedPath, pattern)

			if scr.logger != nil {
				scr.logger.Debugf("Checking for config file: %s", configFile)
			}

			if _, err := os.Stat(configFile); err == nil {
				if scr.logger != nil {
					scr.logger.Debugf("Found config file: %s", configFile)
				}
				return scr.loadServiceConfigFromFile(configFile)
			}
		}
	}

	return nil, fmt.Errorf("service configuration file not found for '%s' in paths: %v", serviceName, scr.configPaths)
}

// loadServiceConfigFromFile loads a service configuration from a specific file
func (scr *ServiceConfigResolver) loadServiceConfigFromFile(configFile string) (*ServiceConfig, error) {
	if scr.logger != nil {
		scr.logger.Debugf("Loading service configuration from file: %s", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	// Expand environment variables
	expandedData, err := expandEnvironmentVariables(data)
	if err != nil {
		return nil, fmt.Errorf("failed to expand environment variables in %s: %w", configFile, err)
	}

	var config ServiceConfig
	if err := json.Unmarshal(expandedData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON config file %s: %w", configFile, err)
	}

	// Validate the configuration
	if err := config.ValidateWithLogger(config.Name, scr.logger); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", configFile, err)
	}

	// Update last modified time
	if stat, err := os.Stat(configFile); err == nil {
		scr.mu.Lock()
		scr.lastModified[configFile] = stat.ModTime()
		scr.mu.Unlock()
	}

	if scr.logger != nil {
		scr.logger.Debugf("Successfully loaded service configuration for %s from %s", config.Name, configFile)
	}

	return &config, nil
}

// findAllConfigFiles finds all configuration files in the configured paths
func (scr *ServiceConfigResolver) findAllConfigFiles() ([]string, error) {
	var configFiles []string

	for _, path := range scr.configPaths {
		expandedPath := scr.expandPath(path)

		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			continue // Skip non-existent paths
		}

		files, err := filepath.Glob(filepath.Join(expandedPath, "*.json"))
		if err != nil {
			if scr.logger != nil {
				scr.logger.Warningf("Failed to scan directory %s: %v", expandedPath, err)
			}
			continue
		}

		configFiles = append(configFiles, files...)
	}

	return configFiles, nil
}

// getServiceNameFromFile extracts the service name from a configuration file path
func (scr *ServiceConfigResolver) getServiceNameFromFile(configFile string) string {
	filename := filepath.Base(configFile)
	name := strings.TrimSuffix(filename, ".json")

	// Handle different naming patterns
	if strings.HasSuffix(name, "-config") {
		name = strings.TrimSuffix(name, "-config")
	} else if strings.HasPrefix(name, "service-") {
		name = strings.TrimPrefix(name, "service-")
	}

	return name
}

// expandPath expands environment variables and home directory in paths
func (scr *ServiceConfigResolver) expandPath(path string) string {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(homeDir, path[2:])
		}
	}

	// Expand environment variables
	return os.ExpandEnv(path)
}

// autoReloadLoop runs the automatic reload functionality
func (scr *ServiceConfigResolver) autoReloadLoop() {
	if scr.logger != nil {
		scr.logger.Debug("Started service config resolver auto-reload loop")
	}

	for {
		select {
		case <-scr.reloadTicker.C:
			scr.checkForChanges()
		case <-scr.stopReload:
			if scr.logger != nil {
				scr.logger.Debug("Service config resolver auto-reload loop stopped")
			}
			return
		}
	}
}

// checkForChanges checks if any configuration files have been modified and reloads them
func (scr *ServiceConfigResolver) checkForChanges() {
	if scr.logger != nil {
		scr.logger.Debug("Checking for configuration file changes")
	}

	scr.mu.RLock()
	filesToCheck := make(map[string]time.Time)
	for file, modTime := range scr.lastModified {
		filesToCheck[file] = modTime
	}
	scr.mu.RUnlock()

	changedFiles := 0
	for file, lastModTime := range filesToCheck {
		if stat, err := os.Stat(file); err == nil {
			if stat.ModTime().After(lastModTime) {
				if scr.logger != nil {
					scr.logger.Infof("Configuration file changed: %s", file)
				}

				// Reload the service configuration
				serviceName := scr.getServiceNameFromFile(file)
				if err := scr.ReloadService(serviceName); err != nil {
					if scr.logger != nil {
						scr.logger.Errorf("Failed to reload changed service %s: %v", serviceName, err)
					}
				} else {
					changedFiles++
				}
			}
		}
	}

	if changedFiles > 0 && scr.logger != nil {
		scr.logger.Infof("Reloaded %d changed service configurations", changedFiles)
	}
}

// GetStats returns statistics about the service resolver
func (scr *ServiceConfigResolver) GetStats() map[string]interface{} {
	scr.mu.RLock()
	defer scr.mu.RUnlock()

	return map[string]interface{}{
		"config_paths":    scr.configPaths,
		"cached_services": len(scr.configs),
		"auto_reload":     scr.autoReload,
		"service_names":   scr.GetAvailableServices(),
	}
}

// ValidateAllServices validates all cached service configurations
func (scr *ServiceConfigResolver) ValidateAllServices() error {
	scr.mu.RLock()
	configs := make(map[string]*ServiceConfig)
	for name, config := range scr.configs {
		configs[name] = config
	}
	scr.mu.RUnlock()

	var errors []string
	for serviceName, config := range configs {
		if err := config.ValidateWithLogger(serviceName, scr.logger); err != nil {
			errors = append(errors, fmt.Sprintf("service %s: %v", serviceName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed for %d services: %s", len(errors), strings.Join(errors, "; "))
	}

	if scr.logger != nil {
		scr.logger.Infof("Validated %d service configurations successfully", len(configs))
	}

	return nil
}
