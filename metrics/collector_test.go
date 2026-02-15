/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if len(c.services) != 0 {
		t.Fatalf("expected 0 services, got %d", len(c.services))
	}
}

func TestRegisterService(t *testing.T) {
	c := New()
	tools := 5
	c.RegisterService("svc1", "api", &tools)

	s := c.GetServiceStats("svc1")
	if s == nil {
		t.Fatal("expected service stats, got nil")
	}
	if s.Name != "svc1" {
		t.Errorf("expected name 'svc1', got '%s'", s.Name)
	}
	if s.Transport != "api" {
		t.Errorf("expected transport 'api', got '%s'", s.Transport)
	}
	if s.Status != "operational" {
		t.Errorf("expected status 'operational', got '%s'", s.Status)
	}
	if s.Tools == nil || *s.Tools != 5 {
		t.Errorf("expected tools=5, got %v", s.Tools)
	}
	if s.Requests != 0 || s.Errors != 0 {
		t.Errorf("expected 0 requests/errors, got %d/%d", s.Requests, s.Errors)
	}
}

func TestRegisterServiceNilTools(t *testing.T) {
	c := New()
	c.RegisterService("internal", "internal", nil)

	s := c.GetServiceStats("internal")
	if s == nil {
		t.Fatal("expected service stats, got nil")
	}
	if s.Tools != nil {
		t.Errorf("expected nil tools, got %v", s.Tools)
	}
}

func TestRegisterServiceUpdate(t *testing.T) {
	c := New()
	tools3 := 3
	tools7 := 7
	c.RegisterService("svc", "api", &tools3)
	c.RecordRequest("svc", false)
	c.RecordRequest("svc", true)

	// Re-register with updated transport and tools
	c.RegisterService("svc", "mcp_stdio", &tools7)

	s := c.GetServiceStats("svc")
	if s.Transport != "mcp_stdio" {
		t.Errorf("expected transport 'mcp_stdio', got '%s'", s.Transport)
	}
	if s.Tools == nil || *s.Tools != 7 {
		t.Errorf("expected tools=7, got %v", s.Tools)
	}
	// Counts should be preserved
	if s.Requests != 2 {
		t.Errorf("expected 2 requests, got %d", s.Requests)
	}
	if s.Errors != 1 {
		t.Errorf("expected 1 error, got %d", s.Errors)
	}
}

func TestRecordRequest(t *testing.T) {
	c := New()
	tools := 1
	c.RegisterService("svc", "api", &tools)

	c.RecordRequest("svc", false)
	c.RecordRequest("svc", false)
	c.RecordRequest("svc", true)

	s := c.GetServiceStats("svc")
	if s.Requests != 3 {
		t.Errorf("expected 3 requests, got %d", s.Requests)
	}
	if s.Errors != 1 {
		t.Errorf("expected 1 error, got %d", s.Errors)
	}
}

func TestRecordRequestUnregistered(t *testing.T) {
	c := New()
	// Should not panic
	c.RecordRequest("unknown", false)
	c.RecordRequest("unknown", true)
}

func TestSetStatus(t *testing.T) {
	c := New()
	tools := 1
	c.RegisterService("svc", "api", &tools)

	c.SetStatus("svc", "degraded")
	s := c.GetServiceStats("svc")
	if s.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", s.Status)
	}

	// Unregistered service should not panic
	c.SetStatus("unknown", "disconnected")
}

func TestSetToolCount(t *testing.T) {
	c := New()
	tools := 1
	c.RegisterService("svc", "api", &tools)

	newCount := 10
	c.SetToolCount("svc", &newCount)
	s := c.GetServiceStats("svc")
	if s.Tools == nil || *s.Tools != 10 {
		t.Errorf("expected tools=10, got %v", s.Tools)
	}

	c.SetToolCount("svc", nil)
	s = c.GetServiceStats("svc")
	if s.Tools != nil {
		t.Errorf("expected nil tools, got %v", s.Tools)
	}

	// Unregistered service should not panic
	c.SetToolCount("unknown", &newCount)
}

func TestGetServiceStatsNotFound(t *testing.T) {
	c := New()
	s := c.GetServiceStats("nonexistent")
	if s != nil {
		t.Errorf("expected nil, got %v", s)
	}
}

func TestGetServiceStatsSnapshot(t *testing.T) {
	c := New()
	tools := 1
	c.RegisterService("svc", "api", &tools)
	c.RecordRequest("svc", false)

	s := c.GetServiceStats("svc")
	// Mutating the snapshot should not affect the collector
	s.Requests = 999
	s2 := c.GetServiceStats("svc")
	if s2.Requests != 1 {
		t.Errorf("snapshot mutation affected collector: got %d", s2.Requests)
	}

	// Mutating the Tools pointer in the snapshot should not affect the collector
	*s.Tools = 999
	s3 := c.GetServiceStats("svc")
	if s3.Tools == nil || *s3.Tools != 1 {
		t.Errorf("Tools pointer mutation affected collector: got %v", s3.Tools)
	}
}

func TestGetAllServiceStats(t *testing.T) {
	c := New()
	tools1 := 3
	tools2 := 5
	c.RegisterService("beta", "api", &tools1)
	c.RegisterService("alpha", "mcp_stdio", &tools2)

	all := c.GetAllServiceStats()
	if len(all) != 2 {
		t.Fatalf("expected 2 services, got %d", len(all))
	}
	// Should be sorted by name
	if all[0].Name != "alpha" {
		t.Errorf("expected first service 'alpha', got '%s'", all[0].Name)
	}
	if all[1].Name != "beta" {
		t.Errorf("expected second service 'beta', got '%s'", all[1].Name)
	}
}

func TestGetAllServiceStatsEmpty(t *testing.T) {
	c := New()
	all := c.GetAllServiceStats()
	if len(all) != 0 {
		t.Errorf("expected 0 services, got %d", len(all))
	}
}

func TestGetUptime(t *testing.T) {
	c := New()
	time.Sleep(10 * time.Millisecond)
	uptime := c.GetUptime()
	if uptime < 10*time.Millisecond {
		t.Errorf("expected uptime >= 10ms, got %v", uptime)
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New()
	tools := 5
	c.RegisterService("svc", "api", &tools)

	const goroutines = 100
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				isError := j%10 == 0
				c.RecordRequest("svc", isError)
			}
		}(i)
	}

	wg.Wait()

	s := c.GetServiceStats("svc")
	expectedRequests := int64(goroutines * requestsPerGoroutine)
	if s.Requests != expectedRequests {
		t.Errorf("expected %d requests, got %d", expectedRequests, s.Requests)
	}

	expectedErrors := int64(goroutines * (requestsPerGoroutine / 10))
	if s.Errors != expectedErrors {
		t.Errorf("expected %d errors, got %d", expectedErrors, s.Errors)
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	c := New()

	var wg sync.WaitGroup
	wg.Add(4)

	// Writer 1: Register services
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			tools := i
			c.RegisterService("svc", "api", &tools)
		}
	}()

	// Writer 2: Record requests
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			c.RecordRequest("svc", i%5 == 0)
		}
	}()

	// Writer 3: Set status
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			c.SetStatus("svc", "operational")
		}
	}()

	// Reader: Get stats
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = c.GetServiceStats("svc")
			_ = c.GetAllServiceStats()
			_ = c.GetUptime()
		}
	}()

	wg.Wait()
	// If we got here without a race condition panic, the test passes
}
