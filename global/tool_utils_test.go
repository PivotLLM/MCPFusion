/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package global

import (
	"testing"
)

func TestParseToolName(t *testing.T) {
	tests := []struct {
		name         string
		toolName     string
		wantService  string
		wantEndpoint string
		wantErr      bool
	}{
		{
			name:         "valid microsoft365 tool",
			toolName:     "microsoft365_calendar_read_summary",
			wantService:  "microsoft365",
			wantEndpoint: "calendar_read_summary",
			wantErr:      false,
		},
		{
			name:         "valid google tool",
			toolName:     "google_drive_list_files",
			wantService:  "google",
			wantEndpoint: "drive_list_files",
			wantErr:      false,
		},
		{
			name:         "tool with single underscore",
			toolName:     "service_endpoint",
			wantService:  "service",
			wantEndpoint: "endpoint",
			wantErr:      false,
		},
		{
			name:         "tool with multiple underscores in endpoint",
			toolName:     "myservice_complex_endpoint_name_here",
			wantService:  "myservice",
			wantEndpoint: "complex_endpoint_name_here",
			wantErr:      false,
		},
		{
			name:         "empty tool name",
			toolName:     "",
			wantService:  "",
			wantEndpoint: "",
			wantErr:      true,
		},
		{
			name:         "no underscore",
			toolName:     "invalidtoolname",
			wantService:  "",
			wantEndpoint: "",
			wantErr:      true,
		},
		{
			name:         "underscore at start",
			toolName:     "_endpoint",
			wantService:  "",
			wantEndpoint: "",
			wantErr:      true,
		},
		{
			name:         "underscore at end",
			toolName:     "service_",
			wantService:  "",
			wantEndpoint: "",
			wantErr:      true,
		},
		{
			name:         "only underscore",
			toolName:     "_",
			wantService:  "",
			wantEndpoint: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotService, gotEndpoint, err := ParseToolName(tt.toolName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseToolName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotService != tt.wantService {
				t.Errorf("ParseToolName() gotService = %v, want %v", gotService, tt.wantService)
			}
			if gotEndpoint != tt.wantEndpoint {
				t.Errorf("ParseToolName() gotEndpoint = %v, want %v", gotEndpoint, tt.wantEndpoint)
			}
		})
	}
}

func TestExtractServiceFromToolName(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		wantService string
		wantErr     bool
	}{
		{
			name:        "valid microsoft365 tool",
			toolName:    "microsoft365_profile_get",
			wantService: "microsoft365",
			wantErr:     false,
		},
		{
			name:        "invalid tool name",
			toolName:    "notool",
			wantService: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotService, err := ExtractServiceFromToolName(tt.toolName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractServiceFromToolName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotService != tt.wantService {
				t.Errorf("ExtractServiceFromToolName() = %v, want %v", gotService, tt.wantService)
			}
		})
	}
}

func TestBuildToolName(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		endpointID   string
		wantToolName string
	}{
		{
			name:         "microsoft365 calendar",
			serviceName:  "microsoft365",
			endpointID:   "calendar_read_summary",
			wantToolName: "microsoft365_calendar_read_summary",
		},
		{
			name:         "simple names",
			serviceName:  "service",
			endpointID:   "endpoint",
			wantToolName: "service_endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildToolName(tt.serviceName, tt.endpointID); got != tt.wantToolName {
				t.Errorf("BuildToolName() = %v, want %v", got, tt.wantToolName)
			}
		})
	}
}

func TestValidateToolName(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		wantErr  bool
	}{
		{
			name:     "valid tool name",
			toolName: "service_endpoint",
			wantErr:  false,
		},
		{
			name:     "invalid tool name",
			toolName: "nounderscores",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateToolName(tt.toolName); (err != nil) != tt.wantErr {
				t.Errorf("ValidateToolName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
