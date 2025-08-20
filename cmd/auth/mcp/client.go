/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PivotLLM/MCPFusion/cmd/auth/debug"
	"github.com/PivotLLM/MCPFusion/cmd/auth/providers"
)

// Client handles communication with MCPFusion server
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// NewClient creates a new MCP client
func NewClient(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:  baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TokenRequest represents a request to store OAuth tokens
type TokenRequest struct {
	Service      string `json:"service"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// TokenResponse represents the response from storing OAuth tokens
type TokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	TokenID string `json:"token_id,omitempty"`
}

// StoreTokens sends OAuth tokens to MCPFusion for storage
func (c *Client) StoreTokens(ctx context.Context, service, accessToken, refreshToken string) (*TokenResponse, error) {
	req := &TokenRequest{
		Service:      service,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	endpoint := fmt.Sprintf("%s/api/v1/oauth/tokens", c.baseURL)

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiToken)
	httpReq.Header.Set("User-Agent", "fusion-oauth/1.0")

	// Log the request if debug is enabled
	debug.LogHTTPRequest(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send token request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// Log the response if debug is enabled
	debug.LogHTTPResponse(resp)

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &tokenResp, fmt.Errorf("token storage failed with status %d: %s", resp.StatusCode, tokenResp.Message)
	}

	return &tokenResp, nil
}

// HealthCheck checks if the MCPFusion server is accessible
func (c *Client) HealthCheck(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("User-Agent", "fusion-oauth/1.0")

	// Log the request if debug is enabled
	debug.LogHTTPRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// Log the response if debug is enabled
	debug.LogHTTPResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MCPFusion server health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// AuthCheck verifies that the API token is valid
func (c *Client) AuthCheck(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/api/v1/auth/verify", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create auth check request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("User-Agent", "fusion-oauth/1.0")

	// Log the request if debug is enabled
	debug.LogHTTPRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth check failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// Log the response if debug is enabled
	debug.LogHTTPResponse(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("API token is invalid or expired")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetServiceConfig retrieves OAuth configuration for a specific service
func (c *Client) GetServiceConfig(ctx context.Context, serviceName string) (*ServiceConfigResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/services/%s/config", c.baseURL, serviceName)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create service config request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("User-Agent", "fusion-oauth/1.0")

	// Log the request if debug is enabled
	debug.LogHTTPRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("service config request failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// Log the response if debug is enabled
	debug.LogHTTPResponse(resp)

	var configResp ServiceConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return nil, fmt.Errorf("failed to decode service config response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &configResp, fmt.Errorf("service config request failed with status %d: %s", resp.StatusCode, configResp.Message)
	}

	return &configResp, nil
}

// ServiceConfigResponse represents the response from getting service config
type ServiceConfigResponse struct {
	Success     bool                     `json:"success"`
	Message     string                   `json:"message"`
	ServiceName string                   `json:"service_name,omitempty"`
	Config      *providers.ServiceConfig `json:"config,omitempty"`
}

// NotifySuccess sends a success notification to MCPFusion
func (c *Client) NotifySuccess(ctx context.Context, serviceName string, userInfo *providers.UserInfo) error {
	endpoint := fmt.Sprintf("%s/api/v1/oauth/success", c.baseURL)

	notification := struct {
		Service   string              `json:"service"`
		UserInfo  *providers.UserInfo `json:"user_info"`
		Timestamp time.Time           `json:"timestamp"`
	}{
		Service:   serviceName,
		UserInfo:  userInfo,
		Timestamp: time.Now(),
	}

	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal success notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create success notification request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("User-Agent", "fusion-oauth/1.0")

	// Log the request if debug is enabled
	debug.LogHTTPRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("success notification failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// Log the response if debug is enabled
	debug.LogHTTPResponse(resp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("success notification failed with status: %d", resp.StatusCode)
	}

	return nil
}

// NotifyError sends an error notification to MCPFusion
func (c *Client) NotifyError(ctx context.Context, serviceName string, errorMsg string) error {
	endpoint := fmt.Sprintf("%s/api/v1/oauth/error", c.baseURL)

	notification := struct {
		Service   string    `json:"service"`
		Error     string    `json:"error"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Service:   serviceName,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}

	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal error notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create error notification request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("User-Agent", "fusion-oauth/1.0")

	// Log the request if debug is enabled
	debug.LogHTTPRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error notification failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// Log the response if debug is enabled
	debug.LogHTTPResponse(resp)

	// Don't fail if error notification fails - it's not critical
	return nil
}
