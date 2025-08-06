# Fusion Package Quick Start Guide

## 🚀 Get Started in 5 Minutes

This guide gets you up and running with the Fusion package for Microsoft 365 and Google APIs integration.

## 📋 Prerequisites

1. **Go 1.21+** installed
2. **Microsoft 365 account** (for Microsoft APIs)
3. **Google account** (for Google APIs)

## ⚡ Quick Setup

### Step 1: Environment Setup

Create environment file:
```bash
mkdir -p ~/.config/mcpfusion
cat > ~/.config/mcpfusion/.env << 'EOF'
# Microsoft 365 (get from Azure Portal)
MS365_CLIENT_ID=your-microsoft-client-id
MS365_TENANT_ID=your-microsoft-tenant-id

# Google APIs (get from Google Cloud Console) 
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret

# Optional: Production settings
FUSION_LOG_LEVEL=info
FUSION_CACHE_ENABLED=true
FUSION_METRICS_ENABLED=true
EOF

chmod 600 ~/.config/mcpfusion/.env
source ~/.config/mcpfusion/.env
```

### Step 2: Basic Integration

Create `main.go`:
```go
package main

import (
    "log"
    "github.com/PivotLLM/MCPFusion/fusion"
    "github.com/PivotLLM/MCPFusion/mcpserver"
    "github.com/PivotLLM/MCPFusion/mlogger"
)

func main() {
    // Create logger
    logger, _ := mlogger.New()
    defer logger.Close()

    // Create Fusion provider
    fusionProvider := fusion.New(
        fusion.WithJSONConfig("configs/microsoft365.json"),
        fusion.WithJSONConfig("configs/google.json"), 
        fusion.WithLogger(logger),
        fusion.WithInMemoryCache(),
    )

    // Create and start server
    server := mcpserver.New(mcpserver.WithPort(8080))
    server.AddToolProvider(fusionProvider)
    
    log.Println("Server starting on port 8080...")
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Step 3: Run the Server

```bash
# Start the server
go run main.go

# In another terminal, test an endpoint
curl -X POST http://localhost:8080/tools/call \
  -H "Content-Type: application/json" \
  -d '{
    "name": "microsoft365_get_profile",
    "arguments": {}
  }'
```

## 🔐 Authentication Setup

### Microsoft 365 (5 minutes)

1. Go to [Azure Portal](https://portal.azure.com) → App registrations
2. Create new registration:
   - Name: "MCPFusion Integration"
   - Redirect URI: Public client → `https://login.microsoftonline.com/common/oauth2/nativeclient`
3. Copy **Application (client) ID** and **Directory (tenant) ID**
4. Add API permissions:
   - Microsoft Graph → `User.Read`, `Calendars.Read`, `Mail.Read`
5. Grant admin consent

### Google APIs (5 minutes)

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create project → Enable APIs:
   - Google Calendar API
   - Gmail API
   - Google Drive API
3. Credentials → Create OAuth 2.0 Client ID:
   - Application type: Desktop application
   - Name: "MCPFusion Integration"
4. Copy **Client ID** and **Client Secret**

## 📱 First API Call

When you run the server, you'll see OAuth device flow prompts:

```
🔐 Microsoft 365 Authentication Required
✅ Please visit: https://microsoft.com/devicelogin
📝 Enter code: ABC-123-DEF

⏳ Waiting for authentication...
✅ Authentication successful!
📦 Token cached for future use
🚀 MCPFusion server ready!
```

## 🛠️ Available Tools

After authentication, you'll have access to these tools:

### Microsoft 365
- `microsoft365_get_profile` - Get user profile
- `microsoft365_calendar_events` - Get calendar events
- `microsoft365_list_mail` - List email messages
- `microsoft365_get_contacts` - Get contacts

### Google APIs  
- `google_get_profile` - Get user profile
- `google_list_calendar_events` - List calendar events
- `google_list_messages` - List Gmail messages
- `google_list_files` - List Drive files

## 🐳 Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o mcpfusion .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/mcpfusion .
COPY configs/ ./configs/
EXPOSE 8080
CMD ["./mcpfusion"]
```

```bash
# Build and run
docker build -t mcpfusion .
docker run -p 8080:8080 \
  -e MS365_CLIENT_ID=your-client-id \
  -e MS365_TENANT_ID=your-tenant-id \
  mcpfusion
```

## 📚 Next Steps

1. **Read Documentation**: Check [README.md](README.md) for complete features
2. **Configuration Guide**: See [README_CONFIG.md](README_CONFIG.md) for advanced setup
3. **Examples**: Explore [examples/](examples/) for production patterns
4. **Troubleshooting**: Common issues and solutions in configuration guide

## 🆘 Quick Troubleshooting

### Authentication Issues
```bash
# Check environment variables
env | grep -E "(MS365|GOOGLE)"

# Test OAuth endpoints
curl -X POST "https://login.microsoftonline.com/${MS365_TENANT_ID}/oauth2/v2.0/devicecode" \
  -d "client_id=${MS365_CLIENT_ID}&scope=https://graph.microsoft.com/User.Read"
```

### Configuration Issues
```bash
# Validate configuration
curl -X POST http://localhost:8080/admin/validate-config \
  -H "Content-Type: application/json" \
  -d @configs/microsoft365.json
```

### Server Issues
```bash
# Check health
curl http://localhost:8080/health

# View metrics
curl http://localhost:8080/metrics
```

---

🎉 **You're Ready!** Your Fusion-powered MCPFusion server is now running with Microsoft 365 and Google APIs integration.

📧 **Need Help?** Check the full documentation or open an issue for support.