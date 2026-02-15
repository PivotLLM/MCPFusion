/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

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

// registerHealthTool returns the tool definition for the health tool.
func (f *Fusion) registerHealthTool() global.ToolDefinition {
	return global.ToolDefinition{
		Name:        "health_status",
		Description: "Returns the operational status of MCPFusion and all connected services as JSON.",
		Parameters:  []global.Parameter{},
		Handler:     f.handleHealth,
		Hints: &global.ToolHints{
			ReadOnly:    global.BoolPtr(true),
			Destructive: global.BoolPtr(false),
			Idempotent:  global.BoolPtr(true),
			OpenWorld:   global.BoolPtr(false),
		},
	}
}

// handleHealth is the tool handler for the health tool.
func (f *Fusion) handleHealth(_ map[string]interface{}) (string, error) {
	allHealthy := true
	var uptime time.Duration

	// Prefer shared collector for service data and uptime
	if f.sharedCollector != nil {
		uptime = f.sharedCollector.GetUptime()

		allStats := f.sharedCollector.GetAllServiceStats()
		cbMetrics := f.GetAllCircuitBreakerMetrics()

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

			// Check base status from shared collector first
			if ss.Status == global.StatusDisconnected || ss.Status == global.StatusDegraded {
				allHealthy = false
			}

			// Overlay circuit breaker state for API services
			if cbm, ok := cbMetrics[ss.Name]; ok {
				hs.CircuitBreaker = strings.ToLower(cbm.State.String())
				if cbm.State == CircuitBreakerOpen {
					hs.Status = global.StatusDegraded
					allHealthy = false
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

	// Fallback: use existing per-package metrics if no shared collector
	if f.metricsCollector != nil {
		gm := f.metricsCollector.GetGlobalMetrics()
		uptime = gm.Uptime
	}

	services := make([]healthService, 0)

	// API services (config-driven, non-hub)
	if f.config != nil {
		cbMetrics := f.GetAllCircuitBreakerMetrics()
		allServiceMetrics := f.GetMetrics()

		for serviceName, svc := range f.config.Services {
			if svc.IsHubService() {
				continue
			}

			status := global.StatusOperational
			cbState := "closed"

			if cbm, ok := cbMetrics[serviceName]; ok {
				cbState = strings.ToLower(cbm.State.String())
				if cbm.State == CircuitBreakerOpen {
					status = global.StatusDegraded
					allHealthy = false
				}
			}

			var requests, errors int64
			if sm, ok := allServiceMetrics[serviceName]; ok && sm != nil {
				requests = sm.RequestCount
				errors = sm.ErrorCount
			}

			toolCount := len(svc.Endpoints)
			services = append(services, healthService{
				Name:           serviceName,
				Transport:      global.TransportAPI,
				Status:         status,
				Tools:          &toolCount,
				Requests:       requests,
				Errors:         errors,
				CircuitBreaker: cbState,
			})
		}
	}

	// Internal tools (knowledge store)
	if f.database != nil {
		knowledgeTools := f.registerKnowledgeTools()
		toolCount := len(knowledgeTools)
		services = append(services, healthService{
			Name:      "knowledge",
			Transport: global.TransportInternal,
			Status:    global.StatusOperational,
			Tools:     &toolCount,
		})
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

// formatDuration renders a duration as a human-readable string like "2h15m".
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
