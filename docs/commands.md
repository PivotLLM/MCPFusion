# Command Execution Guide

MCPFusion supports executing system commands and scripts through MCP tools. This enables AI clients to interact with command-line utilities, security tools, automation scripts, and any executable program.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Configuration Structure](#configuration-structure)
- [Parameter Locations](#parameter-locations)
- [Complete Examples](#complete-examples)
- [Response Format](#response-format)
- [Best Practices](#best-practices)
- [Security Considerations](#security-considerations)

## Overview

Command execution in MCPFusion allows you to:

- **Execute Any Command**: Run system commands, scripts, or executables
- **Control Parameters**: Map MCP parameters to command arguments, flags, environment variables, or stdin
- **Manage Execution**: Configure timeouts, working directory, and execution environment
- **Capture Output**: Get structured responses with stdout, stderr, exit code, and execution time
- **Shell Support**: Run commands directly or through shell interpreters (bash, sh, etc.)

## Quick Start

### Basic Command Execution

Create a simple command tool in your JSON config:

```json
{
  "commands": {
    "basic_tools": {
      "name": "Basic Command Tools",
      "description": "Simple command execution examples",
      "commands": [
        {
          "id": "exec",
          "name": "Execute Shell Command",
          "description": "Execute arbitrary shell commands",
          "parameters": [
            {
              "name": "command",
              "description": "Shell command to execute",
              "type": "string",
              "required": true,
              "location": "argument"
            },
            {
              "name": "executable",
              "type": "string",
              "location": "control",
              "default": "/bin/sh",
              "static": true
            },
            {
              "name": "use_shell",
              "type": "boolean",
              "location": "control",
              "default": true,
              "static": true
            }
          ]
        }
      ]
    }
  }
}
```

This creates an MCP tool called `command_exec` that executes shell commands.

### Using the Tool

When an MCP client calls `command_exec`:

```json
{
  "command": "echo 'Hello World'"
}
```

Response:
```
Exit Code: 0
Execution Time: 12ms
Status: Completed successfully

=== STDOUT ===
Hello World

=== STDERR ===
(empty)
```

## Configuration Structure

Commands are configured in a separate `commands` section at the root level of your JSON config:

```json
{
  "services": { ... },
  "commands": {
    "group_id": {
      "name": "Group Display Name",
      "description": "Group description",
      "commands": [
        {
          "id": "command_id",
          "name": "Command Display Name",
          "description": "Command description",
          "parameters": [ ... ]
        }
      ]
    }
  }
}
```

**Tool Naming**: MCP tools are named as `command_{id}` where `{id}` is the command's `id` field.

### Command Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique identifier for the command (used in tool name) |
| `name` | string | Yes | Display name for the MCP tool |
| `description` | string | Yes | Description shown to AI clients |
| `parameters` | array | Yes | Array of parameter configurations |

## Parameter Locations

Parameters can be mapped to different parts of command execution using the `location` field:

### 1. `argument` - Command-Line Arguments

Maps parameters to command-line arguments. Supports prefixes for flag-style arguments.

**Without Prefix:**
```json
{
  "name": "target",
  "type": "string",
  "location": "argument",
  "required": true
}
```
Value `"example.com"` → Command: `nmap example.com`

**With Prefix:**
```json
{
  "name": "ports",
  "type": "string",
  "location": "argument",
  "prefix": "-p"
}
```
Value `"80,443"` → Command: `nmap -p 80,443`

**Boolean Flags:**
```json
{
  "name": "-v",
  "type": "boolean",
  "location": "argument"
}
```
Value `true` → Command: `nmap -v`
Value `false` → Flag not added

### 2. `arglist` - Array Arguments

Expands an array into multiple command-line arguments.

```json
{
  "name": "targets",
  "type": "array",
  "location": "arglist"
}
```
Value `["10.0.0.1", "10.0.0.2"]` → Command: `nmap 10.0.0.1 10.0.0.2`

### 3. `environment` - Environment Variables

Sets environment variables for command execution.

```json
{
  "name": "API_KEY",
  "type": "string",
  "location": "environment"
}
```
Value `"secret123"` → Sets `API_KEY=secret123` in environment

### 4. `stdin` - Standard Input

Sends data to command's standard input.

```json
{
  "name": "input_data",
  "type": "string",
  "location": "stdin"
}
```
Value `"test data"` → Piped to command's stdin

### 5. `control` - Execution Control

Controls how the command is executed. All control parameters are optional.

| Parameter Name | Type | Default | Description |
|----------------|------|---------|-------------|
| `executable` | string | - | Path to executable (required for non-shell commands) |
| `timeout` | integer | 300 | Timeout in seconds (max execution time) |
| `cwd` | string | - | Working directory for command execution |
| `kill_grace_period` | integer | 5 | Grace period in seconds before force-killing |
| `capture_stdout` | boolean | true | Whether to capture stdout |
| `capture_stderr` | boolean | true | Whether to capture stderr |
| `use_shell` | boolean | false | Execute through shell interpreter |
| `shell_interpreter` | string | `/bin/sh` | Shell to use when use_shell is true |

**Example:**
```json
{
  "name": "timeout",
  "type": "integer",
  "location": "control",
  "default": 60,
  "static": true
}
```

## Complete Examples

### Example 1: Network Scanner (nmap)

```json
{
  "commands": {
    "security_tools": {
      "name": "Security Tools",
      "description": "Network security and reconnaissance tools",
      "commands": [
        {
          "id": "nmap",
          "name": "Network Port Scanner",
          "description": "Scan network hosts for open ports and services",
          "parameters": [
            {
              "name": "target",
              "description": "Target host or network (IP, domain, or CIDR)",
              "type": "string",
              "required": true,
              "location": "argument",
              "examples": ["192.168.1.1", "example.com", "10.0.0.0/24"]
            },
            {
              "name": "ports",
              "description": "Port specification (e.g., 80, 1-1000, 80,443)",
              "type": "string",
              "location": "argument",
              "prefix": "-p",
              "default": "1-1000"
            },
            {
              "name": "scan_type",
              "description": "Scan technique",
              "type": "string",
              "location": "argument",
              "enum": ["-sS", "-sT", "-sU", "-sV"],
              "default": "-sS"
            },
            {
              "name": "verbose",
              "description": "Enable verbose output",
              "type": "boolean",
              "location": "argument",
              "prefix": "-v"
            },
            {
              "name": "executable",
              "type": "string",
              "location": "control",
              "default": "/usr/bin/nmap",
              "static": true
            },
            {
              "name": "timeout",
              "type": "integer",
              "location": "control",
              "default": 300,
              "static": true
            },
            {
              "name": "capture_stdout",
              "type": "boolean",
              "location": "control",
              "default": true,
              "static": true
            },
            {
              "name": "capture_stderr",
              "type": "boolean",
              "location": "control",
              "default": true,
              "static": true
            }
          ]
        }
      ]
    }
  }
}
```

**Usage:**
```json
{
  "target": "192.168.1.1",
  "ports": "22,80,443",
  "scan_type": "-sV",
  "verbose": true
}
```

**Executed Command:**
```bash
/usr/bin/nmap -p 22,80,443 -sV -v 192.168.1.1
```

### Example 2: Script with Environment and Stdin

```json
{
  "commands": {
    "data_processing": {
      "name": "Data Processing Scripts",
      "description": "Custom data processing utilities",
      "commands": [
        {
          "id": "process_data",
          "name": "Process JSON Data",
          "description": "Process JSON data through custom script",
          "parameters": [
            {
              "name": "script_path",
              "description": "Path to processing script",
              "type": "string",
              "required": true,
              "location": "argument"
            },
            {
              "name": "config_file",
              "description": "Configuration file path",
              "type": "string",
              "location": "argument",
              "prefix": "--config"
            },
            {
              "name": "data",
              "description": "JSON data to process",
              "type": "string",
              "required": true,
              "location": "stdin"
            },
            {
              "name": "API_TOKEN",
              "description": "API token for external service",
              "type": "string",
              "location": "environment"
            },
            {
              "name": "LOG_LEVEL",
              "description": "Logging level",
              "type": "string",
              "location": "environment",
              "default": "INFO"
            },
            {
              "name": "executable",
              "type": "string",
              "location": "control",
              "default": "/usr/bin/python3",
              "static": true
            },
            {
              "name": "cwd",
              "type": "string",
              "location": "control",
              "default": "/opt/scripts",
              "static": true
            }
          ]
        }
      ]
    }
  }
}
```

**Usage:**
```json
{
  "script_path": "process.py",
  "config_file": "/etc/app/config.json",
  "data": "{\"records\": [1, 2, 3]}",
  "API_TOKEN": "secret123",
  "LOG_LEVEL": "DEBUG"
}
```

**Executed:**
```bash
cd /opt/scripts
API_TOKEN=secret123 LOG_LEVEL=DEBUG /usr/bin/python3 process.py --config /etc/app/config.json
# with stdin: {"records": [1, 2, 3]}
```

### Example 3: Metasploit with Shell Execution

```json
{
  "commands": {
    "exploit_tools": {
      "name": "Exploit Framework",
      "commands": [
        {
          "id": "metasploit",
          "name": "Metasploit Framework",
          "description": "Execute Metasploit modules with resource files",
          "parameters": [
            {
              "name": "module",
              "description": "Metasploit module path",
              "type": "string",
              "required": true,
              "location": "argument"
            },
            {
              "name": "resource_commands",
              "description": "Metasploit resource file commands (one per line)",
              "type": "string",
              "required": true,
              "location": "stdin"
            },
            {
              "name": "executable",
              "type": "string",
              "location": "control",
              "default": "msfconsole -q -r /dev/stdin",
              "static": true
            },
            {
              "name": "use_shell",
              "type": "boolean",
              "location": "control",
              "default": true,
              "static": true
            },
            {
              "name": "timeout",
              "type": "integer",
              "location": "control",
              "default": 600,
              "static": true
            }
          ]
        }
      ]
    }
  }
}
```

## Response Format

All command executions return a structured text response:

```
Exit Code: {code}
Execution Time: {duration}
Status: {status_message}

=== STDOUT ===
{stdout_content}

=== STDERR ===
{stderr_content}
```

**Status Values:**
- `Completed successfully` - Exit code 0
- `Command failed` - Non-zero exit code
- `Timed out` - Execution exceeded timeout

**Example Response:**
```
Exit Code: 0
Execution Time: 245ms
Status: Completed successfully

=== STDOUT ===
Starting Nmap 7.94 ( https://nmap.org )
Nmap scan report for 192.168.1.1
Host is up (0.0012s latency).
PORT    STATE SERVICE
22/tcp  open  ssh
80/tcp  open  http
443/tcp open  https

=== STDERR ===
(empty)
```

## Best Practices

### 1. Use Static Parameters for Security

Mark security-critical parameters as `static: true` so they cannot be changed by MCP clients:

```json
{
  "name": "timeout",
  "type": "integer",
  "location": "control",
  "default": 60,
  "static": true
}
```

### 2. Set Reasonable Timeouts

Always configure timeouts to prevent hanging commands:

```json
{
  "name": "timeout",
  "location": "control",
  "default": 300,
  "static": true
}
```

### 3. Use Absolute Paths

Specify full paths to executables for security and predictability:

```json
{
  "name": "executable",
  "default": "/usr/bin/nmap",
  "static": true
}
```

### 4. Validate Input

Use parameter validation to restrict dangerous inputs:

```json
{
  "name": "target",
  "type": "string",
  "pattern": "^[a-zA-Z0-9.-]+$",
  "examples": ["example.com", "192.168.1.1"]
}
```

### 5. Document Parameters Clearly

Provide clear descriptions and examples:

```json
{
  "name": "ports",
  "description": "Port specification: single port (80), range (1-1000), or list (80,443,8080)",
  "examples": ["80", "1-1000", "80,443,8080"]
}
```

### 6. Group Related Commands

Organize commands into logical groups:

```json
{
  "commands": {
    "security_scanners": { ... },
    "data_processing": { ... },
    "system_utilities": { ... }
  }
}
```

## Security Considerations

### Trusted Client Model

MCPFusion's command execution is designed for **trusted client environments only**:

- **No Sandboxing**: Commands execute with full server privileges
- **No Command Filtering**: Arbitrary command execution if configured
- **No Output Sanitization**: Raw stdout/stderr returned to clients

### Security Best Practices

1. **Use Multi-Tenant Authentication**: Always require API tokens
2. **Restrict Executable Paths**: Use static parameters with absolute paths
3. **Limit Timeout Values**: Prevent resource exhaustion
4. **Validate All Input**: Use patterns, enums, and validation rules
5. **Run with Least Privilege**: Execute MCPFusion with minimal required permissions
6. **Monitor Execution**: Log all command executions for audit trails
7. **Network Isolation**: Run in isolated network environments when possible

### What NOT to Do

❌ **Never expose unrestricted command execution to untrusted clients**
```json
// DANGEROUS - allows arbitrary commands
{
  "name": "command",
  "location": "argument",
  "required": true
}
```

❌ **Never allow clients to override security parameters**
```json
// DANGEROUS - client can disable timeout
{
  "name": "timeout",
  "location": "control",
  "static": false  // BAD!
}
```

❌ **Never use shell execution for untrusted input**
```json
// DANGEROUS - shell injection risk
{
  "name": "use_shell",
  "default": true,
  "static": false  // BAD!
}
```

### Recommended Configuration Template

Safe command configuration template:

```json
{
  "id": "safe_tool",
  "name": "Safe Tool",
  "parameters": [
    {
      "name": "safe_param",
      "type": "string",
      "location": "argument",
      "pattern": "^[a-zA-Z0-9._-]+$",
      "required": true
    },
    {
      "name": "executable",
      "type": "string",
      "location": "control",
      "default": "/usr/local/bin/safe-tool",
      "static": true
    },
    {
      "name": "timeout",
      "type": "integer",
      "location": "control",
      "default": 60,
      "static": true
    },
    {
      "name": "use_shell",
      "type": "boolean",
      "location": "control",
      "default": false,
      "static": true
    }
  ]
}
```

## Testing Commands

Test your command configurations:

```bash
# Run command tests
go test ./fusion -run TestCommand -v

# Test with your config
go test ./fusion -run TestKaliConfig -v
```

See the [Testing Guide](../tests/README.md) for more details.

## Examples

Complete working examples are available in:

- **Kali Linux Tools**: `/configs/kali.json` - 11 security tools (nmap, sqlmap, metasploit, etc.)
- **Basic Commands**: See Quick Start section above
- **Custom Scripts**: See Example 2 (Script with Environment and Stdin)

## Additional Resources

- [Configuration Guide](config.md) - General configuration documentation
- [Token Management](TOKEN_MANAGEMENT.md) - Authentication setup
- [Testing Guide](../tests/README.md) - Testing your commands
- [Architecture](../fusion/README.md) - System design overview
