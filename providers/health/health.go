/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package health provides the health_status MCP tool as a standalone ToolProvider.
package health

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/metrics"
)

// CircuitBreakerSource is the interface the health provider uses to obtain circuit
// breaker state for each registered service.  It is defined here so that callers
// can supply an adapter without importing the fusion package.
type CircuitBreakerSource interface {
	GetAllCircuitBreakerMetrics() map[string]CircuitBreakerInfo
}

// CircuitBreakerInfo carries the minimal circuit-breaker state needed by the
// health handler.
type CircuitBreakerInfo struct {
	// State is the human-readable state: "closed", "open", or "half-open".
	State string
	// IsOpen is true when the circuit breaker is in the open state.
	IsOpen bool
}

// healthResponse is the top-level JSON structure returned by the health tool.
type healthResponse struct {
	Server   healthServer    `json:"server"`
	Services []healthService `json:"services"`
}

// healthServer describes the overall server status.
type healthServer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
	Uptime  string `json:"uptime"`
}

// healthService describes the operational status of a single service.
type healthService struct {
	Name           string `json:"name"`
	Transport      string `json:"transport"`
	Status         string `json:"status"`
	Tools          *int   `json:"tools,omitempty"`
	Requests       int64  `json:"requests"`
	Errors         int64  `json:"errors"`
	CircuitBreaker string `json:"circuit_breaker,omitempty"`
}

// Provider implements global.ToolProvider for the health_status tool.
type Provider struct {
	logger    global.Logger
	collector *metrics.Collector
	cbSource  CircuitBreakerSource
}

// Option is a functional option for configuring a Provider.
type Option func(*Provider)

// WithLogger sets the logger.
func WithLogger(l global.Logger) Option {
	return func(p *Provider) { p.logger = l }
}

// WithCollector sets the shared metrics collector used to obtain service stats
// and uptime.
func WithCollector(c *metrics.Collector) Option {
	return func(p *Provider) { p.collector = c }
}

// WithCircuitBreakerSource sets the source for circuit-breaker state.
func WithCircuitBreakerSource(s CircuitBreakerSource) Option {
	return func(p *Provider) { p.cbSource = s }
}

// New creates a new health Provider with the given options.
func New(opts ...Option) *Provider {
	p := &Provider{}
	for _, o := range opts {
		o(p)
	}
	return p
}

// RegisterTools implements global.ToolProvider and returns the health_status tool.
func (p *Provider) RegisterTools() []global.ToolDefinition {
	return []global.ToolDefinition{
		{
			Name:        "health_status",
			Description: "Returns the operational status of MCPFusion and all connected services as JSON.",
			Parameters:  []global.Parameter{},
			Handler:     p.handleHealth,
			Hints: &global.ToolHints{
				ReadOnly:    global.BoolPtr(true),
				Destructive: global.BoolPtr(false),
				Idempotent:  global.BoolPtr(true),
				OpenWorld:   global.BoolPtr(false),
			},
		},
	}
}

// handleHealth is the tool handler for the health_status tool.
func (p *Provider) handleHealth(_ map[string]interface{}) (string, error) {
	allHealthy := true

	if p.collector == nil {
		// No collector — return minimal response.
		resp := healthResponse{
			Server: healthServer{
				Name:    global.AppName,
				Version: global.AppVersion,
				Status:  global.StatusHealthy,
				Uptime:  "0s",
			},
			Services: []healthService{},
		}
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal health response: %w", err)
		}
		return string(data), nil
	}

	uptime := p.collector.GetUptime()
	allStats := p.collector.GetAllServiceStats()

	var cbMetrics map[string]CircuitBreakerInfo
	if p.cbSource != nil {
		cbMetrics = p.cbSource.GetAllCircuitBreakerMetrics()
	}

	services := make([]healthService, 0, len(allStats))
	for _, ss := range allStats {
		hs := healthService{
			Name:      ss.Name,
			Transport: ss.Transport,
			Status:    ss.Status,
			Tools:     ss.Tools,
			Requests:  ss.Requests,
			Errors:    ss.Errors,
		}

		// Check base status from shared collector.
		if ss.Status == global.StatusDisconnected || ss.Status == global.StatusDegraded {
			allHealthy = false
		}

		// Overlay circuit-breaker state for API services.
		if cbMetrics != nil {
			if info, ok := cbMetrics[ss.Name]; ok {
				hs.CircuitBreaker = strings.ToLower(info.State)
				if info.IsOpen {
					hs.Status = global.StatusDegraded
					allHealthy = false
				}
			}
		}

		services = append(services, hs)
	}

	overallStatus := global.StatusHealthy
	if !allHealthy {
		overallStatus = global.StatusDegraded
	}

	resp := healthResponse{
		Server: healthServer{
			Name:    global.AppName,
			Version: global.AppVersion,
			Status:  overallStatus,
			Uptime:  formatDuration(uptime),
		},
		Services: services,
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal health response: %w", err)
	}
	return string(data), nil
}

// formatDuration renders a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
