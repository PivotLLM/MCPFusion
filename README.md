# MCPFusion

---

**This is a work in progress. It has been released as Open Source to obtain community feedback and to help those who require a trustworthy network-based MCP server. Please open a GitHub issue if you encounter any problems.**

**Kali Linux™ users: This project implements a standards-based network MCP server with bearer token authentication that can be installed on a Kali host or VM to facilitate the use of command line utilities by MCP clients.**

---

MCPFusion is a configuration-driven MCP (Model Context Protocol) server that enables AI clients to interact with multiple APIs and command-line applications. Applications range from facilitating access to a single API endpoint to allowing command-line execution. This project evolved from the author's desire to maintain code for one MCP server.

The application loads one or more JSON configuration files which define MCP tools.

Clients connecting to the MCP server are authenticated using standard bearer tokens. When MCPFusion authenticates to external services, any tokens it obtains—such as OAuth access tokens—are securely stored and associated with the client’s API key. This design allows multiple users or service instances to operate independently; for example, two users can each access their own Microsoft 365 accounts through the same MCPFusion instance by using separate API keys.

Users should carefully review their configuration to understand what access MCPFusion is granted to APIs and command-line tools, and consider the security implications. Allowing unrestricted command execution within a controlled security-testing environment may be appropriate, while doing so on production systems likely poses unacceptable risks. Use caution and configure MCPFusion in accordance with your security requirements and policies.

## Features

- **Universal API Integration**: Connect to any REST API
- **Command Execution**: Execute system commands and scripts with full parameter control
- **Multi-Tenant Authentication**: Token-based tenant isolation with embedded database-backed token management
- **Bearer Token Support**: Industry-standard `Authorization: Bearer <token>` authentication
- **Enhanced Parameter System**: Rich parameter metadata with defaults, validation, and constraints
- **Reliability**: Circuit breakers, retry logic, caching, and error handling
- **CLI Token Management**: Command-line token management
- **User Management**: Stable user identity with UUID-based accounts, API key linking, and automatic migration of existing tokens
- **Knowledge Store**: Per-user persistent knowledge storage with domain/key organization, exposed as native MCP tools

## To Do

- Decide on an appropriate approach to HTTPS implementation
- (What else do we need in a comprehensive universal MCP server solution?)

## Security

### Environment

MCPFusion is intended for use in controlled environments on private networks. To reduce setup friction in these scenarios, HTTP is permitted. Do not expose MCPFusion over the public internet without TLS and authentication. If you require HTTPS, place MCPFusion behind a reverse proxy (e.g., NGINX) or load balancer to terminate TLS.

### HTTPS

The author is contemplating built-in HTTPS support, including:
	•	Simple configuration to reference a certificate and key
	•	Certbot integration
	•	Implementing the ACME protocol

Suggestions and contributions are welcome.

### Authentication

Authentication currently uses long-lived bearer tokens, a common and proven API pattern. The author is aware of proposals to standardize MCP authentication via OAuth/OIDC and will continue to track this development. However, for typical localhost, private-network, or single-user deployments, requiring an external IdP introduces unnecessary cost and complexity without a proportional security benefit. In practice, controls that are disproportionate to the risk and costly to operate are often bypassed or misconfigured, yielding a weaker security posture than woudl be achieved by a simpler easy to deploy mechanism.

MCPFusion maintains an independent tenant context within it's embedded database for each API key. If used to access one or more APIs that require the user to authenticate using OAuth, the resulting tokens are stored within the tenant context for the API key. For these use cases, it is essential that a unique API key is generated for each user of the system to achieve isolation. Otherwise, one user may be granted access to an API using oauth tokens belonging to a different user, resulting in significant security and privacy issues. Similarly, if access to multiple accounts is desired, simply generate a different MCPFusion API key for each.

### No-Auth Mode

The `--no-auth` flag disables authentication and is intended for testing, debugging, and pootentially temporarily working around broken MCP clients. This mode is **insecure** and should be used with **extreme caution.***

**CAUTION:** In addition to providing all available MCP tools and resources to any client able to connect to the TCP port, all unauthicated requests share the same "NOAUTH" tenant context. Please review the information in the preceeding section on authentication. If MCPFusion is used to access a service that requires a user to authenticate using OAuth (for example Microsoft365), the separation that would normally exist by virtue of different FusionMCP API tokens will not be present. ** This could have severe security and privacy implications. ** 

## Quick Start

1. **Create an environment file**:
/opt/mcpfusion/env is recommended. For example:
 ```
MCP_FUSION_CONFIG=/opt/mcpfusion/microsoft365.json
MCP_FUSION_LISTEN=127.0.0.1:8888
MCP_FUSION_DB_DIR=/opt/mcpfusion/db

# Example for Microsoft 365 Graph API registration
MS365_CLIENT_ID=<application client ID>
MS365_TENANT_ID=common
 ```

If parameters are provided via the environment, no command-line switches are required. MCPFusion will automatically load /opt/mcpfusion/env into the environment as long as it has permission to read the file.

Linux users, please see the example mcpfusion.service file. In many Linux distros, this can be copied to /etc/systemd/system. Note the the user and group on lines 8 and 9 respectively need to be updated or created, and if you wish to use a location other than /opt/mcpfusion you will need to adjust paths throughout the file. Please remember to run `sudo systemctl daemon-reload` after creating or modifying a .service file.

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

**Authentication**: Unless disabled using --no-auth, both endpoints require a Bearer token in the Authorization header:
  `Authorization: Bearer <TOKEN>`

For clients unable to set custom HTTP headers, or those with unnecessarily restrictive support for network-based MCP servers, users may wish to consider bridging between MCP stdio transport and network-based MCP servers. In this case, https://github.com/PivotLLM/MCPRelay may be helpful.

## Included Configurations

MCPFusion ships with JSON configurations for two services in the `configs/` directory. These can be used as-is or customized to suit your needs.

**Microsoft 365** (`configs/microsoft365.json`) — Provides calendar management (read, create, update, search across calendars), email (inbox, folders, search, drafts with reply/reply-all/forward, folder creation, message moves), contacts (list, read, search), and OneDrive file access (browse, search, read, download). Uses OAuth2 device code flow for authentication.

**Google Workspace** (`configs/google-workspace.json`) — Provides calendar management (list, create, update, search events, list calendars), Gmail (read, search, drafts with reply/reply-all/forward, label management, message organization), Google Drive (browse, read, download, create, delete, share files), and Contacts (list, read, search). Uses browser-based OAuth2 authorization code flow via the `fusion-auth` helper.

Both configurations are designed to support reading and drafting email but not sending, as a safety measure. Users who want to send email can add the appropriate scopes and endpoints.

## Development

PRs are welcome as long as the submitter states that submissions are consistent with Apache License 2.0.

The author acknoledges the use of Claude® Code to assist with assigned dvelopment tasks.

## Documentation

### **Configuration & Setup**

- **[Configuration Guide](docs/config.md)** - Complete guide to creating JSON configurations for any API
-  **[Command Execution Guide](docs/commands.md)** - Execute system commands and scripts with parameter control
-  **[Client Integration](docs/clients.md)** - Connect Cline, Claude Desktop, and custom MCP clients
-  **[Token Management Guide](docs/TOKEN_MANAGEMENT.md)** - Multi-tenant authentication and CLI usage
-  **[User & Knowledge Management](docs/user_management.md)** - User accounts, API key linking, and persistent knowledge store
-  **[Microsoft 365 Setup](docs/Microsoft365.md)** - Microsoft 365 setup
-  **[Google APIs Setup](docs/Google_Workspace.md)** - Google Workspace setup
-  **[HTTP Session Management](docs/HTTP_SESSION_MANAGEMENT.md)** - Connection pooling, timeouts, and reliability features

## Copyright and license

Copyright (c) 2025-2026 by Tenebris Technologies Inc. This software is licensed under the MIT License. Please see LICENSE for details.

## Trademarks

“Kali Linux” is a trademark of OffSec Services Limited. “Linux” is a registered trademark of Linus Torvalds. "Claude" is a registered trademark of Anthropic PBC. Any references are for identification only and do not imply sponsorship, endorsement, or affiliation.

## No Warranty (zilch, none, void, nil, null, "", {}, 0x00, 0b00000000, EOF)

THIS SOFTWARE IS PROVIDED “AS IS,” WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

Made in Canada
