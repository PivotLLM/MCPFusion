/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PivotLLM/MCPFusion/mlogger"
)

func TestMetricsCollector(t *testing.T) {
	logger, _ := mlogger.New()

	t.Run("basic metrics collection", func(t *testing.T) {
		collector := NewMetricsCollector(logger, true)

		// Record some requests
		req1 := RequestMetrics{
			ServiceName:   "test-service",
			EndpointID:    "test-endpoint",
			Method:        "GET",
			URL:           "https://example.com/test",
			StatusCode:    200,
			Latency:       100 * time.Millisecond,
			Success:       true,
			RetryCount:    0,
			CacheHit:      false,
			CorrelationID: "test-123",
			Timestamp:     time.Now(),
		}
		collector.RecordRequest(req1)

		req2 := RequestMetrics{
			ServiceName:   "test-service",
			EndpointID:    "test-endpoint",
			Method:        "POST",
			URL:           "https://example.com/test",
			StatusCode:    500,
			Latency:       200 * time.Millisecond,
			Success:       false,
			RetryCount:    2,
			CacheHit:      false,
			ErrorCategory: ErrorCategoryServer,
			CorrelationID: "test-456",
			Timestamp:     time.Now(),
		}
		collector.RecordRequest(req2)

		// Check service metrics
		serviceMetrics := collector.GetServiceMetrics("test-service")
		if serviceMetrics == nil {
			t.Fatal("Expected service metrics but got nil")
		}

		if serviceMetrics.RequestCount != 2 {
			t.Errorf("Expected 2 requests, got %d", serviceMetrics.RequestCount)
		}
		if serviceMetrics.SuccessCount != 1 {
			t.Errorf("Expected 1 success, got %d", serviceMetrics.SuccessCount)
		}
		if serviceMetrics.ErrorCount != 1 {
			t.Errorf("Expected 1 error, got %d", serviceMetrics.ErrorCount)
		}

		// Check error categorization
		if serviceMetrics.ErrorsByType[ErrorCategoryServer] != 1 {
			t.Errorf("Expected 1 server error, got %d", serviceMetrics.ErrorsByType[ErrorCategoryServer])
		}

		// Check endpoint metrics
		endpointStats := serviceMetrics.EndpointStats["test-endpoint"]
		if endpointStats == nil {
			t.Fatal("Expected endpoint stats but got nil")
		}

		if endpointStats.RequestCount != 2 {
			t.Errorf("Expected 2 endpoint requests, got %d", endpointStats.RequestCount)
		}
		if endpointStats.RetryCount != 2 {
			t.Errorf("Expected 2 total retries, got %d", endpointStats.RetryCount)
		}

		// Check latency calculations
		expectedAvgLatency := (100*time.Millisecond + 200*time.Millisecond) / 2
		if serviceMetrics.AvgLatency != expectedAvgLatency {
			t.Errorf("Expected avg latency %v, got %v", expectedAvgLatency, serviceMetrics.AvgLatency)
		}
		if serviceMetrics.MinLatency != 100*time.Millisecond {
			t.Errorf("Expected min latency %v, got %v", 100*time.Millisecond, serviceMetrics.MinLatency)
		}
		if serviceMetrics.MaxLatency != 200*time.Millisecond {
			t.Errorf("Expected max latency %v, got %v", 200*time.Millisecond, serviceMetrics.MaxLatency)
		}
	})

	t.Run("global metrics", func(t *testing.T) {
		collector := NewMetricsCollector(logger, true)

		// Record requests for multiple services
		req1 := RequestMetrics{
			ServiceName: "service1",
			EndpointID:  "endpoint1",
			Success:     true,
			Timestamp:   time.Now(),
		}
		collector.RecordRequest(req1)

		req2 := RequestMetrics{
			ServiceName: "service2",
			EndpointID:  "endpoint1",
			Success:     false,
			Timestamp:   time.Now(),
		}
		collector.RecordRequest(req2)

		globalMetrics := collector.GetGlobalMetrics()
		if globalMetrics.RequestCount != 2 {
			t.Errorf("Expected 2 total requests, got %d", globalMetrics.RequestCount)
		}
		if globalMetrics.ErrorCount != 1 {
			t.Errorf("Expected 1 total error, got %d", globalMetrics.ErrorCount)
		}
		if globalMetrics.SuccessRate != 50.0 {
			t.Errorf("Expected 50%% success rate, got %.1f%%", globalMetrics.SuccessRate)
		}
		if globalMetrics.ServiceCount != 2 {
			t.Errorf("Expected 2 services, got %d", globalMetrics.ServiceCount)
		}
	})

	t.Run("cache hit tracking", func(t *testing.T) {
		collector := NewMetricsCollector(logger, true)

		cacheHitReq := RequestMetrics{
			ServiceName: "test-service",
			EndpointID:  "test-endpoint",
			Success:     true,
			CacheHit:    true,
			Timestamp:   time.Now(),
		}
		collector.RecordRequest(cacheHitReq)

		serviceMetrics := collector.GetServiceMetrics("test-service")
		endpointStats := serviceMetrics.EndpointStats["test-endpoint"]

		if endpointStats.CacheHitCount != 1 {
			t.Errorf("Expected 1 cache hit, got %d", endpointStats.CacheHitCount)
		}
	})

	t.Run("error rate calculation", func(t *testing.T) {
		collector := NewMetricsCollector(logger, true)

		// Record 8 successes and 2 failures
		for i := 0; i < 8; i++ {
			req := RequestMetrics{
				ServiceName: "test-service",
				EndpointID:  "test-endpoint",
				Success:     true,
				Timestamp:   time.Now(),
			}
			collector.RecordRequest(req)
		}

		for i := 0; i < 2; i++ {
			req := RequestMetrics{
				ServiceName: "test-service",
				EndpointID:  "test-endpoint",
				Success:     false,
				Timestamp:   time.Now(),
			}
			collector.RecordRequest(req)
		}

		errorRate := collector.GetErrorRate("test-service", 5*time.Minute)
		expectedRate := 20.0 // 2 errors out of 10 requests = 20%
		if errorRate != expectedRate {
			t.Errorf("Expected error rate %.1f%%, got %.1f%%", expectedRate, errorRate)
		}

		// Check service health
		if !collector.IsServiceHealthy("test-service", 25.0) {
			t.Error("Expected service to be healthy with 25% threshold")
		}
		if collector.IsServiceHealthy("test-service", 15.0) {
			t.Error("Expected service to be unhealthy with 15% threshold")
		}
	})

	t.Run("disabled metrics", func(t *testing.T) {
		collector := NewMetricsCollector(logger, false)

		req := RequestMetrics{
			ServiceName: "test-service",
			EndpointID:  "test-endpoint",
			Success:     true,
			Timestamp:   time.Now(),
		}
		collector.RecordRequest(req)

		// Should not record anything when disabled
		serviceMetrics := collector.GetServiceMetrics("test-service")
		if serviceMetrics != nil {
			t.Error("Expected nil metrics when disabled")
		}

		allMetrics := collector.GetAllMetrics()
		if allMetrics != nil {
			t.Error("Expected nil metrics when disabled")
		}
	})

	t.Run("metrics reset", func(t *testing.T) {
		collector := NewMetricsCollector(logger, true)

		req := RequestMetrics{
			ServiceName: "test-service",
			EndpointID:  "test-endpoint",
			Success:     true,
			Timestamp:   time.Now(),
		}
		collector.RecordRequest(req)

		// Verify metrics are recorded
		global := collector.GetGlobalMetrics()
		if global.RequestCount != 1 {
			t.Errorf("Expected 1 request before reset, got %d", global.RequestCount)
		}

		// Reset metrics
		collector.Reset()

		// Verify metrics are cleared
		global = collector.GetGlobalMetrics()
		if global.RequestCount != 0 {
			t.Errorf("Expected 0 requests after reset, got %d", global.RequestCount)
		}

		serviceMetrics := collector.GetServiceMetrics("test-service")
		if serviceMetrics != nil {
			t.Error("Expected nil service metrics after reset")
		}
	})

	t.Run("periodic logging", func(t *testing.T) {
		collector := NewMetricsCollector(logger, true)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start periodic logging with very short interval
		collector.StartPeriodicLogging(ctx, 10*time.Millisecond)

		// Record a request to generate some metrics
		req := RequestMetrics{
			ServiceName: "test-service",
			EndpointID:  "test-endpoint",
			Success:     true,
			Timestamp:   time.Now(),
		}
		collector.RecordRequest(req)

		// Wait for context to be done (should complete without hanging)
		<-ctx.Done()

		// Test passes if it completes without hanging
	})
}

func TestCorrelationIDGenerator(t *testing.T) {
	generator := NewCorrelationIDGenerator()

	t.Run("unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)

		// Generate 100 IDs and check uniqueness
		for i := 0; i < 100; i++ {
			id := generator.Generate()
			if ids[id] {
				t.Errorf("Generated duplicate correlation ID: %s", id)
			}
			ids[id] = true

			// Check format
			if !strings.HasPrefix(id, "mcpfusion-") {
				t.Errorf("Expected ID to start with 'mcpfusion-', got: %s", id)
			}
		}
	})

	t.Run("sequential counter", func(t *testing.T) {
		id1 := generator.Generate()
		id2 := generator.Generate()

		// IDs should be different due to counter increment
		if id1 == id2 {
			t.Errorf("Expected different IDs, got same: %s", id1)
		}
	})
}
