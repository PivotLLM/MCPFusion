/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package fusion provides a production-ready, configuration-driven MCP (Model Context Protocol)
// provider that enables seamless integration with multiple APIs through JSON configuration.
//
// Key Features:
// - OAuth2 Device Flow authentication (Microsoft 365, Google APIs)
// - Advanced retry logic with circuit breakers
// - Response caching with intelligent TTL management
// - Parameter validation and transformation
// - Comprehensive error handling with correlation tracking
// - Real-time metrics collection and monitoring
// - Support for paginated responses with automatic multi-page fetching
//
// This package is production-ready and supports enterprise-grade deployments with
// advanced reliability features including exponential backoff retries, circuit breaker
// patterns, and comprehensive observability.
//
// Example usage:
//
//	multiTenantAuth := fusion.NewMultiTenantAuthManager(db, cache, fusion.WithLogger(logger))
//	fusionProvider, err := fusion.New(
//		fusion.WithJSONConfig("configs/microsoft365.json"),
//		fusion.WithLogger(logger),
//	)
//	server.AddToolProvider(fusionProvider)
package fusion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/metrics"
	"github.com/PivotLLM/MCPFusion/providers/health"
	"github.com/PivotLLM/toolspec"
	"github.com/joho/godotenv"
)

// Ensure Fusion implements the required interfaces
var _ global.ToolProvider = (*Fusion)(nil)
var _ global.ResourceProvider = (*Fusion)(nil)
var _ global.PromptProvider = (*Fusion)(nil)

// Fusion is the main provider struct that implements the MCPFusion interfaces for
// dynamic API integration. It manages configuration, authentication, caching, metrics,
// and provides production-grade reliability features.
//
// The Fusion struct is thread-safe and can handle concurrent requests across multiple
// API services. It implements global.ToolProvider, global.ResourceProvider, and
// global.PromptProvider interfaces for seamless integration with MCPFusion servers.
//
// Key Components:
// - config: Service and endpoint configuration loaded from JSON
// - authManager: Handles OAuth2, bearer tokens, API keys, and basic auth
// - httpClient: HTTP client with timeout and connection pooling
// - cache: Token and response caching for performance optimization
// - logger: Structured logging with correlation ID support
// - metricsCollector: Real-time metrics and health monitoring
// - circuitBreakers: Per-service circuit breaker protection
//
// Thread Safety:
// All public methods are thread-safe and can be called concurrently from multiple
// goroutines. Internal state is protected by appropriate synchronization primitives.
type Fusion struct {
	config                 *Config                    // Service configuration and endpoints
	multiTenantAuth        *MultiTenantAuthManager    // Multi-tenant authentication manager (required)
	httpClient             *http.Client               // HTTP client with timeouts
	cache                  Cache                      // Database cache from multi-tenant auth manager
	logger                 global.Logger              // Structured logging interface
	metricsCollector       *MetricsCollector          // Performance and health metrics
	correlationIDGenerator *CorrelationIDGenerator    // Request correlation tracking
	circuitBreakers        map[string]*CircuitBreaker // Per-service circuit breakers
	circuitBreakersMutex   sync.RWMutex               // Protects circuitBreakers map

	// Connection health management
	connectionCleanupTicker *time.Ticker  // Periodic connection cleanup
	shutdownChan            chan struct{} // Channel for graceful shutdown
	shutdownOnce            sync.Once     // Ensures cleanup happens only once

	// External URL for auth code blob generation (used by auth setup tools)
	externalURL string

	// allowDestructive controls whether destructive tools (DELETE, etc.) can execute.
	// When false, destructive tools are still registered but return an error when called.
	allowDestructive bool

	// database provides direct access to the database for native tools (e.g., knowledge store)
	database db.Database

	// sharedCollector is the cross-package metrics collector for the health tool
	sharedCollector *metrics.Collector

	// nativeToolPrefixRegistrar registers native tool prefixes with the config manager
	nativeToolPrefixRegistrar NativeToolPrefixRegistrar

	// maxResponseBytes limits the size of responses returned to callers.
	// Responses exceeding this limit are replaced with an informational message.
	// A value of 0 disables the limit.
	maxResponseBytes int

	// downloadDir is the directory where binary responses are saved.
	// If empty, binary responses return an informational message only.
	downloadDir string

	// authCommandName is the CLI command name shown in auth setup instructions
	// (default "fusion-auth"). Embedding hosts (e.g. ClawEh) override it via
	// WithAuthCommandName so the utility name matches their distribution.
	authCommandName string
}

// NativeToolPrefixRegistrar allows registering prefixes for native (non-config-driven)
// tools so that auth middleware recognises them as valid service names.
type NativeToolPrefixRegistrar interface {
	RegisterNativeToolPrefix(prefix string)
}

// Option configures a Fusion instance. Options are applied during New.
//
// Example usage:
//
//	fusion, err := New(
//		WithJSONConfig("config.json"),
//		WithLogger(logger),
//	)
type Option interface {
	applyToFusion(*Fusion)
}

// optionFunc adapts a function to the Option interface.
type optionFunc func(*Fusion)

func (o optionFunc) applyToFusion(f *Fusion) { o(f) }

// WithJSONConfig loads API service configuration from a JSON file.
// This is the primary way to configure API endpoints, authentication, and service settings.
//
// The configuration file supports environment variable expansion using ${VAR_NAME} syntax
// and includes comprehensive validation to ensure all required fields are present.
//
// Parameters:
//   - configPath: File path to the JSON configuration file
//
// The configuration file should contain a "services" object with service definitions.
// Each service can define multiple endpoints with parameters, authentication, and response handling.
//
// Example:
//
//	fusion, err := New(WithJSONConfig("configs/microsoft365.json"))
//
// If the file cannot be loaded or contains invalid configuration, the error will be
// logged and the Fusion instance will be created without that configuration.
func WithJSONConfig(configPath string) Option {
	return optionFunc(func(f *Fusion) {
		if f.logger != nil {
			f.logger.Infof("Loading configuration from file: %s", configPath)
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			if f.logger != nil {
				f.logger.Fatalf("Failed to read config file %s: %v", configPath, err)
			}
			return
		}

		config, err := LoadConfigFromJSONWithLogger(data, configPath, f.logger)
		if err != nil {
			if f.logger != nil {
				f.logger.Fatalf("Failed to load config from %s: %v", configPath, err)
			}
			return
		}
		f.config = config
	})
}

// WithJSONConfigData loads configuration from JSON data
func WithJSONConfigData(jsonData []byte, configPath string) Option {
	return optionFunc(func(f *Fusion) {
		if f.logger != nil {
			f.logger.Infof("Loading configuration from JSON data (path: %s)", configPath)
		}

		config, err := LoadConfigFromJSONWithLogger(jsonData, configPath, f.logger)
		if err != nil {
			if f.logger != nil {
				f.logger.Fatalf("Failed to load config from JSON data: %v", err)
			}
			return
		}
		f.config = config
	})
}

// WithConfig sets the configuration directly
func WithConfig(config *Config) Option {
	return optionFunc(func(f *Fusion) {
		f.config = config
	})
}

// WithConfigManager sets the configuration from a config manager.
// If the config manager also implements NativeToolPrefixRegistrar, the reference
// is stored so that native tool prefixes can be registered during RegisterTools().
func WithConfigManager(configManager interface{ Config() *Config }) Option {
	return optionFunc(func(f *Fusion) {
		if configManager != nil {
			f.config = configManager.Config()
			if f.logger != nil && f.config != nil {
				f.logger.Infof("Loaded configuration from config manager with %d services", len(f.config.Services))
			}
			if registrar, ok := configManager.(NativeToolPrefixRegistrar); ok {
				f.nativeToolPrefixRegistrar = registrar
			}
		}
	})
}

// WithHTTPClient sets a custom HTTP client
//
//goland:noinspection GoUnusedExportedFunction
func WithHTTPClient(client *http.Client) Option {
	return optionFunc(func(f *Fusion) {
		f.httpClient = client
	})
}

// Cache is managed exclusively by the multi-tenant auth manager
// Legacy cache options have been removed - only database cache is supported

// WithTimeout sets the HTTP client timeout
//
//goland:noinspection GoUnusedExportedFunction
func WithTimeout(timeout time.Duration) Option {
	return optionFunc(func(f *Fusion) {
		if f.httpClient == nil {
			f.httpClient = &http.Client{}
		}
		f.httpClient.Timeout = timeout
	})
}

// WithMetrics enables or disables metrics collection
//
//goland:noinspection GoUnusedExportedFunction
func WithMetrics(enabled bool) Option {
	return optionFunc(func(f *Fusion) {
		f.metricsCollector = NewMetricsCollector(enabled, WithLogger(f.logger))
	})
}

// WithMetricsCollector sets a custom metrics collector
//
//goland:noinspection GoUnusedExportedFunction
func WithMetricsCollector(collector *MetricsCollector) Option {
	return optionFunc(func(f *Fusion) {
		f.metricsCollector = collector
	})
}

// WithCorrelationIDGenerator sets a custom correlation ID generator
//
//goland:noinspection GoUnusedExportedFunction
func WithCorrelationIDGenerator(generator *CorrelationIDGenerator) Option {
	return optionFunc(func(f *Fusion) {
		f.correlationIDGenerator = generator
	})
}

// WithMultiTenantAuth sets a custom multi-tenant authentication manager
// NOTE: This is optional - if not provided, a default auth manager will be auto-created
func WithMultiTenantAuth(multiTenantAuth *MultiTenantAuthManager) Option {
	return optionFunc(func(f *Fusion) {
		f.multiTenantAuth = multiTenantAuth
	})
}

// WithExternalURL sets the externally-accessible URL of this server.
// Used by auth setup tools to generate auth code blobs for fusion-auth.
// Typically set from the MCP_FUSION_EXTERNAL_URL environment variable.
func WithExternalURL(url string) Option {
	return optionFunc(func(f *Fusion) {
		f.externalURL = url
	})
}

// WithDownloadDir sets the directory where binary responses (e.g. DOCX files) are saved.
// If the directory does not exist, Fusion will attempt to create it.
func WithDownloadDir(dir string) Option {
	return optionFunc(func(f *Fusion) {
		f.downloadDir = dir
	})
}

// WithDatabase sets the database for native tool operations such as the knowledge store.
func WithDatabase(database db.Database) Option {
	return optionFunc(func(f *Fusion) {
		f.database = database
	})
}

// WithDataStore wires stateful multi-tenant auth on top of a generic
// toolspec.DataStore, for embedded hosts (e.g. ClawEh) that provide only the
// 3-method DataStore KV rather than a full *db.DB. It builds a DataStore-backed
// TokenStore, a database cache, and a multi-tenant auth manager from it, so
// embedded callers get persistent OAuth tokens and auth codes without importing
// the bbolt database. A nil ds is ignored (behavior unchanged: New auto-creates a
// stateless manager). Place after WithLogger so the manager and cache share the
// configured logger.
func WithDataStore(ds toolspec.DataStore) Option {
	return optionFunc(func(f *Fusion) {
		if ds == nil {
			return
		}
		store := NewDataStoreTokenStore(ds, WithLogger(f.logger))
		dbCache := NewDatabaseCache(store, WithLogger(f.logger))
		f.multiTenantAuth = NewMultiTenantAuthManager(store, dbCache, WithLogger(f.logger))
		if f.logger != nil {
			f.logger.Info("Configured multi-tenant auth from DataStore-backed TokenStore")
		}
	})
}

// WithConfigDir loads configuration from a host-provided directory: it first
// loads "<dir>/env" via godotenv (so ${VAR} references in the JSON resolve), then
// loads and merges every "<dir>/*.json" file using the same loader as
// WithConfigFiles. Service and command definitions from all files are merged into
// a single Config. Later files overwrite earlier ones on name collision, matching
// the standalone config manager. Place after WithLogger for load logging.
func WithConfigDir(dir string) Option {
	return optionFunc(func(f *Fusion) {
		if dir == "" {
			return
		}

		// Load environment variables before any ${VAR} expansion in the JSON.
		envPath := filepath.Join(dir, "env")
		if _, err := os.Stat(envPath); err == nil {
			if err := godotenv.Load(envPath); err != nil {
				if f.logger != nil {
					f.logger.Warningf("Failed to load env file %s: %v", envPath, err)
				}
			} else if f.logger != nil {
				f.logger.Infof("Loaded environment from %s", envPath)
			}
		}

		matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil {
			if f.logger != nil {
				f.logger.Errorf("Failed to scan config dir %s: %v", dir, err)
			}
			return
		}
		sort.Strings(matches) // deterministic merge order

		merged := &Config{
			Services: make(map[string]*ServiceConfig),
			Commands: make(map[string]*CommandGroupConfig),
		}
		loaded := 0
		for _, path := range matches {
			cfg, err := LoadConfigFromFile(path)
			if err != nil {
				if f.logger != nil {
					f.logger.Errorf("Failed to load config %s: %v", path, err)
				}
				continue
			}
			for name, svc := range cfg.Services {
				if _, exists := merged.Services[name]; exists && f.logger != nil {
					f.logger.Warningf("Service '%s' from %s overwrites previous definition", name, path)
				}
				merged.Services[name] = svc
			}
			for name, grp := range cfg.Commands {
				if _, exists := merged.Commands[name]; exists && f.logger != nil {
					f.logger.Warningf("Command group '%s' from %s overwrites previous definition", name, path)
				}
				merged.Commands[name] = grp
			}
			loaded++
		}

		if loaded == 0 {
			if f.logger != nil {
				f.logger.Warningf("No configuration files loaded from %s", dir)
			}
			return
		}

		f.config = merged
		if f.logger != nil {
			f.logger.Infof("Loaded %d services and %d command groups from %s",
				len(merged.Services), len(merged.Commands), dir)
		}
	})
}

// WithSharedCollector sets the cross-package metrics collector used by the health tool
// to report request/error counts for all services (API, hub, knowledge, etc.).
func WithSharedCollector(c *metrics.Collector) Option {
	return optionFunc(func(f *Fusion) {
		f.sharedCollector = c
	})
}

// WithMaxResponseBytes sets a limit on the size of responses returned to callers.
// Responses exceeding this limit are replaced with an informational message.
// A value of 0 disables the limit. Default is global.DefaultMaxResponseBytes (1 MB).
func WithMaxResponseBytes(n int) Option {
	return optionFunc(func(f *Fusion) {
		f.maxResponseBytes = n
	})
}

// WithAllowDestructive enables destructive tools (e.g. DELETE operations) for this instance.
// This option overrides the MCP_FUSION_ALLOW_DESTRUCTIVE environment variable.
func WithAllowDestructive(allow bool) Option {
	return optionFunc(func(f *Fusion) {
		f.allowDestructive = allow
	})
}

// WithAuthCommandName sets the CLI command name shown in auth setup instructions
// (default "fusion-auth"). Embedding hosts override it, e.g. "claw-auth", so the
// generated command matches the utility they ship. An empty name is ignored.
func WithAuthCommandName(name string) Option {
	return optionFunc(func(f *Fusion) {
		if name != "" {
			f.authCommandName = name
		}
	})
}

// New creates a new production-ready Fusion instance with the provided configuration options.
// This is the primary constructor for the Fusion provider and initializes all components
// required for API integration including multi-tenant authentication, database caching,
// metrics, and circuit breakers.
//
// Multi-tenant authentication is automatically enabled by default. You can optionally
// provide a custom auth manager using WithMultiTenantAuth() if needed.
//
// Default Configuration:
// - HTTP Client: 30-second timeout with connection pooling
// - Cache: Database cache from multi-tenant auth manager
// - Metrics: Real-time metrics collection enabled
// - Circuit Breakers: Per-service failure protection
// - Correlation IDs: Request tracking for debugging
//
// The function applies functional options in order, allowing later options to override
// earlier ones. This provides maximum flexibility for configuration.
//
// Parameters:
//   - options: Variable number of functional options to configure the Fusion instance
//
// Returns:
//   - *Fusion: Configured Fusion instance ready for use as an MCP provider
//   - error: non-nil if the multi-tenant auth manager has no cache
//
// Example Basic Usage:
//
//	fusion, err := New()  // Creates instance with defaults
//
// Example Production Usage:
//
//	fusion, err := New(
//		WithJSONConfig("configs/microsoft365.json"),
//		WithJSONConfig("configs/google.json"),
//		WithLogger(logger),
//		WithTimeout(45*time.Second),
//		WithCircuitBreaker(true),
//		WithMetricsCollection(true),
//	)
//
// Thread Safety:
// The returned Fusion instance is thread-safe and can handle concurrent requests
// from multiple goroutines without additional synchronization.
func New(options ...Option) (*Fusion, error) {
	// Create custom HTTP transport with optimized connection pooling
	transport := &http.Transport{
		// Connection pooling settings
		MaxIdleConns:        global.HTTPMaxIdleConns,
		MaxIdleConnsPerHost: global.HTTPMaxIdleConnsPerHost,
		IdleConnTimeout:     global.HTTPIdleConnTimeout,

		// Keep-alive settings
		DisableKeepAlives: false,

		// Timeouts for connection establishment
		DialContext: (&net.Dialer{
			Timeout:   global.HTTPDialTimeout,
			KeepAlive: global.HTTPKeepAliveInterval,
		}).DialContext,

		// Response timeouts
		TLSHandshakeTimeout:   global.HTTPTLSHandshakeTimeout,
		ResponseHeaderTimeout: global.HTTPResponseHeaderTimeout,
		ExpectContinueTimeout: global.HTTPExpectContinueTimeout,

		// Connection limits
		MaxConnsPerHost: global.HTTPMaxConnsPerHost,
	}

	fusion := &Fusion{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   global.HTTPDefaultClientTimeout,
		},
		cache:                  nil,                       // Cache will be set by multi-tenant auth manager
		metricsCollector:       NewMetricsCollector(true), // Enable metrics by default
		correlationIDGenerator: NewCorrelationIDGenerator(),
		circuitBreakers:        make(map[string]*CircuitBreaker),
		maxResponseBytes:       global.DefaultMaxResponseBytes,
		authCommandName:        "fusion-auth",
	}

	// Apply all options
	for _, opt := range options {
		opt.applyToFusion(fusion)
	}

	// Parse MCP_FUSION_ALLOW_DESTRUCTIVE environment variable
	if envVal := os.Getenv("MCP_FUSION_ALLOW_DESTRUCTIVE"); envVal != "" {
		switch strings.ToLower(envVal) {
		case "true", "yes", "1":
			fusion.allowDestructive = true
		}
	}

	if fusion.logger != nil {
		fusion.logger.Debug("Initializing Fusion instance")
		fusion.logger.Debugf("HTTP client timeout: %v", fusion.httpClient.Timeout)
		fusion.logger.Infof("Destructive tools enabled: %v", fusion.allowDestructive)
	}

	// Automatically create multi-tenant auth manager if not provided
	if fusion.multiTenantAuth == nil {
		// Create database cache for multi-tenant authentication
		dbCache := NewDatabaseCache(nil, WithLogger(fusion.logger))
		fusion.multiTenantAuth = NewMultiTenantAuthManager(nil, dbCache, WithLogger(fusion.logger))

		if fusion.logger != nil {
			fusion.logger.Info("Auto-created multi-tenant authentication manager")
		}
	}

	// Use the database cache from multi-tenant auth manager
	if dbCache := fusion.multiTenantAuth.cache; dbCache != nil {
		fusion.cache = dbCache
		if fusion.logger != nil {
			fusion.logger.Info("Using database-backed cache for persistent token storage")
		}
	} else {
		return nil, errors.New("multi-tenant auth manager has no cache")
	}

	// Update metrics collector with logger
	if fusion.metricsCollector != nil {
		fusion.metricsCollector.logger = fusion.logger
		if fusion.logger != nil {
			fusion.logger.Debug("Metrics collection enabled")
		}
	}

	// Set references in config if we have one
	if fusion.config != nil {
		fusion.config.Logger = fusion.logger
		fusion.config.HTTPClient = fusion.httpClient
		fusion.config.Cache = fusion.cache

		// Note: Authentication is handled by the multi-tenant auth manager at the server level

		if fusion.logger != nil {
			fusion.logger.Infof("Fusion initialized with %d services", len(fusion.config.Services))

			// Log service summary
			for serviceName, service := range fusion.config.Services {
				fusion.logger.Debugf("Service '%s': baseURL=%s, auth=%s, endpoints=%d",
					serviceName, service.BaseURL, service.Auth.Type, len(service.Endpoints))
			}
		}
	} else {
		if fusion.logger != nil {
			fusion.logger.Warning("No configuration provided - services will need to be configured separately")
		}
	}

	// Initialize connection health management
	fusion.shutdownChan = make(chan struct{})
	fusion.startConnectionHealthManagement()

	if fusion.logger != nil {
		fusion.logger.Info("Fusion instance initialization completed")
	}

	return fusion, nil
}

// Legacy authentication strategies removed - only multi-tenant auth is supported

// Config returns the current configuration
func (f *Fusion) Config() *Config {
	return f.config
}

// Legacy GetAuthManager removed - use multi-tenant auth manager

// HTTPClient returns the HTTP client
func (f *Fusion) HTTPClient() *http.Client {
	return f.httpClient
}

// Cache returns the cache
func (f *Fusion) Cache() Cache {
	return f.cache
}

// Logger returns the logger
func (f *Fusion) Logger() global.Logger {
	return f.logger
}

// MaxResponseBytes returns the configured response size limit in bytes.
// A value of 0 means no limit is enforced.
func (f *Fusion) MaxResponseBytes() int {
	return f.maxResponseBytes
}

// DownloadDir returns the configured download directory for binary responses.
func (f *Fusion) DownloadDir() string {
	return f.downloadDir
}

// RegisterTools implements the global.ToolProvider interface and dynamically generates
// MCP tools based on the loaded JSON configuration. Each API endpoint becomes an executable
// tool that can be called by AI clients through the MCP protocol.
//
// This method is called by the MCPFusion server during initialization to discover all
// available tools. The tools are generated from endpoint configurations and include:
// - Parameter validation based on configuration
// - Authentication handling for the target API
// - Request/response transformation
// - Error handling with retry logic and circuit breakers
// - Caching support for improved performance
//
// Tool Naming Convention:
// Tools are named using the pattern: "{service_name}_{endpoint_id}"
// For example: "microsoft365_calendar_events" or "google_list_files"
//
// Returns:
//   - []global.ToolDefinition: Slice of tool definitions for MCP server registration
//
// The returned tools are fully functional and ready for execution. Each tool includes:
// - Name and description from endpoint configuration
// - Parameter schema with validation rules
// - Handler function that executes the API call
//
// Thread Safety:
// This method is thread-safe and can be called multiple times. It returns a new slice
// each time but the underlying tool handlers share the same Fusion instance state.
//
// Example:
// If configured with Microsoft 365 and Google APIs, this might return tools like:
// - microsoft365_get_profile
// - microsoft365_calendar_events
// - google_list_calendar_events
// - google_list_files
func (f *Fusion) RegisterTools() []global.ToolDefinition {
	if f.config == nil {
		if f.logger != nil {
			f.logger.Warning("No configuration loaded, cannot register tools")
		}
		return []global.ToolDefinition{}
	}

	var tools []global.ToolDefinition

	// Register service tools (existing)
	for serviceName, service := range f.config.Services {
		for _, endpoint := range service.Endpoints {
			tool := f.createToolDefinition(serviceName, service, &endpoint)
			tools = append(tools, tool)
		}
	}

	// Register auth setup tools for services requiring authentication
	for serviceName, service := range f.config.Services {
		if service.Auth.RequiresUserSetup() {
			tool := f.createAuthSetupToolDefinition(serviceName, service)
			tools = append(tools, tool)
			if f.logger != nil {
				f.logger.Infof("Registered auth setup tool: %s", tool.Name)
			}
		}
	}

	// Register command tools (NEW)
	for groupName, commandGroup := range f.config.Commands {
		for i := range commandGroup.Commands {
			command := &commandGroup.Commands[i]
			tool := f.createCommandToolDefinition(groupName, commandGroup, command)
			tools = append(tools, tool)
		}
	}

	// Register native tool prefixes so auth middleware recognises them
	if f.nativeToolPrefixRegistrar != nil {
		f.nativeToolPrefixRegistrar.RegisterNativeToolPrefix("command")
	}

	// Register services with the shared metrics collector
	if f.sharedCollector != nil {
		// API services (non-hub, config-driven)
		for serviceName, svc := range f.config.Services {
			if svc.IsHubService() {
				continue
			}
			toolCount := len(svc.Endpoints)
			f.sharedCollector.RegisterService(serviceName, global.TransportAPI, &toolCount)
		}
	}

	if f.logger != nil {
		f.logger.Infof("Registered %d dynamic tools from configuration", len(tools))
	}

	return tools
}

// createToolDefinition creates a tool definition from an endpoint configuration
func (f *Fusion) createToolDefinition(serviceName string, service *ServiceConfig, endpoint *EndpointConfig) global.ToolDefinition {
	// Validate parameter names for conflicts
	if err := ValidateParameterNames(endpoint.Parameters); err != nil {
		if f.logger != nil {
			f.logger.Errorf("Parameter name validation failed for %s_%s: %v", serviceName, endpoint.ID, err)
		}
	}

	// Create tool parameters from endpoint parameters
	var parameters []global.Parameter
	for _, param := range endpoint.Parameters {
		// Skip static parameters - they are not exposed to MCP
		if param.Static {
			if f.logger != nil {
				f.logger.Debugf("Skipping static parameter '%s' in %s_%s (will use default)",
					param.Name, serviceName, endpoint.ID)
			}
			continue
		}

		// Use MCP-compliant name (alias or sanitized)
		mcpName := MCPParameterName(&param)

		// Log the mapping if different from original
		if f.logger != nil && mcpName != param.Name {
			if param.Alias != "" {
				f.logger.Infof("Using parameter alias '%s' for '%s' in %s_%s",
					mcpName, param.Name, serviceName, endpoint.ID)
			} else {
				f.logger.Warningf("Auto-sanitized parameter '%s' to '%s' in %s_%s - consider adding explicit alias",
					param.Name, mcpName, serviceName, endpoint.ID)
			}
		}

		globalParam := global.Parameter{
			Name:        mcpName, // Use MCP-compliant name
			Description: param.Description,
			Required:    param.Required,
			Type:        string(param.Type),
			Items:       string(param.Items),
			Default:     param.Default,
			Examples:    param.Examples,
		}

		// Copy validation rules if present
		if param.Validation != nil {
			globalParam.Pattern = param.Validation.Pattern
			globalParam.Format = param.Validation.Format
			globalParam.Enum = param.Validation.Enum
			if param.Validation.MinLength != nil {
				globalParam.MinLength = param.Validation.MinLength
			}
			if param.Validation.MaxLength != nil {
				globalParam.MaxLength = param.Validation.MaxLength
			}
			if param.Validation.Minimum != nil {
				globalParam.Minimum = param.Validation.Minimum
			}
			if param.Validation.Maximum != nil {
				globalParam.Maximum = param.Validation.Maximum
			}
		}

		// Use enhanced description
		globalParam.Description = globalParam.EnhancedDescription()

		parameters = append(parameters, globalParam)
	}

	// Create the tool handler
	handler := f.createToolHandler(serviceName, service, endpoint)

	// Generate tool name by combining service and endpoint names
	toolName := fmt.Sprintf("%s_%s", serviceName, endpoint.ID)

	// Compute default hints based on HTTP method
	hints := global.ComputeDefaultHints(endpoint.Method)

	// Override with any explicitly configured hints
	if endpoint.Hints != nil {
		if endpoint.Hints.ReadOnly != nil {
			hints.ReadOnly = endpoint.Hints.ReadOnly
		}
		if endpoint.Hints.Destructive != nil {
			hints.Destructive = endpoint.Hints.Destructive
		}
		if endpoint.Hints.Idempotent != nil {
			hints.Idempotent = endpoint.Hints.Idempotent
		}
		if endpoint.Hints.OpenWorld != nil {
			hints.OpenWorld = endpoint.Hints.OpenWorld
		}
	}

	// Gate destructive tools when MCP_FUSION_ALLOW_DESTRUCTIVE is not enabled
	if hints.Destructive != nil && *hints.Destructive && !f.allowDestructive {
		originalHandler := handler
		handler = func(args map[string]interface{}) (string, error) {
			_ = originalHandler // preserve reference
			return "", fmt.Errorf("destructive operations are disabled (set MCP_FUSION_ALLOW_DESTRUCTIVE=true to allow)")
		}
	}

	return global.ToolDefinition{
		Name:        toolName,
		Description: fmt.Sprintf("%s: %s", service.Name, endpoint.Description),
		Parameters:  parameters,
		Handler:     handler,
		Hints:       &hints,
	}
}

// createToolHandler creates a handler function for a specific endpoint
func (f *Fusion) createToolHandler(_ string, service *ServiceConfig, endpoint *EndpointConfig) global.ToolHandler {
	httpHandler := NewHTTPHandler(f, service, endpoint)

	// Create a context-aware handler that the MCP server can detect and use
	contextHandler := &contextAwareHandler{
		httpHandler: httpHandler,
	}

	return contextHandler.Call
}

// createCommandToolDefinition creates a tool definition from command configuration
func (f *Fusion) createCommandToolDefinition(groupName string, commandGroup *CommandGroupConfig, command *CommandConfig) global.ToolDefinition {
	// Create tool parameters from command parameters (skip static ones)
	var parameters []global.Parameter
	for _, param := range command.Parameters {
		// Skip static parameters - they are not exposed to MCP
		if param.Static {
			if f.logger != nil {
				f.logger.Debugf("Skipping static parameter '%s' in command_%s (will use default)",
					param.Name, command.ID)
			}
			continue
		}

		// Skip control parameters that are always static
		if param.Location == ParameterLocationControl {
			// Only expose control parameters that are explicitly not static
			if param.Static {
				continue
			}
		}

		globalParam := global.Parameter{
			Name:        param.Name,
			Description: param.Description,
			Required:    param.Required,
			Type:        string(param.Type),
			Items:       string(param.Items),
			Default:     param.Default,
			Examples:    param.Examples,
		}

		// Copy validation rules if present
		if param.Validation != nil {
			globalParam.Pattern = param.Validation.Pattern
			globalParam.Format = param.Validation.Format
			globalParam.Enum = param.Validation.Enum
			if param.Validation.MinLength != nil {
				globalParam.MinLength = param.Validation.MinLength
			}
			if param.Validation.MaxLength != nil {
				globalParam.MaxLength = param.Validation.MaxLength
			}
			if param.Validation.Minimum != nil {
				globalParam.Minimum = param.Validation.Minimum
			}
			if param.Validation.Maximum != nil {
				globalParam.Maximum = param.Validation.Maximum
			}
		}

		// Use enhanced description
		globalParam.Description = globalParam.EnhancedDescription()

		parameters = append(parameters, globalParam)
	}

	// Create the tool handler
	handler := f.createCommandToolHandler(commandGroup, command)

	// Generate tool name: command_{id}
	toolName := fmt.Sprintf("command_%s", command.ID)

	return global.ToolDefinition{
		Name:        toolName,
		Description: command.Description,
		Parameters:  parameters,
		Handler:     handler,
	}
}

// createCommandToolHandler creates a handler for command execution
func (f *Fusion) createCommandToolHandler(commandGroup *CommandGroupConfig, command *CommandConfig) global.ToolHandler {
	return func(args map[string]interface{}) (string, error) {
		// Create command handler
		handler := NewCommandHandler(f, commandGroup, command)

		// Execute command
		ctx := context.Background()
		return handler.Handle(ctx, args)
	}
}

// contextAwareHandler holds the HTTP handler and provides both legacy and context-aware interfaces
type contextAwareHandler struct {
	httpHandler *HTTPHandler
}

// Call implements the legacy interface - extracts context from options if available
func (h *contextAwareHandler) Call(options map[string]any) (string, error) {
	ctx := context.Background()

	if ctxValue, exists := options["__mcp_context"]; exists {
		if contextFromMCP, ok := ctxValue.(context.Context); ok {
			ctx = contextFromMCP
			filteredOptions := make(map[string]any, len(options)-1)
			for k, v := range options {
				if k != "__mcp_context" {
					filteredOptions[k] = v
				}
			}
			return h.CallWithContext(ctx, filteredOptions)
		}
	}
	return h.CallWithContext(ctx, options)
}

// CallWithContext implements the context-aware interface that MCP server will detect
func (h *contextAwareHandler) CallWithContext(ctx context.Context, options map[string]any) (string, error) {
	result, err := h.httpHandler.Handle(ctx, options)
	if err != nil {
		// Check if it's a device code error
		if deviceCodeErr, ok := AsDeviceCodeError(err); ok {
			// For MCP, we return the device code message and expect the client to handle it
			// The client should display the message and call back when ready
			return deviceCodeErr.Error(), nil
		}
		return "", err
	}

	return result, nil
}

// RegisterResources implements the global.ResourceProvider interface
func (f *Fusion) RegisterResources() []global.ResourceDefinition {
	// Fusion doesn't provide static resources by default
	return []global.ResourceDefinition{}
}

// RegisterResourceTemplates implements the global.ResourceProvider interface
func (f *Fusion) RegisterResourceTemplates() []global.ResourceTemplateDefinition {
	// Fusion doesn't provide resource templates by default
	return []global.ResourceTemplateDefinition{}
}

// RegisterPrompts implements the global.PromptProvider interface
func (f *Fusion) RegisterPrompts() []global.PromptDefinition {
	// Fusion doesn't provide prompts by default
	return []global.PromptDefinition{}
}

// Validate validates the current configuration
func (f *Fusion) Validate() error {
	if f.config == nil {
		return NewConfigurationError("config", "", "no configuration loaded", nil)
	}

	return f.config.Validate()
}

// ReloadConfig reloads the configuration from the original file
func (f *Fusion) ReloadConfig() error {
	if f.config == nil || f.config.ConfigPath == "" {
		return NewConfigurationError("configPath", "", "no config path available for reload", nil)
	}

	newConfig, err := LoadConfigFromFile(f.config.ConfigPath)
	if err != nil {
		return NewConfigurationError("config", "", "failed to reload configuration", err)
	}

	// Update configuration
	f.config = newConfig
	f.config.Logger = f.logger
	// Legacy authManager reference removed
	f.config.HTTPClient = f.httpClient
	f.config.Cache = f.cache

	if f.logger != nil {
		f.logger.Info("Configuration reloaded successfully")
	}

	return nil
}

// ServiceNames returns a list of configured service names
func (f *Fusion) ServiceNames() []string {
	if f.config == nil {
		return []string{}
	}

	names := make([]string, 0, len(f.config.Services))
	for name := range f.config.Services {
		names = append(names, name)
	}
	return names
}

// Service returns a service configuration by name
func (f *Fusion) Service(name string) *ServiceConfig {
	if f.config == nil {
		return nil
	}

	return f.config.Services[name]
}

// HasService checks if a service is configured
func (f *Fusion) HasService(name string) bool {
	if f.config == nil {
		return false
	}

	_, exists := f.config.Services[name]
	return exists
}

// Endpoint returns an endpoint configuration by service and endpoint ID
func (f *Fusion) Endpoint(serviceName, endpointID string) *EndpointConfig {
	service := f.Service(serviceName)
	if service == nil {
		return nil
	}

	return service.EndpointByID(endpointID)
}

// Legacy authentication methods removed - use multi-tenant auth manager

// buildRequest constructs an HTTP request from endpoint configuration and user options
func (f *Fusion) buildRequest(ctx context.Context, serviceName string, service *ServiceConfig, endpoint *EndpointConfig, options map[string]any) (*http.Request, error) {
	if f.logger != nil {
		f.logger.Debugf("Building request for service %s, endpoint %s", serviceName, endpoint.ID)
	}

	// Start with the base URL and path
	requestURL := strings.TrimSuffix(service.BaseURL, "/") + "/" + strings.TrimPrefix(endpoint.Path, "/")
	if f.logger != nil {
		f.logger.Debugf("Initial request URL: %s", requestURL)
	}

	// Parse URL for modifications
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		if f.logger != nil {
			f.logger.Errorf("Failed to parse URL %s: %v", requestURL, err)
		}
		return nil, NewConfigurationError("path", serviceName, fmt.Sprintf("invalid URL: %s", requestURL), err)
	}

	// Prepare request body and query parameters
	queryParams := parsedURL.Query()
	var requestBody interface{}
	bodyParameters := make(map[string]interface{})
	pathParams := make(map[string]interface{})
	headerParams := make(map[string]interface{})

	if f.logger != nil {
		f.logger.Debugf("Processing %d parameters for endpoint %s", len(endpoint.Parameters), endpoint.ID)
	}

	// Process each parameter
	for _, param := range endpoint.Parameters {
		if f.logger != nil {
			f.logger.Debugf("Processing parameter: %s (type: %s, location: %s, required: %t)",
				param.Name, param.Type, param.Location, param.Required)
		}

		value, provided := options[param.Name]

		// Check if required parameter is missing
		if param.Required && !provided {
			if f.logger != nil {
				f.logger.Errorf("Required parameter missing: %s", param.Name)
			}
			return nil, NewValidationError(param.Name, nil, "required", "parameter is required")
		}

		// Use default value if not provided
		if !provided && param.Default != nil {
			value = param.Default
			provided = true
			if f.logger != nil {
				f.logger.Debugf("Using default value for parameter %s: %v", param.Name, value)
			}
		}

		// Skip optional parameters that weren't provided
		if !provided {
			if f.logger != nil {
				f.logger.Debugf("Skipping optional parameter: %s", param.Name)
			}
			continue
		}

		// Validate the parameter
		if f.logger != nil {
			f.logger.Debugf("Validating parameter %s with value: %v", param.Name, value)
		}
		if err := f.validateParameter(&param, value); err != nil {
			if f.logger != nil {
				f.logger.Errorf("Parameter validation failed for %s: %v", param.Name, err)
			}
			return nil, err
		}

		// Transform the parameter if needed
		transformedValue, err := f.transformParameter(&param, value)
		if err != nil {
			if f.logger != nil {
				f.logger.Errorf("Parameter transformation failed for %s: %v", param.Name, err)
			}
			return nil, err
		}

		if transformedValue != value && f.logger != nil {
			f.logger.Debugf("Parameter %s transformed: %v -> %v", param.Name, value, transformedValue)
		}

		// Apply parameter to appropriate location
		switch param.Location {
		case ParameterLocationPath:
			pathParams[param.TransformedParameterName()] = transformedValue
			// Replace path parameter
			placeholder := "{" + param.TransformedParameterName() + "}"
			parsedURL.Path = strings.ReplaceAll(parsedURL.Path, placeholder, fmt.Sprintf("%v", transformedValue))
			if f.logger != nil {
				f.logger.Debugf("Applied path parameter %s: %v", param.Name, transformedValue)
			}
		case ParameterLocationQuery:
			queryParams.Set(param.TransformedParameterName(), fmt.Sprintf("%v", transformedValue))
			if f.logger != nil {
				f.logger.Debugf("Applied query parameter %s: %v", param.Name, transformedValue)
			}
		case ParameterLocationHeader:
			headerParams[param.TransformedParameterName()] = transformedValue
			if f.logger != nil {
				f.logger.Debugf("Prepared header parameter %s: %v", param.Name, transformedValue)
			}
		case ParameterLocationBody:
			bodyParameters[param.TransformedParameterName()] = transformedValue
			if f.logger != nil {
				f.logger.Debugf("Applied body parameter %s: %v", param.Name, transformedValue)
			}
		}
	}

	// Set query parameters
	parsedURL.RawQuery = queryParams.Encode()
	if f.logger != nil && len(queryParams) > 0 {
		f.logger.Debugf("Final query string: %s", parsedURL.RawQuery)
	}

	// Prepare request body
	var bodyReader io.Reader
	if len(bodyParameters) > 0 {
		switch strings.ToUpper(endpoint.Method) {
		case "POST", "PUT", "PATCH":
			if f.logger != nil {
				f.logger.Debugf("Marshaling request body with %d parameters", len(bodyParameters))
			}
			bodyData, err := json.Marshal(bodyParameters)
			if err != nil {
				if f.logger != nil {
					f.logger.Errorf("Failed to marshal request body: %v", err)
				}
				return nil, NewTransformationError("request", "body", "json.Marshal", bodyParameters, "failed to marshal request body", err)
			}
			bodyReader = bytes.NewReader(bodyData)
			requestBody = bodyParameters
			if f.logger != nil {
				sanitizedBody := f.sanitizeRequestBody(bodyData)
				f.logger.Debugf("Request body: %s", sanitizedBody)
			}
		default:
			if f.logger != nil {
				f.logger.Warningf("Body parameters provided for %s request but will be ignored", endpoint.Method)
			}
		}
	}

	// Create the HTTP request
	if f.logger != nil {
		f.logger.Debugf("Creating HTTP request: %s %s", strings.ToUpper(endpoint.Method), parsedURL.String())
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(endpoint.Method), parsedURL.String(), bodyReader)
	if err != nil {
		if f.logger != nil {
			f.logger.Errorf("Failed to create HTTP request: %v", err)
		}
		return nil, NewNetworkError(parsedURL.String(), endpoint.Method, "failed to create HTTP request", err, false, false)
	}

	// Set content type for body requests
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
		if f.logger != nil {
			f.logger.Debug("Set Content-Type: application/json")
		}
	}

	// Set header parameters
	headerCount := 0
	for _, param := range endpoint.Parameters {
		if param.Location == ParameterLocationHeader {
			if value, exists := options[param.Name]; exists {
				headerName := param.TransformedParameterName()
				headerValue := fmt.Sprintf("%v", value)
				req.Header.Set(headerName, headerValue)
				headerCount++
				if f.logger != nil {
					f.logger.Debugf("Set header %s: %s", headerName, headerValue)
				}
			}
		}
	}

	if f.logger != nil {
		f.logger.Infof("Successfully built request: %s %s (path params: %d, query params: %d, headers: %d, body params: %d)",
			req.Method, req.URL.String(), len(pathParams), len(queryParams), headerCount, len(bodyParameters))
	}

	return req, nil
}

// processResponse processes the HTTP response according to endpoint configuration
func (f *Fusion) processResponse(resp *http.Response, endpoint *EndpointConfig, serviceName string) (string, error) {
	if f.logger != nil {
		f.logger.Debugf("Processing response for service %s, endpoint %s (status: %s)", serviceName, endpoint.ID, resp.Status)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)

		if f.logger != nil {
			sanitizedResponse := f.sanitizeResponseBody(bodyBytes, 500)
			f.logger.Errorf("HTTP error response for service %s, endpoint %s: %s - Body: %s",
				serviceName, endpoint.ID, resp.Status, sanitizedResponse)
		}

		retryable := resp.StatusCode >= 500 || resp.StatusCode == 429 // Server errors and rate limiting
		return "", NewAPIError(serviceName, endpoint.ID, resp.StatusCode, resp.Status, bodyStr, retryable)
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		if f.logger != nil {
			f.logger.Errorf("Failed to read response body for service %s, endpoint %s: %v", serviceName, endpoint.ID, err)
		}
		return "", NewNetworkError("", "", "failed to read response body", err, false, false)
	}

	if f.logger != nil {
		sanitizedResponse := f.sanitizeResponseBody(bodyBytes, 1000)
		f.logger.Debugf("Response body for service %s, endpoint %s: %s", serviceName, endpoint.ID, sanitizedResponse)
	}

	// Process based on response type
	switch endpoint.Response.Type {
	case ResponseTypeJSON:
		if f.logger != nil {
			f.logger.Debugf("Processing JSON response for service %s, endpoint %s", serviceName, endpoint.ID)
		}
		return f.processJSONResponse(bodyBytes, endpoint, serviceName)
	case ResponseTypeText:
		if f.logger != nil {
			f.logger.Debugf("Processing text response for service %s, endpoint %s", serviceName, endpoint.ID)
		}
		return string(bodyBytes), nil
	case ResponseTypeBinary:
		if f.logger != nil {
			f.logger.Debugf("Processing binary response for service %s, endpoint %s (%d bytes)",
				serviceName, endpoint.ID, len(bodyBytes))
		}
		// For binary responses, return base64 encoded data or metadata
		return fmt.Sprintf("Binary response received (%d bytes, Content-Type: %s)",
			len(bodyBytes), resp.Header.Get("Content-Type")), nil
	default:
		if f.logger != nil {
			f.logger.Debugf("Processing response as text (default) for service %s, endpoint %s", serviceName, endpoint.ID)
		}
		// Default to treating as text
		return string(bodyBytes), nil
	}
}

// processJSONResponse processes JSON responses with optional transformation
func (f *Fusion) processJSONResponse(bodyBytes []byte, endpoint *EndpointConfig, _ string) (string, error) {
	// Parse JSON
	var responseData interface{}
	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return "", NewTransformationError("response", "json", "json.Unmarshal", string(bodyBytes), "failed to parse JSON response", err)
	}

	// Apply transformation if specified
	if endpoint.Response.Transform != "" {
		transformed, err := f.applyResponseTransform(responseData, endpoint.Response.Transform)
		if err != nil {
			return "", err
		}
		responseData = transformed
	}

	// Convert back to JSON string for consistent output
	result, err := json.MarshalIndent(responseData, "", "  ")
	if err != nil {
		return "", NewTransformationError("response", "json", "json.MarshalIndent", responseData, "failed to marshal response", err)
	}

	return string(result), nil
}

// validateParameter validates a parameter value according to its configuration
func (f *Fusion) validateParameter(param *ParameterConfig, value interface{}) error {
	if f.logger != nil {
		f.logger.Debugf("Validating parameter %s (type: %s, value: %v)", param.Name, param.Type, value)
	}

	// Type checking should always happen
	switch param.Type {
	case ParameterTypeString:
		strValue, ok := value.(string)
		if !ok {
			// Try to convert other types to string as a recovery mechanism
			if convertedStr := fmt.Sprintf("%v", value); f.logger != nil {
				f.logger.Warningf("Parameter %s: converting %T to string: %v -> %s", param.Name, value, value, convertedStr)
			}
			strValue = fmt.Sprintf("%v", value)
		}

		// Additional validation if config exists
		if param.Validation != nil {
			validation := param.Validation

			// Length validation
			if !validation.IsValidLength(strValue) {
				message := fmt.Sprintf("string length must be between %d and %d (actual: %d)",
					validation.MinLength, validation.MaxLength, len(strValue))
				if f.logger != nil {
					f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
				}
				return NewValidationError(param.Name, value, "length", message)
			}

			// Pattern validation
			if !validation.MatchesPattern(strValue) {
				message := fmt.Sprintf("string must match pattern: %s", validation.Pattern)
				if f.logger != nil {
					f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
				}
				return NewValidationError(param.Name, value, "pattern", message)
			}
		}

	case ParameterTypeNumber, ParameterTypeInteger:
		// Accept both int and float with better error handling
		var numValue float64
		var converted bool

		switch v := value.(type) {
		case float64:
			numValue = v
			converted = true
		case float32:
			numValue = float64(v)
			converted = true
		case int:
			numValue = float64(v)
			converted = true
		case int64:
			numValue = float64(v)
			converted = true
		case int32:
			numValue = float64(v)
			converted = true
		case string:
			// Try to parse string as number for recovery
			if parsedValue, err := strconv.ParseFloat(v, 64); err == nil {
				numValue = parsedValue
				converted = true
				if f.logger != nil {
					f.logger.Warningf("Parameter %s: converted string to number: %s -> %f", param.Name, v, numValue)
				}
			}
		}

		if !converted {
			message := fmt.Sprintf("parameter must be a number (received %T: %v)", value, value)
			if f.logger != nil {
				f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
			}
			return NewValidationError(param.Name, value, "type", message)
		}

		// For integer type, reject decimal values
		if param.Type == ParameterTypeInteger && numValue != float64(int64(numValue)) {
			message := fmt.Sprintf("parameter must be an integer, not a decimal (received: %v)", numValue)
			if f.logger != nil {
				f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
			}
			return NewValidationError(param.Name, value, "type", message)
		}

	case ParameterTypeBoolean:
		if _, ok := value.(bool); !ok {
			// Try to convert string to boolean for recovery
			if strValue, isString := value.(string); isString {
				if boolValue, err := strconv.ParseBool(strValue); err == nil {
					if f.logger != nil {
						f.logger.Warningf("Parameter %s: converted string to boolean: %s -> %t", param.Name, strValue, boolValue)
					}
				} else {
					message := fmt.Sprintf("parameter must be a boolean (received %T: %v)", value, value)
					if f.logger != nil {
						f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
					}
					return NewValidationError(param.Name, value, "type", message)
				}
			} else {
				message := fmt.Sprintf("parameter must be a boolean (received %T: %v)", value, value)
				if f.logger != nil {
					f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
				}
				return NewValidationError(param.Name, value, "type", message)
			}
		}

	case ParameterTypeArray:
		if reflect.TypeOf(value).Kind() != reflect.Slice {
			message := fmt.Sprintf("parameter must be an array (received %T: %v)", value, value)
			if f.logger != nil {
				f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
			}
			return NewValidationError(param.Name, value, "type", message)
		}

	case ParameterTypeObject:
		if reflect.TypeOf(value).Kind() != reflect.Map {
			message := fmt.Sprintf("parameter must be an object (received %T: %v)", value, value)
			if f.logger != nil {
				f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
			}
			return NewValidationError(param.Name, value, "type", message)
		}
	}

	// Enum validation if config exists
	if param.Validation != nil && !param.Validation.IsValidEnumValue(value) {
		message := fmt.Sprintf("value must be one of: %v (received: %v)", param.Validation.Enum, value)
		if f.logger != nil {
			f.logger.Errorf("Parameter %s validation failed: %s", param.Name, message)
		}
		return NewValidationError(param.Name, value, "enum", message)
	}

	if f.logger != nil {
		f.logger.Debugf("Parameter %s validation successful", param.Name)
	}

	return nil
}

// transformParameter applies parameter transformation if configured
func (f *Fusion) transformParameter(param *ParameterConfig, value interface{}) (interface{}, error) {
	if param.Transform == nil || param.Transform.Expression == "" {
		return value, nil
	}

	if f.logger != nil {
		f.logger.Debugf("Applying transformation to parameter %s: %s", param.Name, param.Transform.Expression)
	}

	// For now, implement basic transformations
	// This can be extended with a full expression evaluator
	expression := param.Transform.Expression

	switch expression {
	case "toString":
		result := fmt.Sprintf("%v", value)
		if f.logger != nil {
			f.logger.Debugf("Parameter %s toString transformation: %v -> %s", param.Name, value, result)
		}
		return result, nil

	case "toInt":
		switch v := value.(type) {
		case string:
			intValue, err := strconv.Atoi(v)
			if err != nil {
				if f.logger != nil {
					f.logger.Errorf("Parameter %s toInt transformation failed: %v", param.Name, err)
				}
				return nil, NewTransformationError("parameter", param.Name, expression, value, "failed to convert string to integer", err)
			}
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toInt transformation: %s -> %d", param.Name, v, intValue)
			}
			return intValue, nil
		case float64:
			intValue := int(v)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toInt transformation: %f -> %d", param.Name, v, intValue)
			}
			return intValue, nil
		case float32:
			intValue := int(v)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toInt transformation: %f -> %d", param.Name, v, intValue)
			}
			return intValue, nil
		default:
			if f.logger != nil {
				f.logger.Warningf("Parameter %s toInt transformation: unsupported type %T, returning unchanged", param.Name, value)
			}
			return value, nil
		}

	case "toFloat":
		switch v := value.(type) {
		case string:
			floatValue, err := strconv.ParseFloat(v, 64)
			if err != nil {
				if f.logger != nil {
					f.logger.Errorf("Parameter %s toFloat transformation failed: %v", param.Name, err)
				}
				return nil, NewTransformationError("parameter", param.Name, expression, value, "failed to convert string to float", err)
			}
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toFloat transformation: %s -> %f", param.Name, v, floatValue)
			}
			return floatValue, nil
		case int:
			floatValue := float64(v)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toFloat transformation: %d -> %f", param.Name, v, floatValue)
			}
			return floatValue, nil
		case int64:
			floatValue := float64(v)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toFloat transformation: %d -> %f", param.Name, v, floatValue)
			}
			return floatValue, nil
		default:
			if f.logger != nil {
				f.logger.Warningf("Parameter %s toFloat transformation: unsupported type %T, returning unchanged", param.Name, value)
			}
			return value, nil
		}

	case "toLowerCase":
		if strValue, ok := value.(string); ok {
			result := strings.ToLower(strValue)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toLowerCase transformation: %s -> %s", param.Name, strValue, result)
			}
			return result, nil
		}
		if f.logger != nil {
			f.logger.Warningf("Parameter %s toLowerCase transformation: value is not a string (%T), returning unchanged", param.Name, value)
		}
		return value, nil

	case "toUpperCase":
		if strValue, ok := value.(string); ok {
			result := strings.ToUpper(strValue)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s toUpperCase transformation: %s -> %s", param.Name, strValue, result)
			}
			return result, nil
		}
		if f.logger != nil {
			f.logger.Warningf("Parameter %s toUpperCase transformation: value is not a string (%T), returning unchanged", param.Name, value)
		}
		return value, nil

	case "trim":
		if strValue, ok := value.(string); ok {
			result := strings.TrimSpace(strValue)
			if f.logger != nil {
				f.logger.Debugf("Parameter %s trim transformation: '%s' -> '%s'", param.Name, strValue, result)
			}
			return result, nil
		}
		if f.logger != nil {
			f.logger.Warningf("Parameter %s trim transformation: value is not a string (%T), returning unchanged", param.Name, value)
		}
		return value, nil

	default:
		// If we don't recognize the expression, just return the value unchanged
		if f.logger != nil {
			f.logger.Warningf("Parameter %s: unknown transformation expression '%s', returning value unchanged", param.Name, expression)
		}
		return value, nil
	}
}

// applyResponseTransform applies transformation to response data
func (f *Fusion) applyResponseTransform(data interface{}, transform string) (interface{}, error) {
	// For now, implement basic JSON path extraction
	// This can be extended with a full transformation engine

	if strings.HasPrefix(transform, "$.") {
		// Simple JSON path extraction
		return f.extractJSONPath(data, transform)
	}

	// If we don't recognize the transform, return data unchanged
	return data, nil
}

// extractJSONPath performs simple JSON path extraction
func (f *Fusion) extractJSONPath(data interface{}, path string) (interface{}, error) {
	// Remove the leading "$."
	path = strings.TrimPrefix(path, "$.")

	// Split path into parts
	parts := strings.Split(path, ".")

	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil, NewTransformationError("response", "json_path", path, data, fmt.Sprintf("cannot navigate to '%s' in non-object", part), nil)
		}

		if current == nil {
			return nil, NewTransformationError("response", "json_path", path, data, fmt.Sprintf("path '%s' not found", part), nil)
		}
	}

	return current, nil
}

// sanitizeRequestBody removes or masks sensitive information from request body for logging
func (f *Fusion) sanitizeRequestBody(body []byte) string {
	// Try to parse as JSON first
	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		// If not JSON, check if it contains sensitive keywords and truncate/mask if needed
		bodyStr := string(body)
		if f.containsSensitiveData(bodyStr) {
			return "[REDACTED - contains sensitive data]"
		}
		// Truncate long non-JSON bodies
		if len(bodyStr) > 500 {
			return bodyStr[:500] + "...[truncated]"
		}
		return bodyStr
	}

	// Sanitize JSON data
	sanitized := f.sanitizeJSONData(jsonData)
	sanitizedBytes, _ := json.Marshal(sanitized)
	return string(sanitizedBytes)
}

// sanitizeJSONData recursively sanitizes JSON data by masking sensitive fields
func (f *Fusion) sanitizeJSONData(data interface{}) interface{} {
	sensitiveFields := map[string]bool{
		"password":      true,
		"token":         true,
		"access_token":  true,
		"refresh_token": true,
		"api_key":       true,
		"apikey":        true,
		"secret":        true,
		"key":           true,
		"auth":          true,
		"authorization": true,
		"credential":    true,
		"credentials":   true,
	}

	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			lowerKey := strings.ToLower(key)
			if sensitiveFields[lowerKey] {
				result[key] = "[REDACTED]"
			} else {
				result[key] = f.sanitizeJSONData(value)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = f.sanitizeJSONData(item)
		}
		return result
	default:
		return v
	}
}

// containsSensitiveData checks if a string contains sensitive keywords
func (f *Fusion) containsSensitiveData(data string) bool {
	sensitiveKeywords := []string{
		"password", "token", "secret", "key", "auth", "credential",
		"bearer", "oauth", "jwt", "api_key", "apikey",
	}

	lowerData := strings.ToLower(data)
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lowerData, keyword) {
			return true
		}
	}
	return false
}

// sanitizeResponseBody removes or masks sensitive information from response body for logging
func (f *Fusion) sanitizeResponseBody(body []byte, maxLength int) string {
	if maxLength <= 0 {
		maxLength = 1000 // Default max length
	}

	// Try to parse as JSON first
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		// If not JSON, check for sensitive data and truncate
		bodyStr := string(body)
		if f.containsSensitiveData(bodyStr) {
			return "[REDACTED - response contains sensitive data]"
		}
		// Truncate long responses
		if len(bodyStr) > maxLength {
			return bodyStr[:maxLength] + fmt.Sprintf("...[truncated, %d more bytes]", len(bodyStr)-maxLength)
		}
		return bodyStr
	}

	// Sanitize JSON data
	sanitized := f.sanitizeJSONData(jsonData)
	sanitizedBytes, _ := json.Marshal(sanitized)
	sanitizedStr := string(sanitizedBytes)

	// Truncate if too long
	if len(sanitizedStr) > maxLength {
		return sanitizedStr[:maxLength] + fmt.Sprintf("...[truncated, %d more bytes]", len(sanitizedStr)-maxLength)
	}

	return sanitizedStr
}

// getOrCreateCircuitBreaker gets or creates a circuit breaker for a service
func (f *Fusion) getOrCreateCircuitBreaker(serviceName string, config *CircuitBreakerConfig) *CircuitBreaker {
	f.circuitBreakersMutex.RLock()
	if cb, exists := f.circuitBreakers[serviceName]; exists {
		f.circuitBreakersMutex.RUnlock()
		return cb
	}
	f.circuitBreakersMutex.RUnlock()

	// Create new circuit breaker
	f.circuitBreakersMutex.Lock()
	defer f.circuitBreakersMutex.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := f.circuitBreakers[serviceName]; exists {
		return cb
	}

	if f.logger != nil {
		f.logger.Infof("Creating circuit breaker for service '%s'", serviceName)
	}

	cb := NewCircuitBreaker(config, WithLogger(f.logger))
	f.circuitBreakers[serviceName] = cb
	return cb
}

// CircuitBreakerMetrics returns circuit breaker metrics for a service
func (f *Fusion) CircuitBreakerMetrics(serviceName string) *CircuitBreakerMetrics {
	f.circuitBreakersMutex.RLock()
	defer f.circuitBreakersMutex.RUnlock()

	if cb, exists := f.circuitBreakers[serviceName]; exists {
		metrics := cb.Metrics()
		return &metrics
	}
	return nil
}

// AllCircuitBreakerMetrics returns circuit breaker metrics for all services
func (f *Fusion) AllCircuitBreakerMetrics() map[string]*CircuitBreakerMetrics {
	f.circuitBreakersMutex.RLock()
	defer f.circuitBreakersMutex.RUnlock()

	result := make(map[string]*CircuitBreakerMetrics)
	for serviceName, cb := range f.circuitBreakers {
		metrics := cb.Metrics()
		result[serviceName] = &metrics
	}
	return result
}

// Metrics returns metrics for all services
func (f *Fusion) Metrics() map[string]*ServiceMetrics {
	if f.metricsCollector == nil {
		return nil
	}
	return f.metricsCollector.AllMetrics()
}

// ServiceMetrics returns metrics for a specific service
func (f *Fusion) ServiceMetrics(serviceName string) *ServiceMetrics {
	if f.metricsCollector == nil {
		return nil
	}
	return f.metricsCollector.ServiceMetrics(serviceName)
}

// GlobalMetrics returns global system metrics
func (f *Fusion) GlobalMetrics() *GlobalMetrics {
	if f.metricsCollector == nil {
		return nil
	}
	metrics := f.metricsCollector.GlobalMetrics()
	return &metrics
}

// ResetMetrics resets all collected metrics
func (f *Fusion) ResetMetrics() {
	if f.metricsCollector != nil {
		f.metricsCollector.Reset()
	}
}

// StartMetricsLogging starts periodic logging of metrics
func (f *Fusion) StartMetricsLogging(ctx context.Context, interval time.Duration) {
	if f.metricsCollector != nil {
		f.metricsCollector.StartPeriodicLogging(ctx, interval)
	}
}

// startConnectionHealthManagement initializes periodic connection pool cleanup
func (f *Fusion) startConnectionHealthManagement() {
	// Start periodic connection cleanup every 5 minutes
	f.connectionCleanupTicker = time.NewTicker(5 * time.Minute)

	go func() {
		defer f.connectionCleanupTicker.Stop()

		for {
			select {
			case <-f.connectionCleanupTicker.C:
				f.cleanupConnections()
				if f.logger != nil {
					f.logger.Debugf("Goroutine count: %d", runtime.NumGoroutine())
				}
			case <-f.shutdownChan:
				if f.logger != nil {
					f.logger.Debug("Connection health management shutting down")
				}
				return
			}
		}
	}()

	if f.logger != nil {
		f.logger.Debug("Connection health management started")
	}
}

// cleanupConnections forces cleanup of idle connections in the HTTP client's transport
func (f *Fusion) cleanupConnections() {
	if f.httpClient != nil && f.httpClient.Transport != nil {
		if transport, ok := f.httpClient.Transport.(*http.Transport); ok {
			// Close idle connections to prevent stale connection reuse
			transport.CloseIdleConnections()

			if f.logger != nil {
				f.logger.Debug("Cleaned up idle HTTP connections")
			}
		}
	}
}

// Shutdown gracefully shuts down the Fusion instance and cleans up resources
func (f *Fusion) Shutdown() {
	f.shutdownOnce.Do(func() {
		if f.logger != nil {
			f.logger.Info("Shutting down Fusion instance")
		}

		// Signal shutdown to background goroutines
		close(f.shutdownChan)

		// Final connection cleanup
		f.cleanupConnections()

		if f.logger != nil {
			f.logger.Info("Fusion instance shutdown completed")
		}
	})
}

// ForceConnectionCleanup manually triggers connection pool cleanup
// This can be called when connection issues are detected
func (f *Fusion) ForceConnectionCleanup() {
	if f.logger != nil {
		f.logger.Info("Forcing connection pool cleanup")
	}
	f.cleanupConnections()
}

// circuitBreakerSourceAdapter adapts Fusion's internal CircuitBreakerMetrics to
// the health.CircuitBreakerSource interface without exposing internal types.
type circuitBreakerSourceAdapter struct {
	fusion *Fusion
}

// AllCircuitBreakerMetrics implements health.CircuitBreakerSource.
func (a *circuitBreakerSourceAdapter) AllCircuitBreakerMetrics() map[string]health.CircuitBreakerInfo {
	raw := a.fusion.AllCircuitBreakerMetrics()
	result := make(map[string]health.CircuitBreakerInfo, len(raw))
	for name, m := range raw {
		result[name] = health.CircuitBreakerInfo{
			State:  strings.ToLower(m.State.String()),
			IsOpen: m.State == CircuitBreakerOpen,
		}
	}
	return result
}

// CircuitBreakerSource returns a health.CircuitBreakerSource that exposes the
// circuit-breaker state of all services managed by this Fusion instance.
func (f *Fusion) CircuitBreakerSource() health.CircuitBreakerSource {
	return &circuitBreakerSourceAdapter{fusion: f}
}
