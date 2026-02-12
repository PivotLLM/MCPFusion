# Google Workspace Integration

MCPFusion provides access to Google Workspace APIs (Calendar, Gmail, Drive, and Contacts) through MCP tools. This guide covers setup from scratch.

## Prerequisites

- A Google account
- MCPFusion server running with a valid API token
- The `fusion-auth` helper tool (built from `cmd/auth/`)

## Step 1: Create a Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Click the project dropdown at the top of the page (next to "Google Cloud")
3. Click **New Project**
4. Enter a project name (e.g., "MCPFusion")
5. Click **Create**
6. Make sure the new project is selected in the project dropdown

## Step 2: Enable Required APIs

From the Google Cloud Console with your project selected:

1. Go to **APIs & Services > Library** (or search for "API Library" in the search bar)
2. Search for and enable each of the following APIs:
   - **Google Calendar API**
   - **Gmail API**
   - **Google Drive API**
   - **Google People API** (this is the Contacts API)
3. Click on each API and click **Enable**

## Step 3: Configure the OAuth Consent Screen and Create Credentials

1. Go to **APIs & Services > OAuth consent screen**
2. Select a user type:
   - **Internal** — if you're using a Google Workspace account and only need access for your own organization (recommended for personal/team use)
   - **External** — if you're using a personal Gmail account or need users outside your organization to authenticate
3. Click **Create**
4. Fill in the required fields:
   - **App name**: MCPFusion (or your preferred name)
   - **User support email**: your email address
   - **Developer contact information**: your email address
5. Click **Save and Continue**
6. You will be prompted to **Create OAuth client**:
   - Select **Desktop app** as the application type
   - Enter a name (e.g., "MCPFusion Desktop")
   - Click **Create**
7. Save the **Client ID** and **Client Secret** shown in the confirmation

## Step 4: Configure OAuth Scopes

1. From the **OAuth consent screen** overview, click **Data Access** in the left menu
2. Click **Add or Remove Scopes**
3. Add the following scopes:
   - `https://www.googleapis.com/auth/userinfo.email`
   - `https://www.googleapis.com/auth/userinfo.profile`
   - `https://www.googleapis.com/auth/calendar`
   - `https://www.googleapis.com/auth/gmail.modify`
   - `https://www.googleapis.com/auth/drive`
   - `https://www.googleapis.com/auth/contacts.readonly`
4. Click **Update**, then **Save**

> **Note**: For Internal apps, all users in the organization can authenticate. For External apps in "Testing" status, you must add test users under **Audience** in the left menu — only those users can authenticate. You do not need to go through Google's verification process for personal use.

> **Note**: Even if scopes are not pre-registered on the consent screen, the fusion-auth helper will request them at authorization time. However, registering them avoids warnings during the consent flow.

## Step 5: Set Environment Variables

Add the following to the MCPFusion environment file (default: `/opt/mcpfusion/env`):

```bash
GOOGLE_CLIENT_ID=your-client-id-here.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret-here
```

Restart MCPFusion after updating the environment file so it picks up the new values.

## Step 6: Build the fusion-auth Helper

Build the helper from the MCPFusion source:

```bash
cd cmd/auth
go build -o fusion-auth .
```

Alternatively, copy a pre-built `fusion-auth` binary to the machine where you will run it (see Step 7).

## Step 7: Authenticate

### Why fusion-auth is needed

Most OAuth providers support the device authorization flow, which is convenient for CLI tools and headless servers — the user visits a URL on any device and enters a code. However, Google severely restricts the scopes available to the device flow, excluding Calendar, Gmail, Drive, and Contacts. This makes the device flow unusable for MCPFusion's Google Workspace integration.

Instead, `fusion-auth` uses the browser-based authorization code flow with PKCE, which supports all required scopes.

### Important: Where to run fusion-auth

The `fusion-auth` helper uses browser-based OAuth authentication. During the flow, Google redirects the browser to a temporary local callback server (`http://127.0.0.1:<port>/callback`) started by fusion-auth. **This means fusion-auth must run on the same machine where the browser is open** — the OAuth callback is local to that machine.

Common scenarios:

| Setup | How to run fusion-auth |
|-------|----------------------|
| **MCPFusion on the same machine** | Run fusion-auth directly, pointing to `http://localhost:8888` |
| **MCPFusion on a remote server** | Run fusion-auth on your **local machine** (where your browser is), pointing to the remote server's address (e.g., `http://10.0.0.5:8888`) |
| **Remote desktop session** | Run fusion-auth within the remote desktop session where the browser is available |

The `-fusion` URL must be reachable from the machine running fusion-auth. If MCPFusion is on a different host, use its IP address or hostname and ensure the port is accessible over the network.

### Running fusion-auth (auth code mode — recommended)

On the MCPFusion server, generate an auth code:

```bash
./mcpfusion -auth-code google -auth-url http://10.0.0.5:8888
```

This prints a single blob (base64url-encoded). Copy it and run fusion-auth on your local machine (where the browser is):

```bash
./fusion-auth eyJhbGciOi...
```

The blob contains the server URL, service name, and a time-limited auth code (15 minutes). No other parameters are needed.

If multiple API tokens exist on the server, use `-auth-token` to specify which tenant:

```bash
./mcpfusion -auth-code google -auth-url http://10.0.0.5:8888 -auth-token abc12345
```

### Running fusion-auth (manual mode)

You can also specify parameters individually:

```bash
./fusion-auth -service google -fusion <mcpfusion-url> -token <your-mcpfusion-api-token>
```

Parameters:
- `-service google` — specifies the Google OAuth provider
- `-fusion <mcpfusion-url>` — the URL of your MCPFusion server (e.g., `http://localhost:8888`, `http://10.0.0.5:8888`)
- `-token <your-mcpfusion-api-token>` — a valid MCPFusion API token (generate one with `./mcpfusion -token-add` on the server)
- `-verbose` — (optional) enable detailed logging for troubleshooting

### Example: MCPFusion on a remote server

```bash
# Auth code mode (recommended):
./mcpfusion -auth-code google -auth-url http://10.0.0.5:8888
# Copy the blob, then on your local machine:
./fusion-auth <blob>

# Manual mode:
./fusion-auth -service google -fusion http://10.0.0.5:8888 -token abc123def456
```

### What happens

1. fusion-auth starts a temporary local HTTP server on a random port
2. Your default browser opens to Google's consent page
3. You sign in and authorize the requested permissions
4. Google redirects the browser to the local callback (`http://127.0.0.1:<port>/callback`)
5. fusion-auth receives the authorization code, exchanges it for tokens
6. Tokens are securely pushed to the MCPFusion server
7. MCPFusion stores the tokens and uses them for subsequent API calls

## Step 8: Verify

Once authenticated, the following MCP tools become available:

| Category | Tools |
|----------|-------|
| **Profile** | `google_profile_get` |
| **Calendar** | `google_calendar_events_list`, `google_calendar_event_create`, `google_calendar_event_get`, `google_calendar_event_update`, `google_calendar_search`, `google_calendar_list` |
| **Gmail (read)** | `google_gmail_messages_list`, `google_gmail_message_get`, `google_gmail_message_read`, `google_gmail_search_messages` |
| **Gmail (drafts)** | `google_gmail_draft_create`, `google_gmail_draft_get`, `google_gmail_draft_update`, `google_gmail_draft_delete`, `google_gmail_draft_list`, `google_gmail_draft_reply`, `google_gmail_draft_reply_all`, `google_gmail_draft_forward` |
| **Gmail (organize)** | `google_gmail_labels_list`, `google_gmail_label_create`, `google_gmail_message_move` |
| **Drive** | `google_drive_files_list`, `google_drive_file_get`, `google_drive_file_download`, `google_drive_file_create`, `google_drive_file_delete`, `google_drive_file_share` |
| **Contacts** | `google_contacts_list`, `google_contacts_get`, `google_contacts_search` |

> **Note**: Gmail tools support reading, drafting, and organizing emails. Sending is not supported by design.

## Troubleshooting

### "The OAuth client was not found"
- Verify `GOOGLE_CLIENT_ID` is set correctly and matches the Client ID from Step 4
- Make sure the environment variable is available to the MCPFusion server process

### "Access blocked: This app's request is invalid"
- Ensure you selected **Desktop app** as the application type in Step 4
- Verify the OAuth consent screen is configured (Step 3)

### "Access denied" or 403 errors
- Make sure your email is added as a test user (Step 3, item 8)
- Verify all required APIs are enabled (Step 2)
- Check that the OAuth consent screen includes all required scopes (Step 3, item 6)

### Token refresh issues
- Google access tokens expire after 1 hour
- MCPFusion automatically refreshes tokens using the stored refresh token
- If refresh fails, re-run the fusion-auth helper to re-authenticate

## Architecture Notes

Google restricts OAuth2 device flow scopes, so MCPFusion uses the `fusion-auth` helper for browser-based authorization code flow with PKCE instead. The helper starts a temporary local HTTP server to receive the OAuth callback, exchanges the authorization code for tokens, and pushes them to the MCPFusion server's token storage API.

The Google People API (Contacts) uses a different base URL (`people.googleapis.com`) than other Google APIs (`www.googleapis.com`). This is handled via per-endpoint `baseURL` overrides in the configuration file.
