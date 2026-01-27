/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package config

// GetServiceConfigs returns default service configurations with hardcoded credentials
func GetServiceConfigs() map[string]*ServiceConfig {
	return map[string]*ServiceConfig{
		"google": {
			DisplayName:  "Google",
			ClientID:     "714108368582-cpadlbijda7594iqptevpds09ie4l17o.apps.googleusercontent.com",
			ClientSecret: "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",
			Endpoints: &EndpointConfig{
				AuthorizationURL: "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL:         "https://oauth2.googleapis.com/token",
			},
			Scope: "https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/calendar https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/drive",
			Metadata: map[string]string{
				"description": "Access to Google services including Calendar, Gmail, and Drive",
			},
		},
		"github": {
			DisplayName:  "GitHub",
			ClientID:     "YOUR_GITHUB_CLIENT_ID_HERE",
			ClientSecret: "YOUR_GITHUB_CLIENT_SECRET_HERE",
			Metadata: map[string]string{
				"description": "Access to GitHub repositories and user information",
			},
		},
		"microsoft365": {
			DisplayName: "Microsoft 365",
			ClientID:    "YOUR_MS365_CLIENT_ID_HERE",
			TenantID:    "YOUR_MS365_TENANT_ID_HERE",
			Endpoints: &EndpointConfig{
				AuthorizationURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				TokenURL:         "https://login.microsoftonline.com/common/oauth2/v2.0/token",
				DeviceCodeURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode",
			},
			Metadata: map[string]string{
				"description": "Access to Microsoft 365 services including Outlook, OneDrive, and Teams",
			},
		},
	}
}
