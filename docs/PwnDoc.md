# PwnDoc Integration Guide

PwnDoc is a pentest reporting platform that stores audit data (findings, sections, vulnerabilities) in MongoDB and generates DOCX reports via a Node.js backend. MCPFusion provides an MCP tool interface for all major PwnDoc operations.

## Table of Contents

- [Overview](#overview)
- [Configuration](#configuration)
- [Authentication](#authentication)
- [Available Tools](#available-tools)
- [Field Reference](#field-reference)
- [Known Issues and Considerations](#known-issues-and-considerations)

## Overview

MCPFusion connects to PwnDoc using session JWT authentication. Credentials are provided via environment variables and MCPFusion handles token acquisition, caching, and refresh automatically. All PwnDoc entities (audits, findings, vulnerabilities, clients, companies, users, templates, sections) are accessible through the MCP tool interface.

## Configuration

Set the following environment variables before starting MCPFusion:

| Variable | Description |
|----------|-------------|
| `PWNDOC_URL` | Base URL of the PwnDoc instance (e.g. `https://pwndoc.example.com:8443`) |
| `PWNDOC_USERNAME` | PwnDoc username |
| `PWNDOC_PASSWORD` | PwnDoc password |

The configuration file is `configs/pwndoc.json`. The `baseURL` field references `${PWNDOC_URL}`, so the environment variable must include the protocol and port but no trailing slash.

## Authentication

MCPFusion uses PwnDoc's session JWT mechanism:

1. On first use, MCPFusion posts credentials to `/api/users/token` and receives a JWT.
2. The JWT is stored as a `token` cookie and cached in the MCPFusion database.
3. For subsequent requests, the cached token is applied as a cookie header.
4. On 401 responses, the token is invalidated and re-acquired automatically.
5. Token refresh uses `GET /api/users/refreshtoken` (GET only — POST is not supported by PwnDoc's API).

The default token expiry is 3600 seconds. MCPFusion refreshes the token before it expires.

## Available Tools

### Audit Operations

| Tool | Description |
|------|-------------|
| `pwndoc_list_audits` | List all audits |
| `pwndoc_get_audit` | Get audit metadata with finding and section summaries |
| `pwndoc_get_audit_general` | Get audit general information |
| `pwndoc_get_audit_network` | Get audit network/scope information |
| `pwndoc_get_audit_sections` | Get all sections with full content for an audit |
| `pwndoc_list_audit_sections` | List sections with name and ID only |
| `pwndoc_create_audit` | Create a new audit |
| `pwndoc_delete_audit` | Delete an audit |
| `pwndoc_update_audit_general` | Update audit general fields |
| `pwndoc_update_audit_network` | Update audit network/scope |
| `pwndoc_update_audit_sections` | Update one or more audit sections |
| `pwndoc_toggle_audit_approval` | Toggle audit approval state |
| `pwndoc_update_review_status` | Update the review status |
| `pwndoc_generate_audit_report` | Generate and download the audit DOCX report |
| `pwndoc_sort_findings` | Sort findings within an audit |

### Finding Operations

| Tool | Description |
|------|-------------|
| `pwndoc_list_audit_findings` | List all findings for an audit |
| `pwndoc_get_finding` | Get full details of a single finding |
| `pwndoc_create_finding` | Create a new finding in an audit |
| `pwndoc_update_finding` | Update an existing finding |
| `pwndoc_delete_finding` | Delete a finding |
| `pwndoc_move_finding` | Move a finding to a different audit |

### Vulnerability Template Operations

| Tool | Description |
|------|-------------|
| `pwndoc_list_vulnerabilities` | List vulnerability templates |
| `pwndoc_get_vulnerability` | Get full details of a vulnerability template |
| `pwndoc_create_vulnerability` | Create a new vulnerability template |
| `pwndoc_update_vulnerability` | Update a vulnerability template |
| `pwndoc_delete_vulnerability` | Delete a vulnerability template |
| `pwndoc_export_vulnerabilities` | Export all vulnerability templates as JSON |

### Client and Company Operations

| Tool | Description |
|------|-------------|
| `pwndoc_list_clients` | List clients |
| `pwndoc_create_client` | Create a client |
| `pwndoc_update_client` | Update a client |
| `pwndoc_delete_client` | Delete a client |
| `pwndoc_list_companies` | List companies |
| `pwndoc_create_company` | Create a company |
| `pwndoc_update_company` | Update a company |
| `pwndoc_delete_company` | Delete a company |

### User and Settings Operations

| Tool | Description |
|------|-------------|
| `pwndoc_list_users` | List users |
| `pwndoc_get_user` | Get a specific user |
| `pwndoc_get_current_user` | Get the currently authenticated user |
| `pwndoc_create_user` | Create a user |
| `pwndoc_update_user` | Update a user |
| `pwndoc_update_current_user` | Update the current user's profile |
| `pwndoc_list_reviewers` | List users eligible as reviewers |
| `pwndoc_get_settings` | Get PwnDoc settings |
| `pwndoc_update_settings` | Update PwnDoc settings |
| `pwndoc_get_public_settings` | Get public settings |
| `pwndoc_get_statistics` | Get usage statistics |

### Reference Data

| Tool | Description |
|------|-------------|
| `pwndoc_list_audit_types` | List audit types |
| `pwndoc_create_audit_type` | Create an audit type |
| `pwndoc_update_audit_type` | Update an audit type |
| `pwndoc_delete_audit_type` | Delete an audit type |
| `pwndoc_list_languages` | List languages |
| `pwndoc_create_language` | Create a language |
| `pwndoc_update_language` | Update a language |
| `pwndoc_delete_language` | Delete a language |
| `pwndoc_list_templates` | List report templates |
| `pwndoc_delete_template` | Delete a template |
| `pwndoc_list_sections` | List available sections |
| `pwndoc_list_custom_fields` | List custom field definitions |
| `pwndoc_list_vulnerability_categories` | List vulnerability categories |
| `pwndoc_list_vulnerability_types` | List vulnerability types |

## Field Reference

### Custom Fields

Custom fields are user-defined fields attached to findings, sections, and vulnerability templates. Each custom field definition has an `_id`, `label`, `fieldType`, and `description`.

**Critical: The `customFields` parameter requires fully embedded objects.** PwnDoc's report generator dereferences the `customField` sub-object at render time using `field.customField.label.toLowerCase()`. If the `customField` key is absent or is not a full object, the report generator will throw a null-pointer exception and fail.

Three formats are possible. Only Format 1 is correct:

| Format | Structure | Result |
|--------|-----------|--------|
| Format 1 (correct) | `{"customField": {"_id": "...", "label": "...", "fieldType": "...", "description": "..."}, "text": "value"}` | Report generates successfully |
| Format 2 (incorrect) | `{"customField": "object-id-string", "text": "value"}` | Report generator crashes on `label.toLowerCase()` |
| Format 3 (incorrect) | `{"customField": {"_id": "..."}, "text": "value"}` | Report generator crashes on `label.toLowerCase()` |

To obtain the full embedded object for a custom field, use `pwndoc_list_custom_fields`. The response includes the complete field definition including `_id`, `label`, `fieldType`, and `description` — copy the entire object as the value of the `customField` key.

MCPFusion enforces Format 1 via the `validate_object_fields:customField._id,customField.label` transform on all `customFields` parameters. Requests that provide Format 2 or Format 3 are rejected before reaching PwnDoc with a descriptive error.

### HTML Fields

Findings and vulnerability templates contain several HTML fields: `description`, `observation`, `remediation`, and `poc` (findings only). PwnDoc's report generator converts these fields to OOXML using a SAX-based parser (`html2ooxml.js`).

**Issue:** The SAX parser cannot handle whitespace-only text nodes between HTML elements (inter-element whitespace). When an AI model or editor inserts a newline between closing and opening tags (e.g. `</p>\n<p>`), the parser encounters a text node containing only whitespace between two block elements, which causes a null-pointer exception during OOXML generation.

MCPFusion applies the `html_compact` transform to all top-level HTML string fields in findings, which strips these whitespace-only inter-element text nodes before the data is sent to PwnDoc.

### Vulnerability Details Array

Vulnerability templates use a `details` array instead of flat HTML fields. Each element in the array represents a locale and contains:

| Field | Type | Description |
|-------|------|-------------|
| `locale` | string | Language code (e.g. `en`, `fr`) |
| `title` | string | Vulnerability title |
| `vulnType` | string | Vulnerability type |
| `description` | string | HTML description |
| `observation` | string | HTML observation |
| `remediation` | string | HTML remediation steps |
| `references` | array of strings | External references |
| `customFields` | array of objects | Custom field values (see above) |

Because `description`, `observation`, and `remediation` are nested inside objects within an array, the flat `html_compact` transform does not reach them. MCPFusion applies the `html_compact_fields:description,observation,remediation` transform on the `details` parameter, which walks each element and compacts those three fields.

### Sections

Audit sections contain HTML content and optional custom fields. The `update_audit_sections` endpoint requires a `section_id` (obtained from `pwndoc_get_audit` or `pwndoc_list_audit_sections`) and accepts `text` (HTML) and `customFields` (array of fully-embedded objects as described above).

There is no endpoint to create or delete sections — sections are defined by the audit type template and are fixed per audit. Sections can only be updated.

### Findings vs. Vulnerability Templates

PwnDoc distinguishes between:

- **Findings**: Instances within a specific audit. Created via `pwndoc_create_finding`. Contain audit-specific fields like `status`, `identifier`, `scope`, and `poc`.
- **Vulnerability templates**: Reusable templates stored globally. Created via `pwndoc_create_vulnerability`. Contain locale-based `details` arrays.

When creating a finding, you provide flat fields (`description`, `observation`, `remediation` directly on the finding object). When creating or updating a vulnerability template, you provide a `details` array of locale objects.

## Known Issues and Considerations

### 1. Custom Field Format Enforcement

**Issue:** PwnDoc's report generator requires fully embedded custom field objects (Format 1). There is no server-side validation — PwnDoc accepts all three formats but crashes at report generation time.

**MCPFusion mitigation:** The `validate_object_fields:customField._id,customField.label` transform rejects requests that do not provide Format 1. This catches the error at the API gateway level rather than at report generation time.

**Remaining consideration:** If a user bypasses MCPFusion (e.g. direct API access) and stores Format 2 or Format 3 data, report generation will fail. PwnDoc itself does not validate this constraint.

### 2. HTML Inter-Element Whitespace

**Issue:** PwnDoc's `html2ooxml.js` SAX parser crashes on whitespace-only text nodes between block elements. AI models frequently produce HTML with newlines between elements (e.g. `<p>text</p>\n<p>more text</p>`).

**MCPFusion mitigation:** The `html_compact` transform strips these whitespace nodes from all top-level HTML fields in findings and sections before the request is sent. The `html_compact_fields` transform does the same for HTML fields nested inside vulnerability `details` array elements.

**Remaining consideration:** If HTML is stored in PwnDoc directly (not through MCPFusion) and contains inter-element whitespace, report generation may fail. PwnDoc does not sanitize HTML on write.

### 3. Vulnerability Template Retrieval

**Issue:** PwnDoc does not provide a `GET /api/vulnerabilities/:id` endpoint. Only a list endpoint exists (`GET /api/vulnerabilities/:locale`), which returns slim records without all fields.

**MCPFusion mitigation:** `pwndoc_get_vulnerability` fetches the full list and uses a JQ filter to extract the record by `_id`. This is less efficient than a direct lookup but is the only available approach given the API.

### 4. Report Generation Timeout

Report generation (DOCX export) can take a significant amount of time for large audits with many findings. MCPFusion applies a 300-second timeout to `pwndoc_generate_audit_report`, which is distinct from the default connection timeout applied to all other endpoints.

### 5. Token Refresh Endpoint

PwnDoc's token refresh endpoint (`/api/users/refreshtoken`) accepts only GET requests. Sending a POST results in a 404. MCPFusion is configured to use GET for this endpoint.

### 6. Retry Configuration

PwnDoc endpoints use a conservative retry configuration. Only `network_error` and `server_error` conditions trigger retries. Transport-level timeouts (`timeout`) do **not** trigger retries for PwnDoc, because retrying a timed-out report generation or write operation could result in duplicate data.
