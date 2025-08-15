# Microsoft 365 API Setup Guide for MCPFusion

This guide walks you through setting up MCPFusion to connect to Microsoft 365 APIs for accessing calendar, mail, contacts, and profile information.

## Prerequisites

- Microsoft 365 account (work, school, or personal)
- Admin access to Azure Portal (for app registration)
- MCPFusion installed and built

## Step 1: Azure App Registration

### 1.1 Access Azure Portal

1. Navigate to [Azure Portal](https://portal.azure.com)
2. Sign in with your Microsoft 365 admin account

### 1.2 Create New App Registration

1. Go to **Azure Active Directory** → **App registrations**
2. Click **"New registration"**
3. Configure the application:
   - **Name**: `MCPFusion` (or your preferred name)
   - **Supported account types**: Choose based on your needs:
     - `Accounts in this organizational directory only` - Single tenant (your org only)
     - `Accounts in any organizational directory` - Multitenant (any work/school account)
     - `Accounts in any organizational directory and personal Microsoft accounts` - All users
   - **Redirect URI**: Leave blank (not needed for device flow)
4. Click **"Register"**

### 1.3 Save Application IDs

After registration, save these values:
- **Application (client) ID**: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
- **Directory (tenant) ID**: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

You'll find these on the app's Overview page.

### 1.4 Configure API Permissions

1. In your app registration, go to **"API permissions"**
2. Click **"Add a permission"**
3. Select **"Microsoft Graph"**
4. Choose **"Delegated permissions"**
5. Add the following permissions:

   **Essential Permissions:**
   - `User.Read` - Sign in and read user profile
   - `Calendars.Read` - Read user calendars
   - `Mail.Read` - Read user mail
   - `Contacts.Read` - Read user contacts

   **Optional Permissions (add if needed):**
   - `Calendars.ReadWrite` - Create and modify calendar events
   - `Mail.Send` - Send mail as the user
   - `Contacts.ReadWrite` - Create and modify contacts
   - `Files.Read` - Read OneDrive files
   - `Sites.Read.All` - Read SharePoint sites

6. Click **"Add permissions"**

### 1.5 Enable Device Code Flow

1. Go to **"Authentication"** in the left menu
2. Scroll to **"Advanced settings"**
3. Set **"Allow public client flows"** to **Yes**
4. Click **"Save"**

### 1.6 (Optional) Grant Admin Consent

If you're setting this up for your organization:
1. Return to **"API permissions"**
2. Click **"Grant admin consent for [Your Organization]"**
3. Confirm the action

This prevents users from needing to individually consent to permissions.

## Step 2: Configure Environment Variables

### 2.1 Create Configuration File

Create a `.mcp` file in your home directory:

```bash
# Linux/Mac
nano ~/.mcp

# Windows
notepad %USERPROFILE%\.mcp
```

### 2.2 Add Your Configuration

Add the following content with your actual IDs:

```bash
# Microsoft 365 Configuration
MS365_CLIENT_ID=your-application-client-id-here
MS365_TENANT_ID=your-directory-tenant-id-here

# Optional: Specify tenant type
# MS365_TENANT_ID=common           # Multitenant (default)
# MS365_TENANT_ID=organizations    # Work/school accounts only
# MS365_TENANT_ID=consumers        # Personal Microsoft accounts only
# MS365_TENANT_ID=your-tenant-id   # Specific tenant only
```

**Example with actual values:**
```bash
MS365_CLIENT_ID=a1b2c3d4-e5f6-7890-abcd-ef1234567890
MS365_TENANT_ID=12345678-90ab-cdef-1234-567890abcdef
```

### 2.3 Secure the File (Linux/Mac)

```bash
chmod 600 ~/.mcp
```

## Step 3: Run MCPFusion with Microsoft 365

### 3.1 Build MCPFusion

```bash
cd /path/to/MCPFusion
go build -o mcpfusion .
```

### 3.2 Start the Server

```bash
./mcpfusion -config configs/microsoft365.json -port 8888
```

You should see output like:
```
2025-01-07 10:00:00 MCP [INFO] Loading configuration from: configs/microsoft365.json
2025-01-07 10:00:00 MCP [INFO] Registered 13 dynamic tools from configuration
2025-01-07 10:00:00 MCP [INFO] MCP server listening on http://localhost:8888
```

## Step 4: First-Time Authentication

### 4.1 Device Code Flow

When you first use a Microsoft 365 tool, you'll see a device code prompt:

```
Please visit https://microsoft.com/devicelogin and enter code: ABCD1234
```

### 4.2 Complete Authentication

1. Open https://microsoft.com/devicelogin in your browser
2. Enter the provided code (e.g., `ABCD1234`)
3. Sign in with your Microsoft 365 account
4. Review the requested permissions
5. Click **"Accept"** to grant permissions

### 4.3 Token Storage

After successful authentication:
- Tokens are cached automatically
- No need to re-authenticate for subsequent requests
- Tokens refresh automatically when expired

## Step 5: Available Microsoft 365 Tools

Once configured, these 13 MCP tools are available:

### 5.1 Profile Management
**Tool**: `microsoft365_profile_get`
**Description**: Get your Microsoft 365 profile information
**Parameters**: 
- `$select` (optional): Fields to include (default: displayName,mail,userPrincipalName,jobTitle,department,companyName)

### 5.2 Calendar Management

**List All Calendars**
**Tool**: `microsoft365_calendars_list`
**Description**: Get all user calendars
**Parameters**:
- `$select` (optional): Fields to include (default: name,id,owner,isDefaultCalendar)
- `$top` (optional): Number of calendars (default: 1000)

**Calendar Events (All Calendars)**
**Tool**: `microsoft365_calendar_read_summary`
**Description**: Get calendar events with basic information
**Parameters**:
- `startDate` (required): Start date in YYYYMMDD format
- `endDate` (required): End date in YYYYMMDD format
- `$select` (optional): Fields to include (default: subject,start,end)
- `$top` (optional): Number of events (default: 100)

**Tool**: `microsoft365_calendar_read_details`
**Description**: Get calendar events with full details
**Parameters**:
- `startDate` (required): Start date in YYYYMMDD format
- `endDate` (required): End date in YYYYMMDD format  
- `$select` (optional): Fields to include (default: subject,body,bodyPreview,organizer,attendees,start,end,location)
- `$top` (optional): Number of events (default: 10)

**Calendar Events (Specific Calendar)**
**Tool**: `microsoft365_calendar_events_read_summary`
**Description**: Get events from a specific calendar (summary)
**Parameters**:
- `calendarId` (required): Calendar ID to retrieve events from
- `startDate` (optional): Start date in YYYYMMDD format
- `endDate` (optional): End date in YYYYMMDD format
- `$select` (optional): Fields to include (default: subject,start,end)
- `$top` (optional): Number of events (default: 100)

**Tool**: `microsoft365_calendar_events_read_details`
**Description**: Get events from a specific calendar (detailed)
**Parameters**:
- `calendarId` (required): Calendar ID to retrieve events from
- `startDate` (optional): Start date in YYYYMMDD format
- `endDate` (optional): End date in YYYYMMDD format
- `$select` (optional): Fields to include (default: subject,body,bodyPreview,organizer,attendees,start,end,location)
- `$top` (optional): Number of events (default: 10)

**Individual Calendar Event**
**Tool**: `microsoft365_calendar_read_event`
**Description**: Get a specific calendar event by ID
**Parameters**:
- `id` (required): Event ID to retrieve
- `$select` (optional): Fields to include

### 5.3 Mail Management

**List Mail Folders**
**Tool**: `microsoft365_mail_folders_list`
**Description**: Get all mail folders for the user
**Parameters**:
- `$select` (optional): Fields to include (default: displayName,id,parentFolderId,childFolderCount,unreadItemCount,totalItemCount)
- `$top` (optional): Number of folders (default: 1000)

**Mail Messages (Inbox)**
**Tool**: `microsoft365_mail_read_inbox`
**Description**: Get inbox messages with basic information
**Parameters**:
- `$top` (optional): Number of messages (default: 10, max: 1000)
- `$select` (optional): Fields to include (default: subject,from,receivedDateTime,bodyPreview,isRead)
- `$filter` (optional): Filter expression (e.g., 'isRead eq false')

**Mail Messages (Specific Folder)**
**Tool**: `microsoft365_mail_folder_messages`
**Description**: Get messages from a specific mail folder
**Parameters**:
- `folderId` (required): Mail folder ID to retrieve messages from
- `$top` (optional): Number of messages (default: 10, max: 1000)
- `$select` (optional): Fields to include (default: subject,from,receivedDateTime,bodyPreview,isRead)
- `$filter` (optional): Filter expression (e.g., 'isRead eq false')

**Individual Mail Message**
**Tool**: `microsoft365_mail_read_message`
**Description**: Get a specific email message by ID
**Parameters**:
- `id` (required): Message ID to retrieve
- `$select` (optional): Fields to include

### 5.4 Contacts Management

**List Contacts**
**Tool**: `microsoft365_contacts_list`
**Description**: Get contacts from the user's address book
**Parameters**:
- `$top` (optional): Number of contacts (default: 25, max: 1000)
- `$select` (optional): Fields to include (default: displayName,emailAddresses,businessPhones,jobTitle,companyName)
- `$filter` (optional): Filter expression

**Individual Contact**
**Tool**: `microsoft365_contacts_read_contact`
**Description**: Get a specific contact by ID
**Parameters**:
- `id` (required): Contact ID to retrieve
- `$select` (optional): Fields to include

## Step 6: Testing the Integration

### 6.1 Using curl

Test profile retrieval:
```bash
curl -X POST http://localhost:8888 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "microsoft365_profile",
      "arguments": {}
    },
    "id": 1
  }'
```

Test calendar retrieval:
```bash
curl -X POST http://localhost:8888 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "microsoft365_calendar_read_summary",
      "arguments": {
        "startDate": "20250101",
        "endDate": "20250131"
      }
    },
    "id": 2
  }'
```

### 6.2 Using an MCP Client

Any MCP-compatible client can connect to `http://localhost:8888` and use the Microsoft 365 tools.

## Step 7: Troubleshooting

### Common Issues and Solutions

#### 7.1 Authentication Errors

**Problem**: "AADSTS7000218: The request body must contain the following parameter: 'client_assertion' or 'client_secret'"

**Solution**: Ensure "Allow public client flows" is enabled in Azure Portal → Authentication → Advanced settings

#### 7.2 Permission Errors

**Problem**: "Insufficient privileges to complete the operation"

**Solutions**:
1. Verify all required permissions are added in Azure Portal
2. Grant admin consent if in an organization
3. Re-authenticate to refresh permissions

#### 7.3 Token Issues

**Problem**: "Token expired" or authentication prompts repeatedly

**Solutions**:
1. Tokens auto-refresh, but if issues persist:
   ```bash
   # Clear token cache (location varies by implementation)
   rm -rf ~/.mcp_tokens/
   ```
2. Re-authenticate using device code flow

#### 7.4 Network Errors

**Problem**: Cannot connect to Microsoft Graph API

**Solutions**:
1. Check internet connectivity
2. Verify firewall allows HTTPS to:
   - `login.microsoftonline.com`
   - `graph.microsoft.com`
3. Check proxy settings if behind corporate firewall

#### 7.5 Configuration Issues

**Problem**: "MS365_CLIENT_ID environment variable not found"

**Solutions**:
1. Verify `.mcp` file exists in home directory
2. Check file permissions (should be readable)
3. Ensure no typos in environment variable names
4. Restart MCPFusion after changing `.mcp` file

### Debug Mode

Enable debug logging for troubleshooting:
```bash
./mcpfusion -config configs/microsoft365.json -debug
```

## Step 8: Security Best Practices

### 8.1 Environment Variables
- Never commit `.mcp` file to version control
- Use `.gitignore` to exclude sensitive files
- Consider using a secrets management system in production

### 8.2 Permissions
- Only request permissions you actually need
- Use least-privilege principle
- Regular audit of granted permissions

### 8.3 Token Security
- Tokens are stored in memory by default
- Consider encrypted storage for production
- Implement token rotation policies

### 8.4 Network Security
- Always use HTTPS in production
- Consider using TLS client certificates
- Implement rate limiting for API calls

## Step 9: Advanced Configuration

### 9.1 Custom Scopes

Modify `fusion/configs/microsoft365.json` to add custom scopes:
```json
"scope": [
  "https://graph.microsoft.com/User.Read",
  "https://graph.microsoft.com/Calendars.Read",
  "https://graph.microsoft.com/Tasks.Read"  // Add new scope
]
```

### 9.2 Proxy Configuration

For corporate proxies, set environment variables:
```bash
export HTTP_PROXY=http://proxy.company.com:8080
export HTTPS_PROXY=http://proxy.company.com:8080
```

### 9.3 Custom Endpoints

Add new Microsoft Graph endpoints to the configuration:
```json
{
  "id": "tasks_list",
  "name": "List Tasks",
  "method": "GET",
  "path": "/me/todo/lists/{listId}/tasks",
  "parameters": [...]
}
```

## Step 10: Production Deployment

### 10.1 Docker Deployment

Create a Dockerfile:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o mcpfusion .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/mcpfusion .
COPY fusion/configs fusion/configs
CMD ["./mcpfusion", "-fusion-config", "fusion/configs/microsoft365.json"]
```

### 10.2 Kubernetes Deployment

Use ConfigMaps for configuration:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-config
data:
  MS365_CLIENT_ID: "your-client-id"
  MS365_TENANT_ID: "your-tenant-id"
```

### 10.3 Health Checks

MCPFusion includes health check endpoints:
- `/health` - Basic health check
- `/ready` - Readiness probe

## Additional Resources

- [Microsoft Graph API Documentation](https://docs.microsoft.com/graph/overview)
- [Azure AD App Registration Guide](https://docs.microsoft.com/azure/active-directory/develop/quickstart-register-app)
- [OAuth 2.0 Device Code Flow](https://docs.microsoft.com/azure/active-directory/develop/v2-oauth2-device-code)
- [MCPFusion Documentation](README.md)
- [Fusion Configuration Guide](fusion/README_CONFIG.md)

## Support

For issues specific to:
- **MCPFusion**: Open an issue on the GitHub repository
- **Microsoft 365 APIs**: Check [Microsoft Graph Support](https://docs.microsoft.com/graph/support)
- **Azure AD**: Visit [Azure Support](https://azure.microsoft.com/support/)

---

*Last updated: January 2025*