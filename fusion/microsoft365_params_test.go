/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"net/http"
	"strings"
	"testing"
)

func TestMicrosoft365ParameterAliases(t *testing.T) {
	// Load the Microsoft365 configuration
	config, err := LoadConfigFromFile("../configs/microsoft365.json")
	if err != nil {
		t.Skipf("Skipping test - could not load microsoft365.json: %v", err)
	}

	// Check that all $ parameters have aliases
	for serviceName, service := range config.Services {
		for _, endpoint := range service.Endpoints {
			for _, param := range endpoint.Parameters {
				if strings.HasPrefix(param.Name, "$") {
					if param.Alias == "" {
						t.Errorf("Parameter '%s' in endpoint %s.%s has no alias",
							param.Name, serviceName, endpoint.ID)
					} else {
						// Verify the alias is MCP-compliant
						if !IsValidMCPParameterName(param.Alias) {
							t.Errorf("Alias '%s' for parameter '%s' in %s.%s is not MCP-compliant",
								param.Alias, param.Name, serviceName, endpoint.ID)
						}

						// Verify the alias is the parameter name without $
						expectedAlias := strings.TrimPrefix(param.Name, "$")
						if param.Alias != expectedAlias {
							t.Logf("Note: Alias '%s' differs from expected '%s' for parameter '%s' in %s.%s",
								param.Alias, expectedAlias, param.Name, serviceName, endpoint.ID)
						}
					}
				}
			}
		}
	}
}

func TestMicrosoft365ToolGeneration(t *testing.T) {
	// Load the configuration
	config, err := LoadConfigFromFile("../configs/microsoft365.json")
	if err != nil {
		t.Skipf("Skipping test - could not load config: %v", err)
	}

	// Create a Fusion instance with Microsoft365 config
	fusion, err := New(
		WithConfig(config),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Get the registered tools
	tools := fusion.RegisterTools()

	// Check that tools were generated
	if len(tools) == 0 {
		t.Error("No tools were generated from Microsoft365 configuration")
	}

	// Check that all tool parameters are MCP-compliant
	for _, tool := range tools {
		for _, param := range tool.Parameters {
			if !IsValidMCPParameterName(param.Name) {
				t.Errorf("Tool '%s' has non-MCP-compliant parameter '%s'",
					tool.Name, param.Name)
			}

			// Verify no $ prefixes in parameter names
			if strings.HasPrefix(param.Name, "$") {
				t.Errorf("Tool '%s' has parameter with $ prefix: '%s'",
					tool.Name, param.Name)
			}
		}
	}

	t.Logf("Successfully generated %d tools with MCP-compliant parameters", len(tools))
}

// m365Endpoint returns the endpoint id of the shipped Microsoft 365 config.
func m365Endpoint(t *testing.T, id string) *EndpointConfig {
	t.Helper()
	config, err := LoadConfigFromFile("../configs/microsoft365.json")
	if err != nil {
		t.Fatalf("load microsoft365.json: %v", err)
	}
	for _, svc := range config.Services {
		for i := range svc.Endpoints {
			if svc.Endpoints[i].ID == id {
				return &svc.Endpoints[i]
			}
		}
	}
	t.Fatalf("endpoint %s not found", id)
	return nil
}

// mail_read_message always asks Microsoft Graph for a plain-text body, which
// is far smaller than the HTML one, and does not offer the header to callers.
func TestMicrosoft365ReadMessageAsksForTextBody(t *testing.T) {
	ep := m365Endpoint(t, "mail_read_message")
	mapper := NewMapper()
	for name, args := range map[string]map[string]any{
		"no arguments":         {"id": "m1"},
		"a caller's own value": {"id": "m1", "Prefer": `outlook.body-content-type="html"`},
	} {
		req, err := http.NewRequest(http.MethodGet, "https://graph.microsoft.com/v1.0/me/messages/m1", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := mapper.ApplyHeaders(req, ep.Parameters, args); err != nil {
			t.Fatalf("%s: ApplyHeaders: %v", name, err)
		}
		if got := req.Header.Get("Prefer"); got != `outlook.body-content-type="text"` {
			t.Errorf("%s: Prefer = %q", name, got)
		}
	}
	for name := range mapper.ConvertToMCPParameters(ep.Parameters) {
		if strings.EqualFold(name, "Prefer") {
			t.Errorf("the static Prefer header is offered as a tool parameter")
		}
	}
}

// mail_folder_messages can be sorted and paged like the inbox tool.
func TestMicrosoft365FolderMessagesSortAndPage(t *testing.T) {
	ep := m365Endpoint(t, "mail_folder_messages")
	have := map[string]ParameterConfig{}
	for _, p := range ep.Parameters {
		have[p.Name] = p
	}
	for _, name := range []string{"$top", "$skip", "$filter", "$orderby"} {
		if _, ok := have[name]; !ok {
			t.Errorf("mail_folder_messages has no %s parameter", name)
		}
	}
	// No default order: Graph refuses a sort on a property the filter does
	// not name, so a default would break calls that filter on something else.
	if p := have["$orderby"]; p.Default != nil || p.Required {
		t.Errorf("$orderby = %+v; want optional with no default", p)
	}
}
