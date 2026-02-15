/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package global

// Transport display names for metrics and health reporting.
// These are normalized forms used consistently across the application for
// identifying transport types in metrics, health status, and logging.
//
// Note: Config files use different values (e.g., "stdio" vs "mcp_stdio")
// which are defined in fusion.TransportType constants.
const (
	TransportAPI       = "api"        // Standard HTTP/REST API services
	TransportMCPStdio  = "mcp_stdio"  // MCP over stdio transport
	TransportMCPSSE    = "mcp_sse"    // MCP over Server-Sent Events
	TransportMCPHTTP   = "mcp_http"   // MCP over HTTP transport
	TransportInternal  = "internal"   // Internal services (e.g., knowledge store)
)

// Service and server status values for health reporting.
const (
	StatusOperational  = "operational"  // Service is functioning normally
	StatusDegraded     = "degraded"     // Service is impaired but functional
	StatusDisconnected = "disconnected" // Service is not connected
	StatusHealthy      = "healthy"      // Overall server health is good
)
