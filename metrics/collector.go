/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

// Package metrics provides a shared, thread-safe request metrics collector
// that any package (fusion, hub, future services) can use without circular
// dependency risk — it imports only stdlib.
package metrics

import (
	"sort"
	"sync"
	"time"
)

// Collector tracks request and error counts for registered services.
// All methods are safe for concurrent use.
type Collector struct {
	mu       sync.RWMutex
	start    time.Time
	services map[string]*ServiceStats
}

// ServiceStats describes the operational state and request metrics for one service.
type ServiceStats struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`       // "api", "mcp_stdio", "mcp_sse", "mcp_http", "internal"
	Status    string `json:"status"`           // "operational", "degraded", "disconnected"
	Tools     *int   `json:"tools,omitempty"`  // nil for non-tool services
	Requests  int64  `json:"requests"`
	Errors    int64  `json:"errors"`
}

// New creates a new Collector and records the server start time.
func New() *Collector {
	return &Collector{
		start:    time.Now(),
		services: make(map[string]*ServiceStats),
	}
}

// RegisterService registers a service with its transport and initial tool count.
// If the service already exists, it updates transport and tools but preserves
// existing request/error counts and status.
func (c *Collector) RegisterService(name, transport string, tools *int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.services[name]; ok {
		existing.Transport = transport
		existing.Tools = tools
		return
	}

	c.services[name] = &ServiceStats{
		Name:      name,
		Transport: transport,
		Status:    "operational",
		Tools:     tools,
	}
}

// RecordRequest increments the request counter for a service.
// If isError is true, the error counter is also incremented.
// Calls for unregistered services are silently ignored.
func (c *Collector) RecordRequest(service string, isError bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s, ok := c.services[service]
	if !ok {
		return
	}
	s.Requests++
	if isError {
		s.Errors++
	}
}

// SetStatus updates the status string for a service.
// Calls for unregistered services are silently ignored.
func (c *Collector) SetStatus(service, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if s, ok := c.services[service]; ok {
		s.Status = status
	}
}

// SetToolCount updates the tool count for a service.
// Calls for unregistered services are silently ignored.
func (c *Collector) SetToolCount(service string, count *int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if s, ok := c.services[service]; ok {
		s.Tools = count
	}
}

// GetServiceStats returns a snapshot of a single service's stats, or nil if not found.
func (c *Collector) GetServiceStats(service string) *ServiceStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s, ok := c.services[service]
	if !ok {
		return nil
	}
	return s.snapshot()
}

// GetAllServiceStats returns snapshots of all registered services, sorted by name.
func (c *Collector) GetAllServiceStats() []ServiceStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]ServiceStats, 0, len(c.services))
	for _, s := range c.services {
		result = append(result, *s.snapshot())
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// snapshot returns a deep copy of the ServiceStats, including the Tools pointer.
func (s *ServiceStats) snapshot() *ServiceStats {
	cpy := *s
	if s.Tools != nil {
		t := *s.Tools
		cpy.Tools = &t
	}
	return &cpy
}

// GetUptime returns the duration since the collector was created.
func (c *Collector) GetUptime() time.Duration {
	return time.Since(c.start)
}
