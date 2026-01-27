/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package config

import (
	"fmt"
	"sync"

	"github.com/PivotLLM/MCPFusion/fusion"
	"github.com/PivotLLM/MCPFusion/global"
)

// Manager manages all service configurations loaded from multiple files
type Manager struct {
	configFiles []string                              // List of config files to load
	services    map[string]*fusion.ServiceConfig      // Merged services from all files
	commands    map[string]*fusion.CommandGroupConfig // Merged commands from all files
	logger      global.Logger
	mu          sync.RWMutex
}

// Option defines a function type for configuring the Manager
type Option func(*Manager)

// WithLogger sets the logger for the config manager
func WithLogger(logger global.Logger) Option {
	return func(m *Manager) {
		m.logger = logger
	}
}

// WithConfigFiles sets the configuration files to load
func WithConfigFiles(files ...string) Option {
	return func(m *Manager) {
		m.configFiles = files
	}
}

// New creates a new config manager instance
func New(options ...Option) *Manager {
	m := &Manager{
		services:    make(map[string]*fusion.ServiceConfig),
		commands:    make(map[string]*fusion.CommandGroupConfig),
		configFiles: []string{},
	}

	// Apply options
	for _, option := range options {
		option(m)
	}

	return m
}

// LoadConfigs loads all configured configuration files and merges them
func (m *Manager) LoadConfigs() error {
	if len(m.configFiles) == 0 {
		if m.logger != nil {
			m.logger.Warning("No configuration files specified")
		}
		return nil // Not an error, just no configs
	}

	successCount := 0
	for _, configFile := range m.configFiles {
		if m.logger != nil {
			m.logger.Infof("Loading configuration file: %s", configFile)
		}

		if err := m.loadAndMergeConfig(configFile); err != nil {
			if m.logger != nil {
				m.logger.Errorf("Failed to load config %s: %v", configFile, err)
			}
			// Continue loading other files even if one fails
			continue
		}
		successCount++
		if m.logger != nil {
			m.logger.Infof("Successfully loaded config: %s", configFile)
		}
	}

	if successCount == 0 && len(m.configFiles) > 0 {
		return fmt.Errorf("failed to load any configuration files from %d specified", len(m.configFiles))
	}

	if m.logger != nil {
		m.logger.Infof("Loaded %d services and %d command groups from %d config files",
			len(m.services), len(m.commands), successCount)
	}

	return nil
}

// loadAndMergeConfig loads a single config file and merges its services and commands
func (m *Manager) loadAndMergeConfig(configFile string) error {
	// Load the Config from file using fusion's existing loader
	config, err := fusion.LoadConfigFromFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}

	// Merge services and commands into our maps
	m.mu.Lock()
	defer m.mu.Unlock()

	// Merge services
	serviceCount := 0
	for serviceName, service := range config.Services {
		if _, exists := m.services[serviceName]; exists {
			if m.logger != nil {
				m.logger.Warningf("Service '%s' from %s overwrites previous definition",
					serviceName, configFile)
			}
		}
		m.services[serviceName] = service
		serviceCount++
		if m.logger != nil {
			m.logger.Debugf("Loaded service '%s' from %s", serviceName, configFile)
		}
	}

	// Merge commands
	commandCount := 0
	for commandGroupName, commandGroup := range config.Commands {
		if _, exists := m.commands[commandGroupName]; exists {
			if m.logger != nil {
				m.logger.Warningf("Command group '%s' from %s overwrites previous definition",
					commandGroupName, configFile)
			}
		}
		m.commands[commandGroupName] = commandGroup
		commandCount++
		if m.logger != nil {
			m.logger.Debugf("Loaded command group '%s' with %d commands from %s",
				commandGroupName, len(commandGroup.Commands), configFile)
		}
	}

	if m.logger != nil {
		m.logger.Debugf("Merged %d services and %d command groups from %s",
			serviceCount, commandCount, configFile)
	}

	return nil
}

// GetService returns a specific service configuration by name
func (m *Manager) GetService(name string) (*fusion.ServiceConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	service, exists := m.services[name]
	if !exists {
		return nil, fmt.Errorf("service '%s' not found", name)
	}

	return service, nil
}

// GetAllServices returns all loaded service configurations
func (m *Manager) GetAllServices() map[string]*fusion.ServiceConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modifications
	services := make(map[string]*fusion.ServiceConfig)
	for name, service := range m.services {
		services[name] = service
	}

	return services
}

// GetServiceNames returns a list of all loaded service names
func (m *Manager) GetServiceNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.services))
	for name := range m.services {
		names = append(names, name)
	}

	return names
}

// GetAvailableServices returns a list of all available service names
// This method is for compatibility with the existing ServiceConfigResolver interface
func (m *Manager) GetAvailableServices() []string {
	return m.GetServiceNames()
}

// HasService checks if a service exists
func (m *Manager) HasService(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.services[name]
	return exists
}

// ServiceCount returns the number of loaded services
func (m *Manager) ServiceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.services)
}

// GetConfig returns a full Config object with all services and commands
// This is useful for Fusion which expects a Config structure
func (m *Manager) GetConfig() *fusion.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &fusion.Config{
		Services: m.services,
		Commands: m.commands,
	}
}

// GetCommand returns a specific command group configuration by name
func (m *Manager) GetCommand(name string) (*fusion.CommandGroupConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	commandGroup, exists := m.commands[name]
	if !exists {
		return nil, fmt.Errorf("command group '%s' not found", name)
	}

	return commandGroup, nil
}

// GetAllCommands returns all loaded command group configurations
func (m *Manager) GetAllCommands() map[string]*fusion.CommandGroupConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modifications
	commands := make(map[string]*fusion.CommandGroupConfig)
	for name, commandGroup := range m.commands {
		commands[name] = commandGroup
	}

	return commands
}

// GetCommandGroupNames returns a list of all loaded command group names
func (m *Manager) GetCommandGroupNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.commands))
	for name := range m.commands {
		names = append(names, name)
	}

	return names
}

// HasCommand checks if a command group exists
func (m *Manager) HasCommand(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.commands[name]
	return exists
}

// CommandCount returns the number of loaded command groups
func (m *Manager) CommandCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.commands)
}
