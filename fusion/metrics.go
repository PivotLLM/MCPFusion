// Copyright (c) 2025 Tenebris Technologies Inc.
// Please see LICENSE for details.

package fusion

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// MetricsCollector collects metrics for requests, errors, and performance
type MetricsCollector struct {
	mu           sync.RWMutex
	logger       global.Logger
	enabled      bool
	metrics      map[string]*ServiceMetrics
	startTime    time.Time
	requestCount int64
	errorCount   int64
}

// ServiceMetrics contains metrics for a specific service
type ServiceMetrics struct {
	ServiceName   string                    `json:"service_name"`
	RequestCount  int64                     `json:"request_count"`
	ErrorCount    int64                     `json:"error_count"`
	SuccessCount  int64                     `json:"success_count"`
	TotalLatency  time.Duration             `json:"total_latency"`
	MinLatency    time.Duration             `json:"min_latency"`
	MaxLatency    time.Duration             `json:"max_latency"`
	AvgLatency    time.Duration             `json:"avg_latency"`
	ErrorsByType  map[ErrorCategory]int64   `json:"errors_by_type"`
	EndpointStats map[string]*EndpointStats `json:"endpoint_stats"`
	LastError     time.Time                 `json:"last_error"`
	LastRequest   time.Time                 `json:"last_request"`
}

// EndpointStats contains metrics for a specific endpoint
type EndpointStats struct {
	EndpointID    string                  `json:"endpoint_id"`
	RequestCount  int64                   `json:"request_count"`
	ErrorCount    int64                   `json:"error_count"`
	SuccessCount  int64                   `json:"success_count"`
	TotalLatency  time.Duration           `json:"total_latency"`
	MinLatency    time.Duration           `json:"min_latency"`
	MaxLatency    time.Duration           `json:"max_latency"`
	AvgLatency    time.Duration           `json:"avg_latency"`
	ErrorsByType  map[ErrorCategory]int64 `json:"errors_by_type"`
	RetryCount    int64                   `json:"retry_count"`
	CacheHitCount int64                   `json:"cache_hit_count"`
	LastError     time.Time               `json:"last_error"`
	LastRequest   time.Time               `json:"last_request"`
}

// RequestMetrics represents metrics for a single request
type RequestMetrics struct {
	ServiceName   string        `json:"service_name"`
	EndpointID    string        `json:"endpoint_id"`
	Method        string        `json:"method"`
	URL           string        `json:"url"`
	StatusCode    int           `json:"status_code"`
	Latency       time.Duration `json:"latency"`
	Success       bool          `json:"success"`
	RetryCount    int           `json:"retry_count"`
	CacheHit      bool          `json:"cache_hit"`
	ErrorCategory ErrorCategory `json:"error_category,omitempty"`
	CorrelationID string        `json:"correlation_id,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger global.Logger, enabled bool) *MetricsCollector {
	return &MetricsCollector{
		logger:    logger,
		enabled:   enabled,
		metrics:   make(map[string]*ServiceMetrics),
		startTime: time.Now(),
	}
}

// RecordRequest records metrics for a completed request
func (mc *MetricsCollector) RecordRequest(req RequestMetrics) {
	if !mc.enabled {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Update global counters
	mc.requestCount++
	if !req.Success {
		mc.errorCount++
	}

	// Get or create service metrics
	serviceMetrics, exists := mc.metrics[req.ServiceName]
	if !exists {
		serviceMetrics = &ServiceMetrics{
			ServiceName:   req.ServiceName,
			ErrorsByType:  make(map[ErrorCategory]int64),
			EndpointStats: make(map[string]*EndpointStats),
			MinLatency:    req.Latency,
			MaxLatency:    req.Latency,
		}
		mc.metrics[req.ServiceName] = serviceMetrics
	}

	// Update service metrics
	serviceMetrics.RequestCount++
	serviceMetrics.LastRequest = req.Timestamp
	serviceMetrics.TotalLatency += req.Latency

	if req.Success {
		serviceMetrics.SuccessCount++
	} else {
		serviceMetrics.ErrorCount++
		serviceMetrics.LastError = req.Timestamp
		if req.ErrorCategory != "" {
			serviceMetrics.ErrorsByType[req.ErrorCategory]++
		}
	}

	// Update latency stats
	if req.Latency < serviceMetrics.MinLatency || serviceMetrics.MinLatency == 0 {
		serviceMetrics.MinLatency = req.Latency
	}
	if req.Latency > serviceMetrics.MaxLatency {
		serviceMetrics.MaxLatency = req.Latency
	}
	if serviceMetrics.RequestCount > 0 {
		serviceMetrics.AvgLatency = serviceMetrics.TotalLatency / time.Duration(serviceMetrics.RequestCount)
	}

	// Get or create endpoint stats
	endpointStats, exists := serviceMetrics.EndpointStats[req.EndpointID]
	if !exists {
		endpointStats = &EndpointStats{
			EndpointID:   req.EndpointID,
			ErrorsByType: make(map[ErrorCategory]int64),
			MinLatency:   req.Latency,
			MaxLatency:   req.Latency,
		}
		serviceMetrics.EndpointStats[req.EndpointID] = endpointStats
	}

	// Update endpoint stats
	endpointStats.RequestCount++
	endpointStats.LastRequest = req.Timestamp
	endpointStats.TotalLatency += req.Latency
	endpointStats.RetryCount += int64(req.RetryCount)

	if req.CacheHit {
		endpointStats.CacheHitCount++
	}

	if req.Success {
		endpointStats.SuccessCount++
	} else {
		endpointStats.ErrorCount++
		endpointStats.LastError = req.Timestamp
		if req.ErrorCategory != "" {
			endpointStats.ErrorsByType[req.ErrorCategory]++
		}
	}

	// Update endpoint latency stats
	if req.Latency < endpointStats.MinLatency || endpointStats.MinLatency == 0 {
		endpointStats.MinLatency = req.Latency
	}
	if req.Latency > endpointStats.MaxLatency {
		endpointStats.MaxLatency = req.Latency
	}
	if endpointStats.RequestCount > 0 {
		endpointStats.AvgLatency = endpointStats.TotalLatency / time.Duration(endpointStats.RequestCount)
	}

	// Log metrics periodically
	if mc.logger != nil && mc.requestCount%100 == 0 {
		mc.logMetricsSummary()
	}
}

// GetServiceMetrics returns metrics for a specific service
func (mc *MetricsCollector) GetServiceMetrics(serviceName string) *ServiceMetrics {
	if !mc.enabled {
		return nil
	}

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if metrics, exists := mc.metrics[serviceName]; exists {
		// Return a copy to avoid race conditions
		copy := *metrics
		copy.ErrorsByType = make(map[ErrorCategory]int64)
		for k, v := range metrics.ErrorsByType {
			copy.ErrorsByType[k] = v
		}
		copy.EndpointStats = make(map[string]*EndpointStats)
		for k, v := range metrics.EndpointStats {
			endpointCopy := *v
			endpointCopy.ErrorsByType = make(map[ErrorCategory]int64)
			for ek, ev := range v.ErrorsByType {
				endpointCopy.ErrorsByType[ek] = ev
			}
			copy.EndpointStats[k] = &endpointCopy
		}
		return &copy
	}

	return nil
}

// GetAllMetrics returns all collected metrics
func (mc *MetricsCollector) GetAllMetrics() map[string]*ServiceMetrics {
	if !mc.enabled {
		return nil
	}

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*ServiceMetrics)
	for serviceName := range mc.metrics {
		result[serviceName] = mc.GetServiceMetrics(serviceName)
	}

	return result
}

// GetGlobalMetrics returns global metrics summary
func (mc *MetricsCollector) GetGlobalMetrics() GlobalMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	uptime := time.Since(mc.startTime)
	var successRate float64
	if mc.requestCount > 0 {
		successRate = float64(mc.requestCount-mc.errorCount) / float64(mc.requestCount) * 100
	}

	return GlobalMetrics{
		Uptime:       uptime,
		RequestCount: mc.requestCount,
		ErrorCount:   mc.errorCount,
		SuccessRate:  successRate,
		ServiceCount: int64(len(mc.metrics)),
		StartTime:    mc.startTime,
	}
}

// GlobalMetrics represents global system metrics
type GlobalMetrics struct {
	Uptime       time.Duration `json:"uptime"`
	RequestCount int64         `json:"request_count"`
	ErrorCount   int64         `json:"error_count"`
	SuccessRate  float64       `json:"success_rate"`
	ServiceCount int64         `json:"service_count"`
	StartTime    time.Time     `json:"start_time"`
}

// GetErrorRate returns the error rate for a service over a time window
func (mc *MetricsCollector) GetErrorRate(serviceName string, window time.Duration) float64 {
	if !mc.enabled {
		return 0
	}

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics, exists := mc.metrics[serviceName]
	if !exists || metrics.RequestCount == 0 {
		return 0
	}

	return float64(metrics.ErrorCount) / float64(metrics.RequestCount) * 100
}

// IsServiceHealthy checks if a service is healthy based on error rate thresholds
func (mc *MetricsCollector) IsServiceHealthy(serviceName string, errorRateThreshold float64) bool {
	if !mc.enabled {
		return true // Assume healthy if metrics disabled
	}

	errorRate := mc.GetErrorRate(serviceName, 5*time.Minute)
	return errorRate <= errorRateThreshold
}

// Reset resets all metrics
func (mc *MetricsCollector) Reset() {
	if !mc.enabled {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.metrics = make(map[string]*ServiceMetrics)
	mc.requestCount = 0
	mc.errorCount = 0
	mc.startTime = time.Now()

	if mc.logger != nil {
		mc.logger.Info("Metrics collector reset")
	}
}

// logMetricsSummary logs a summary of current metrics
func (mc *MetricsCollector) logMetricsSummary() {
	if mc.logger == nil {
		return
	}

	global := mc.GetGlobalMetrics()
	mc.logger.Infof("Metrics Summary - Requests: %d, Errors: %d, Success Rate: %.1f%%, Services: %d, Uptime: %v",
		global.RequestCount, global.ErrorCount, global.SuccessRate, global.ServiceCount, global.Uptime)

	// Log top error-prone services
	for serviceName, metrics := range mc.metrics {
		if metrics.ErrorCount > 0 {
			errorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount) * 100
			if errorRate > 5.0 { // Log services with >5% error rate
				mc.logger.Warningf("Service '%s': %d requests, %d errors (%.1f%%), avg latency: %v",
					serviceName, metrics.RequestCount, metrics.ErrorCount, errorRate, metrics.AvgLatency)
			}
		}
	}
}

// StartPeriodicLogging starts periodic logging of metrics
func (mc *MetricsCollector) StartPeriodicLogging(ctx context.Context, interval time.Duration) {
	if !mc.enabled || mc.logger == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mc.logMetricsSummary()
			}
		}
	}()
}

// CorrelationIDGenerator generates correlation IDs for request tracking
type CorrelationIDGenerator struct {
	mu      sync.Mutex
	counter int64
}

// NewCorrelationIDGenerator creates a new correlation ID generator
func NewCorrelationIDGenerator() *CorrelationIDGenerator {
	return &CorrelationIDGenerator{}
}

// Generate generates a new correlation ID
func (g *CorrelationIDGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.counter++
	timestamp := time.Now().Unix()
	
	return fmt.Sprintf("mcpfusion-%d-%d", timestamp, g.counter)
}