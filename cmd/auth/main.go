/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/cmd/auth/config"
	"github.com/PivotLLM/MCPFusion/cmd/auth/debug"
	"github.com/PivotLLM/MCPFusion/cmd/auth/mcp"
	"github.com/PivotLLM/MCPFusion/cmd/auth/providers"
	"github.com/PivotLLM/MCPFusion/cmd/auth/providers/google"
)

const (
	defaultTimeout = 10 * time.Minute
	version        = "1.0.0"
)

type cliFlags struct {
	service   string
	fusionURL string
	token     string
	verbose   bool
	debug     bool
	version   bool
	list      bool
}

func main() {
	var flags cliFlags
	parseFlags(&flags)

	// Set global debug flag
	debug.Debug = flags.debug

	if flags.version {
		fmt.Printf("fusion-auth version %s\n", version)
		return
	}

	// Initialize provider registry
	registry := providers.NewProviderRegistry()
	if err := registerProviders(registry); err != nil {
		log.Fatalf("Failed to register providers: %v", err)
	}

	if flags.list {
		listProviders(registry)
		return
	}

	// Validate required flags
	if err := validateFlags(&flags, registry); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create configuration
	cfg, err := createConfiguration(&flags, registry)
	if err != nil {
		log.Fatalf("Failed to create configuration: %v", err)
	}

	// Execute OAuth flow
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if err := executeOAuthFlow(ctx, cfg, &flags, registry); err != nil {
		log.Fatalf("OAuth flow failed: %v", err)
	}
}

func parseFlags(flags *cliFlags) {
	flag.StringVar(&flags.service, "service", "", "OAuth service provider (e.g., google, github, dropbox)")
	flag.StringVar(&flags.fusionURL, "fusion", "", "MCPFusion server URL (e.g., http://10.0.0.1:8888)")
	flag.StringVar(&flags.token, "token", "", "MCPFusion API token for authentication")
	flag.BoolVar(&flags.verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&flags.debug, "debug", false, "Enable debug logging (includes HTTP request/response details)")
	flag.BoolVar(&flags.version, "version", false, "Show version information")
	flag.BoolVar(&flags.list, "list", false, "List available OAuth providers")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "fusion-auth is a generic authentication helper for MCPFusion.\n")
		_, _ = fmt.Fprintf(os.Stderr, "Example:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  %s -service google -fusion http://10.0.0.1:8080 -token abc123\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  %s -list\n\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()
}

func registerProviders(registry *providers.ProviderRegistry) error {

	// Register Google OAuth provider
	googleProvider := google.NewProvider()
	if err := registry.Register(googleProvider); err != nil {
		return fmt.Errorf("failed to register Google provider: %w", err)
	}

	// Additional providers can be registered here
	// dropboxProvider := dropbox.NewProvider()
	// registry.Register(dropboxProvider)

	return nil
}

func listProviders(registry *providers.ProviderRegistry) {
	p := registry.GetAvailableProviders()

	fmt.Println("Available Providers:")
	fmt.Println("====================")

	for name, info := range p {
		fmt.Printf("\nService: %s\n", name)
		fmt.Printf("  Display Name: %s\n", info.DisplayName)
		fmt.Printf("  Device Flow: %v\n", info.SupportsDeviceFlow)
		fmt.Printf("  Auth Code Flow: %v\n", info.SupportsAuthCode)
		fmt.Printf("  Default Scopes: %v\n", info.DefaultScopes)
	}
}

func validateFlags(flags *cliFlags, registry *providers.ProviderRegistry) error {
	if flags.service == "" {
		return fmt.Errorf("service is required (use -list to see available services)")
	}

	if flags.fusionURL == "" {
		return fmt.Errorf("fusion URL is required")
	}

	if flags.token == "" {
		return fmt.Errorf("MCPFusion API token is required")
	}

	// Validate service exists
	_, err := registry.GetProvider(flags.service)
	if err != nil {
		return fmt.Errorf("unsupported service '%s': %w", flags.service, err)
	}

	return nil
}

func createConfiguration(flags *cliFlags, registry *providers.ProviderRegistry) (*config.Config, error) {
	cfg := &config.Config{
		Service:   flags.service,
		FusionURL: flags.fusionURL,
		APIToken:  flags.token,
		Verbose:   flags.verbose,
		Timeout:   defaultTimeout,
	}

	// Load default service configurations (hardcoded)
	cfg.LoadServiceDefaults()

	// Configuration comes from hardcoded defaults only

	// Get provider and validate configuration
	provider, err := registry.GetProvider(flags.service)
	if err != nil {
		return nil, err
	}

	// Create service config for validation using merged configuration
	mergedConfig := cfg.MergeServiceConfig(flags.service)
	serviceConfig := &providers.ServiceConfig{
		ServiceName:  flags.service,
		ClientID:     mergedConfig.ClientID,
		ClientSecret: mergedConfig.ClientSecret,
		TenantID:     mergedConfig.TenantID,
		Scopes:       strings.Join(provider.GetRequiredScopes(), " "), // Use provider's fixed scopes
	}

	if err := provider.ValidateConfiguration(serviceConfig); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

func executeOAuthFlow(ctx context.Context, cfg *config.Config, flags *cliFlags, registry *providers.ProviderRegistry) error {
	provider, err := registry.GetProvider(cfg.Service)
	if err != nil {
		return err
	}

	if flags.verbose {
		log.Printf("Starting OAuth flow for service: %s", provider.GetDisplayName())
		log.Printf("MCPFusion URL: %s", cfg.FusionURL)
	}

	// Create OAuth flow executor
	executor := &OAuthFlowExecutor{
		Provider:  provider,
		Config:    cfg,
		Registry:  registry,
		MCPClient: mcp.NewClient(cfg.FusionURL, cfg.APIToken),
		Verbose:   flags.verbose,
	}

	// Execute the OAuth flow based on what the provider supports
	// Prefer device flow if available, otherwise use auth code flow
	if provider.SupportsDeviceFlow() {
		return executor.ExecuteDeviceFlow(ctx)
	} else if provider.SupportsAuthorizationCode() {
		return executor.ExecuteAuthCodeFlow(ctx)
	} else {
		return fmt.Errorf("provider '%s' does not support any OAuth flow", cfg.Service)
	}
}

// OAuthFlowExecutor handles the execution of OAuth flows
type OAuthFlowExecutor struct {
	Provider  providers.OAuthProvider
	Config    *config.Config
	Registry  *providers.ProviderRegistry
	MCPClient *mcp.Client
	Verbose   bool
}

// ExecuteDeviceFlow implements the OAuth device flow
func (e *OAuthFlowExecutor) ExecuteDeviceFlow(_ context.Context) error {
	if e.Verbose {
		log.Println("Initiating OAuth device flow...")
	}

	// This would implement the device flow logic
	// For now, returning a placeholder
	return fmt.Errorf("device flow implementation pending")
}

// ExecuteAuthCodeFlow implements the OAuth authorization code flow
func (e *OAuthFlowExecutor) ExecuteAuthCodeFlow(ctx context.Context) error {
	if e.Verbose {
		log.Println("Initiating OAuth authorization code flow...")
	}

	// Merge service configuration
	serviceConfig := e.Config.MergeServiceConfig(e.Config.Service)
	providerConfig := &providers.ServiceConfig{
		ServiceName:  e.Config.Service,
		ClientID:     serviceConfig.ClientID,
		ClientSecret: serviceConfig.ClientSecret,
		TenantID:     serviceConfig.TenantID,
		Scopes:       serviceConfig.Scope, // Use scopes from configuration
	}

	// Generate secure state parameter for CSRF protection
	state, err := generateSecureState()
	if err != nil {
		return fmt.Errorf("failed to generate secure state: %w", err)
	}

	// Generate PKCE parameters for additional security
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Start local HTTP server to receive callback
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start local server: %w", err)
	}
	defer func(listener net.Listener) {
		_ = listener.Close()
	}(listener)

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	if e.Verbose {
		log.Printf("Started local callback server on port %d", port)
	}

	// Build authorization URL
	authURL, err := e.buildAuthorizationURL(providerConfig, redirectURI, state, codeChallenge)
	if err != nil {
		return fmt.Errorf("failed to build authorization URL: %w", err)
	}

	// Start HTTP server in background
	resultChan := make(chan authResult, 1)
	server := &http.Server{
		Handler: e.createCallbackHandler(state, codeVerifier, providerConfig, resultChan),
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			resultChan <- authResult{Error: fmt.Errorf("server error: %w", err)}
		}
	}()

	// Launch browser
	fmt.Printf("Opening browser for OAuth authorization...\n")
	fmt.Printf("If the browser doesn't open automatically, please visit:\n%s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		if e.Verbose {
			log.Printf("Failed to open browser automatically: %v", err)
		}
		fmt.Printf("Please manually open the URL above in your browser.\n")
	}

	// Wait for callback or timeout
	select {
	case result := <-resultChan:
		// Shutdown server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)

		if result.Error != nil {
			return result.Error
		}

		// Exchange authorization code for tokens
		tokenInfo, err := e.exchangeCodeForTokens(result.Code, redirectURI, codeVerifier, providerConfig)
		if err != nil {
			return fmt.Errorf("failed to exchange code for tokens: %w", err)
		}

		// Verify tokens by getting user info
		userInfo, err := e.Provider.GetUserInfo(ctx, tokenInfo)
		if err != nil {
			return fmt.Errorf("failed to get user info: %w", err)
		}

		if e.Verbose {
			log.Printf("Successfully authenticated user: %s (%s)", userInfo.Name, userInfo.Email)
		}

		// Store tokens in MCPFusion
		_, err = e.MCPClient.StoreTokens(ctx, e.Config.Service, tokenInfo.AccessToken, tokenInfo.RefreshToken)
		if err != nil {
			return fmt.Errorf("failed to store tokens in MCPFusion: %w", err)
		}

		// Send success notification
		if err := e.MCPClient.NotifySuccess(ctx, e.Config.Service, userInfo); err != nil {
			if e.Verbose {
				log.Printf("Warning: failed to send success notification: %v", err)
			}
		}

		fmt.Printf("\n✓ OAuth authentication successful!\n")
		fmt.Printf("✓ Tokens stored in MCPFusion\n")
		fmt.Printf("✓ Authenticated as: %s (%s)\n", userInfo.Name, userInfo.Email)

		return nil

	case <-ctx.Done():
		// Shutdown server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)

		return fmt.Errorf("OAuth flow timed out")
	}
}

// authResult holds the result of OAuth callback
type authResult struct {
	Code  string
	Error error
}

// generateSecureState generates a cryptographically secure random state parameter
func generateSecureState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// generateCodeVerifier generates a PKCE code verifier
func generateCodeVerifier() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(bytes), nil
}

// generateCodeChallenge generates a PKCE code challenge from verifier
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
}

// buildAuthorizationURL constructs the OAuth authorization URL
func (e *OAuthFlowExecutor) buildAuthorizationURL(config *providers.ServiceConfig, redirectURI, state, codeChallenge string) (string, error) {
	baseURL := e.Provider.GetAuthorizationEndpoint()

	// Handle Microsoft 365 tenant-specific endpoints
	if config.TenantID != "" && strings.Contains(baseURL, "${MS365_TENANT_ID}") {
		baseURL = strings.ReplaceAll(baseURL, "${MS365_TENANT_ID}", config.TenantID)
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid authorization endpoint: %w", err)
	}

	params := url.Values{}
	params.Set("client_id", config.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", config.Scopes)
	params.Set("state", state)

	// Add PKCE parameters
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	// Add service-specific parameters
	if config.ServiceName == "google" {
		params.Set("access_type", "offline")
		params.Set("prompt", "consent")
	} else if config.ServiceName == "microsoft365" {
		params.Set("prompt", "consent")
	}

	parsedURL.RawQuery = params.Encode()
	return parsedURL.String(), nil
}

// createCallbackHandler creates the HTTP handler for OAuth callback
func (e *OAuthFlowExecutor) createCallbackHandler(expectedState, _ string, _ *providers.ServiceConfig, resultChan chan<- authResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}

		// Parse query parameters
		query := r.URL.Query()
		code := query.Get("code")
		state := query.Get("state")
		errorParam := query.Get("error")
		errorDesc := query.Get("error_description")

		// Handle OAuth errors
		if errorParam != "" {
			errorMsg := fmt.Sprintf("OAuth error: %s", errorParam)
			if errorDesc != "" {
				errorMsg += fmt.Sprintf(" - %s", errorDesc)
			}
			e.writeErrorResponse(w, errorMsg)
			resultChan <- authResult{Error: fmt.Errorf(errorMsg)}
			return
		}

		// Validate state parameter
		if state != expectedState {
			errorMsg := "invalid state parameter - possible CSRF attack"
			e.writeErrorResponse(w, errorMsg)
			resultChan <- authResult{Error: fmt.Errorf(errorMsg)}
			return
		}

		// Validate authorization code
		if code == "" {
			errorMsg := "no authorization code received"
			e.writeErrorResponse(w, errorMsg)
			resultChan <- authResult{Error: fmt.Errorf(errorMsg)}
			return
		}

		// Success response
		e.writeSuccessResponse(w)
		resultChan <- authResult{Code: code}
	})
}

// exchangeCodeForTokens exchanges the authorization code for access tokens
func (e *OAuthFlowExecutor) exchangeCodeForTokens(code, redirectURI, codeVerifier string, config *providers.ServiceConfig) (*providers.TokenInfo, error) {
	tokenURL := e.Provider.GetTokenEndpoint()

	// Handle Microsoft 365 tenant-specific endpoints
	if config.TenantID != "" && strings.Contains(tokenURL, "${MS365_TENANT_ID}") {
		tokenURL = strings.ReplaceAll(tokenURL, "${MS365_TENANT_ID}", config.TenantID)
	}

	// Prepare token request parameters
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("client_id", config.ClientID)
	params.Set("code", code)
	params.Set("redirect_uri", redirectURI)
	params.Set("code_verifier", codeVerifier)

	// Add client secret if available and required
	if config.ClientSecret != "" {
		params.Set("client_secret", config.ClientSecret)
	}

	// Allow provider-specific customization
	paramMap := make(map[string]string)
	for key, values := range params {
		if len(values) > 0 {
			paramMap[key] = values[0]
		}
	}

	if err := e.Provider.CustomizeTokenRequest(paramMap, config); err != nil {
		return nil, fmt.Errorf("failed to customize token request: %w", err)
	}

	// Update params with provider customizations
	params = url.Values{}
	for key, value := range paramMap {
		params.Set(key, value)
	}

	// Make token request
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Log the request if debug is enabled
	debug.LogHTTPRequest(req)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// Log the response if debug is enabled
	debug.LogHTTPResponse(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse token response
	var tokenResponse map[string]interface{}
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Let provider process the response
	tokenInfo, err := e.Provider.ProcessTokenResponse(tokenResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to process token response: %w", err)
	}

	if e.Verbose {
		log.Printf("Successfully obtained tokens (expires in %d seconds)", tokenInfo.ExpiresIn)
	}

	return tokenInfo, nil
}

// writeSuccessResponse writes a success HTML response
func (e *OAuthFlowExecutor) writeSuccessResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	htmlDoc := `<!DOCTYPE html>
<html>
<head>
    <title>OAuth Success</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background-color: #f0f8ff; }
        .success { color: #28a745; font-size: 24px; margin-bottom: 20px; }
        .info { color: #666; font-size: 16px; }
        .checkmark { font-size: 48px; color: #28a745; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="checkmark">✓</div>
    <div class="success">OAuth Authentication Successful!</div>
    <div class="info">You can now close this window and return to the terminal.</div>
    <script>
        // Auto-close window after 3 seconds
        setTimeout(function() {
            window.close();
        }, 3000);
    </script>
</body>
</html>`

	_, _ = w.Write([]byte(htmlDoc))
}

// writeErrorResponse writes an error HTML response
func (e *OAuthFlowExecutor) writeErrorResponse(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	htmlDoc := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>OAuth Error</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background-color: #fff5f5; }
        .error { color: #dc3545; font-size: 24px; margin-bottom: 20px; }
        .info { color: #666; font-size: 16px; }
        .cross { font-size: 48px; color: #dc3545; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="cross">✗</div>
    <div class="error">OAuth Authentication Failed</div>
    <div class="info">%s</div>
    <div class="info" style="margin-top: 20px;">Please close this window and try again.</div>
</body>
</html>`, html.EscapeString(errorMsg))

	_, _ = w.Write([]byte(htmlDoc))
}

// openBrowser opens the default browser to the given URL
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.Command(cmd, args...).Start()
}
