# MCPFusion

---

**This is a work in progress. It has been released as Open Source to gain feedback and to help the community. Please open a GitHub issue if you encounter any problems.**

Kali users: This implements a network-based MCP server with bearer token authentication that can be installed on a Kali VM.

---

MCPFusion is a configuration-driven MCP (Model Context Protocol) server that enables AI clients to interact with multiple APIs and command-line applications. Applications range from facilitating access to a single API endpoint to allowing arbitrary command-line execution.

The application loads one or more JSON configuration files, which are used to dynamically create MCP tools.

Clients connecting to the MCP server are authenticated using standard bearer tokens. When MCPFusion authenticates to external services, any tokens it obtains—such as OAuth access tokens—are securely stored and associated with the client’s API key. This design allows multiple users or service instances to operate independently; for example, two users can each access their own Microsoft 365 accounts through the same MCPFusion instance by using separate API keys.

Users should carefully review their configuration to understand what access MCPFusion is granted to APIs and command-line tools, and consider the associated security implications. Allowing unrestricted command execution within a controlled security-testing environment may be appropriate, while doing so on production systems could pose unacceptable risks. Use caution and configure MCPFusion in accordance with your security requirements and policies.

## Features

- **Universal API Integration**: Connect to any REST API
- **Command Execution**: Execute system commands and scripts with full parameter control
- **Multi-Tenant Authentication**: Complete tenant isolation with embedded database-backed token management
- **Bearer Token Support**: Industry-standard `Authorization: Bearer <token>` authentication
- **Enhanced Parameter System**: Rich parameter metadata with defaults, validation, and constraints
- **Reliability**: Circuit breakers, retry logic, caching, and comprehensive error handling
- **CLI Token Management**: Command-line token management

## Security Warning

**IMPORTANT**: MCPFusion requires authentication by default using bearer tokens. While a `--no-auth` flag is available for **testing purposes**, this mode is **insecure** and should **not** be used outside trusted environments.

### No-Auth Mode (Testing Only)

The `--no-auth` flag disables authentication requirements:
- **USE CASE**: Local development and testing
- **SECURITY**: All requests will share a single "NOAUTH" tenant context
- **RISK**: Anyone with network access can execute commands and access configured APIs
- **OAUTH TOKENS**: OAuth tokens obtained in no-auth mode are stored with the "NOAUTH" tenant identifier

**For production use, always generate and use proper API tokens** (see Quick Start below).

## Quick Start

1. **Create an environment file**:
/opt/mcpfusion/env is recommended. For example:
 ```
MCP_FUSION_CONFIG=/opt/mcpfusion/microsoft365.json
MCP_FUSION_LISTEN=127.0.0.1:8888
MCP_FUSION_DB_DIR=/opt/mcpfusion/db

# Example for Microsoft 365 Graph API
MS365_CLIENT_ID=<application client ID>
MS365_TENANT_ID=common
 ```

If parameters are provided via the environment, no command-line switches are required. MCPFusion will automatically load /opt/mcpfusion/env into the environment as long as it has permission to read the file.

2. **Build and Generate API Token**:
   ```bash
   # Build the server
   go build -o mcpfusion .
   
   # Generate API token for your application with a description of your choice
   ./mcpfusion -token-add "Token1"
   ```

3. **Start Server and Connect**:
   ```bash
   # Start server
   ./mcpfusion

   # Optionally pass a config and port to the application if not specified in the evironment.
   ./mcpfusion -config configs/microsoft365.json -port 8888

   # For testing only: start without authentication (INSECURE)
   ./mcpfusion --no-auth
   ```
   
### **Client Configuration**

MCPFusion provides both legacy and modern MCP transports simultaneously:

- **Streamable HTTP Transport (modern)**: `http://localhost:8888/mcp`

- **SSE Transport (legacy)**: `http://localhost:8888/sse`

**Authentication**: All endpoints require the API token as a Bearer token in the Authorization header:
  `Authorization: Bearer <TOKEN>`

See [Client Configuration Guide](docs/clients.md) and [Token Management Guide](docs/TOKEN_MANAGEMENT.md) for detailed
setup instructions.

## Documentation

### **Configuration & Setup**

- **[Configuration Guide](docs/config.md)** - Complete guide to creating JSON configurations for any API
-  **[Command Execution Guide](docs/commands.md)** - Execute system commands and scripts with parameter control
-  **[Client Integration](docs/clients.md)** - Connect Cline, Claude Desktop, and custom MCP clients
-  **[Token Management Guide](docs/TOKEN_MANAGEMENT.md)** - Multi-tenant authentication and CLI usage
-  **[Microsoft 365 Setup](docs/Microsoft365.md)** - Microsoft 365 setup
-  **[Google APIs Setup](docs/Google_Workspace.md)** - Google Workspace setup
-  **[HTTP Session Management](docs/HTTP_SESSION_MANAGEMENT.md)** - Connection pooling, timeouts, and reliability features

## Copyright and license

Copyright (c) 2025 by Tenebris Technologies Inc. and available for use under Apache License 2.0. Please see the LICENSE file for full information.

## No Warranty (zilch, none, void, nil, null, "", {}, 0x00, 0b00000000, EOF)

THIS SOFTWARE IS PROVIDED “AS IS,” WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

Made in Canada

