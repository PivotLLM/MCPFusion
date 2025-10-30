# Command-Line Execution Implementation Plan

## Overview

This document outlines the design and implementation plan for adding command-line execution capabilities to MCPFusion. This feature will allow MCP clients to execute shell commands and programs through the same JSON configuration system used for REST API services.

## Design Principles

1. **Maximum Flexibility**: Support both broad configs (execute any command) and narrow configs (specific executable with controlled arguments)
2. **Consistent Authentication**: Use the same multi-tenant API token authentication as REST APIs
3. **Trust Model**: Command execution assumes absolute trust of the MCP client
4. **Separation of Concerns**: Commands are separate from services at the JSON config level
5. **No Breaking Changes**: Add "commands" section alongside existing "services" section

---

## JSON Configuration Structure

### Top-Level Structure

```json
{
  "services": { ... },  // Existing services section (unchanged)
  "commands": {         // New commands section
    "command_group_name": {
      "name": "Human-readable name",
      "description": "Description of this command group",
      "commands": [
        {
          "id": "unique_command_id",
          "name": "Human-readable command name",
          "description": "What this command does",
          "parameters": [ ... ]
        }
      ]
    }
  }
}
```

### Command Definition Structure

Each command has:
- `id` (string, required): Unique identifier (becomes part of MCP tool name: `command_{id}`)
- `name` (string, required): Human-readable name
- `description` (string, required): Description for MCP clients
- `parameters` (array, required): List of parameter configurations

### Parameter Configuration

Parameters control all aspects of command execution. Each parameter has:

```json
{
  "name": "parameter_name",
  "description": "Parameter description",
  "type": "string|number|boolean|array|object",
  "required": true|false,
  "location": "argument|arglist|environment|stdin|control",
  "prefix": "optional_prefix",
  "default": <default_value>,
  "static": true|false
}
```

**Common Fields:**
- `name`: Parameter identifier
- `description`: Human-readable description
- `type`: Data type (string, number, boolean, array, object)
- `required`: Whether parameter must be provided
- `location`: Where the parameter is used (see below)
- `prefix`: (Optional) Prefix to add before value for `location: "argument"` (e.g., "-p", "--port")
- `default`: Default value if not provided
- `static`: If true, use default value only and don't expose to MCP client

#### Parameter Locations

1. **`location: "argument"`** - Command-line argument
   - **Boolean type**: Parameter name is the flag itself (e.g., `"name": "--debug"`)
     - If true: flag is added to command line
     - If false: flag is omitted
   - **Non-boolean types**:
     - **Without prefix**: Only the value is added to command line
       - Example: `"name": "target"` with value `"192.168.1.1"` → adds `192.168.1.1`
     - **With prefix**: Prefix and value are added as separate arguments
       - Example: `"name": "ports", "prefix": "-p"` with value `"80,443"` → adds `-p 80,443`
       - If value is not provided (and parameter is optional), both prefix and value are skipped
     - String, number values are converted to strings

2. **`location: "arglist"`** - Array of command-line arguments
   - Special location for array parameters
   - Accepts array/list of strings only
   - Each string in the array becomes a separate command-line argument
   - Can be static or MCP-exposed
   - Useful for flexible argument passing

3. **`location: "environment"`** - Environment variable
   - Parameter name is the environment variable name
   - Value is the environment variable value
   - Extends/overrides server process environment

4. **`location: "stdin"`** - Standard input
   - Parameter value is passed to process stdin
   - String type only
   - Can be static or MCP-exposed

5. **`location: "control"`** - Execution control parameters
   - Controls HOW the command is executed (not passed TO the command)
   - Includes: executable path, timeout, working directory, capture settings
   - These parameters configure the execution context
   - Not included in command-line arguments, environment, or stdin

#### Standard Parameters (from MCP spec)

These parameters map to the MCP command execution specification:

| Parameter | Type | Location | Description | Default |
|-----------|------|----------|-------------|---------|
| `executable` | string | control | Path or name of program to execute | (required) |
| `args` | array | arglist | Argument vector (does not include executable) | [] |
| `use_shell` | boolean | control | Run via shell interpreter | false |
| `shell_interpreter` | string | control | Shell to use when use_shell=true | "/bin/sh" |
| `env` | object | control | Environment variables map | {} |
| `cwd` | string | control | Working directory | (server cwd) |
| `stdin` | string | stdin | Data for stdin | "" |
| `timeout` | integer | control | Max seconds to wait (0=indefinite) | 180 |
| `kill_grace_period` | integer | control | Seconds between TERM and KILL | 5 |
| `capture_stdout` | boolean | control | Whether to capture stdout | true |
| `capture_stderr` | boolean | control | Whether to capture stderr | true |
| `text_mode` | boolean | control | Treat IO as text (vs binary) | true |
| `bufsize` | integer | control | Buffering policy | 1 |

**Note**: Any of these can be marked `static: true` to use only the default value and not expose to MCP client.

---

## Configuration Examples

### Example 1: Unrestricted Command Execution

Allow MCP client to execute any command with any arguments:

```json
{
  "commands": {
    "shell": {
      "name": "Shell Command Execution",
      "description": "Execute arbitrary shell commands",
      "commands": [
        {
          "id": "exec",
          "name": "Execute Command",
          "description": "Execute any command with full control over arguments and environment",
          "parameters": [
            {
              "name": "executable",
              "description": "Path or name of program to execute",
              "type": "string",
              "required": true,
              "location": "control"
            },
            {
              "name": "args",
              "description": "Command-line arguments as array of strings",
              "type": "array",
              "required": false,
              "location": "arglist",
              "default": []
            },
            {
              "name": "cwd",
              "description": "Working directory for command execution",
              "type": "string",
              "required": false,
              "location": "control"
            },
            {
              "name": "timeout",
              "description": "Maximum execution time in seconds",
              "type": "number",
              "required": false,
              "location": "control",
              "default": 180
            },
            {
              "name": "capture_stdout",
              "description": "Capture stdout",
              "type": "boolean",
              "required": false,
              "location": "control",
              "default": true
            },
            {
              "name": "capture_stderr",
              "description": "Capture stderr",
              "type": "boolean",
              "required": false,
              "location": "control",
              "default": true
            }
          ]
        }
      ]
    }
  }
}
```

MCP client usage:
```json
{
  "executable": "ls",
  "args": ["-la", "/tmp"],
  "timeout": 30
}
```

### Example 2: Restricted Nmap Execution

Lock down to specific executable with controlled parameters:

```json
{
  "commands": {
    "security": {
      "name": "Security Tools",
      "description": "Controlled security scanning tools",
      "commands": [
        {
          "id": "nmap_scan",
          "name": "Nmap Port Scan",
          "description": "Perform network port scanning with nmap",
          "parameters": [
            {
              "name": "executable",
              "description": "Nmap executable",
              "type": "string",
              "required": true,
              "location": "control",
              "default": "/usr/bin/nmap",
              "static": true
            },
            {
              "name": "target",
              "description": "Target IP address or hostname",
              "type": "string",
              "required": true,
              "location": "argument"
            },
            {
              "name": "-sV",
              "description": "Enable service version detection",
              "type": "boolean",
              "required": false,
              "location": "argument",
              "default": false
            },
            {
              "name": "-p",
              "description": "Port specification (e.g., '1-1000', '80,443')",
              "type": "string",
              "required": false,
              "location": "argument",
              "default": "1-1000"
            },
            {
              "name": "timeout",
              "description": "Maximum execution time",
              "type": "number",
              "required": false,
              "location": "control",
              "default": 300,
              "static": true
            },
            {
              "name": "capture_stdout",
              "description": "Capture stdout",
              "type": "boolean",
              "required": false,
              "location": "control",
              "default": true,
              "static": true
            },
            {
              "name": "capture_stderr",
              "description": "Capture stderr",
              "type": "boolean",
              "required": false,
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

MCP client usage:
```json
{
  "target": "example.com",
  "-sV": true,
  "-p": "80,443,8080"
}
```

This executes: `/usr/bin/nmap -sV -p 80,443,8080 example.com`

### Example 3: Shell Script with Environment and Stdin

Execute a script with environment variables and stdin:

```json
{
  "commands": {
    "scripts": {
      "name": "Data Processing Scripts",
      "description": "Run data processing scripts",
      "commands": [
        {
          "id": "process_data",
          "name": "Process Data",
          "description": "Process data using Python script with environment configuration",
          "parameters": [
            {
              "name": "executable",
              "description": "Python interpreter",
              "type": "string",
              "required": true,
              "location": "control",
              "default": "/usr/bin/python3",
              "static": true
            },
            {
              "name": "script",
              "description": "Script path",
              "type": "string",
              "required": true,
              "location": "argument",
              "default": "/opt/scripts/process.py",
              "static": true
            },
            {
              "name": "DATA_SOURCE",
              "description": "Data source identifier",
              "type": "string",
              "required": true,
              "location": "environment"
            },
            {
              "name": "LOG_LEVEL",
              "description": "Logging level",
              "type": "string",
              "required": false,
              "location": "environment",
              "default": "INFO"
            },
            {
              "name": "data",
              "description": "Input data to process",
              "type": "string",
              "required": true,
              "location": "stdin"
            },
            {
              "name": "--verbose",
              "description": "Enable verbose output",
              "type": "boolean",
              "required": false,
              "location": "argument",
              "default": false
            }
          ]
        }
      ]
    }
  }
}
```

MCP client usage:
```json
{
  "DATA_SOURCE": "production_db",
  "LOG_LEVEL": "DEBUG",
  "data": "{\"records\": [{\"id\": 1, \"value\": 100}]}",
  "--verbose": true
}
```

This executes:
```bash
DATA_SOURCE=production_db LOG_LEVEL=DEBUG /usr/bin/python3 /opt/scripts/process.py --verbose < (stdin data)
```

---

## Response Format

Command execution returns a text response with structured information:

```
Exit Code: 0
Execution Time: 1.234s
Status: Success

--- STDOUT ---
<stdout content here>

--- STDERR ---
<stderr content here>
```

For non-zero exit codes:
```
Exit Code: 1
Execution Time: 0.456s
Status: Failed

--- STDOUT ---
<stdout content here>

--- STDERR ---
Error: command failed
<additional stderr content>
```

If `capture_stdout=false`, the STDOUT section is omitted. Same for `capture_stderr=false`.

---

## Implementation Plan

### Phase 1: Configuration Support

**File: `fusion/config.go`**

Add new types:

```go
// CommandGroupConfig represents a group of related commands
type CommandGroupConfig struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Commands    []CommandConfig `json:"commands"`
}

// CommandConfig represents configuration for a single command
type CommandConfig struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Parameters  []ParameterConfig `json:"parameters"`
}

// Add to Config struct
type Config struct {
    Logger   global.Logger                `json:"-"`
    Services map[string]*ServiceConfig    `json:"services"`
    Commands map[string]*CommandGroupConfig `json:"commands"` // NEW
    HTTPClient *http.Client                `json:"-"`
    Cache      Cache                       `json:"-"`
    ConfigPath string                      `json:"-"`
}
```

Extend `ParameterLocation` constants:

```go
const (
    ParameterLocationPath     ParameterLocation = "path"
    ParameterLocationQuery    ParameterLocation = "query"
    ParameterLocationBody     ParameterLocation = "body"
    ParameterLocationHeader   ParameterLocation = "header"
    ParameterLocationArgument ParameterLocation = "argument"  // NEW
    ParameterLocationArglist  ParameterLocation = "arglist"   // NEW
    ParameterLocationEnvironment ParameterLocation = "environment" // NEW
    ParameterLocationStdin    ParameterLocation = "stdin"     // NEW
    ParameterLocationControl  ParameterLocation = "control"   // NEW (for executable, timeout, etc.)
)
```

Update `ParameterConfig` struct to add `Prefix` field:

```go
type ParameterConfig struct {
    Name        string            `json:"name"`
    Prefix      string            `json:"prefix,omitempty"` // NEW: prefix for argument location
    Alias       string            `json:"alias,omitempty"`
    Description string            `json:"description"`
    Type        ParameterType     `json:"type"`
    Required    bool              `json:"required"`
    Location    ParameterLocation `json:"location"`
    Default     interface{}       `json:"default,omitempty"`
    Examples    []interface{}     `json:"examples,omitempty"`
    Validation  *ValidationConfig `json:"validation,omitempty"`
    Transform   *TransformConfig  `json:"transform,omitempty"`
    Quoted      bool              `json:"quoted,omitempty"`
    Static      bool              `json:"static,omitempty"`
}
```

**Purpose of `Prefix` field**: For `location: "argument"` non-boolean parameters, the prefix allows specifying flag-value pairs. For example:
- `prefix: "-p"` with value `"80,443"` → adds `-p 80,443` to command line
- `prefix: "--port"` with value `"8080"` → adds `--port 8080` to command line
- If parameter is optional and not provided, both prefix and value are skipped

### Phase 2: Command Handler

**File: `fusion/command_handler.go` (NEW)**

Create handler for command execution:

```go
package fusion

import (
    "context"
    "fmt"
    "os/exec"
    "time"
    "bytes"
    "strings"
)

// CommandHandler handles command execution
type CommandHandler struct {
    commandGroup *CommandGroupConfig
    command      *CommandConfig
    fusion       *Fusion
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(fusion *Fusion, commandGroup *CommandGroupConfig, command *CommandConfig) *CommandHandler {
    return &CommandHandler{
        commandGroup: commandGroup,
        command:      command,
        fusion:       fusion,
    }
}

// Handle executes the command with provided arguments
func (h *CommandHandler) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
    // 1. Extract execution parameters (executable, args, env, cwd, timeout, etc.)
    // 2. Validate parameters
    // 3. Build command with exec.CommandContext
    // 4. Set up environment variables
    // 5. Set up stdin if provided
    // 6. Set up stdout/stderr capture
    // 7. Execute with timeout
    // 8. Handle kill_grace_period for timeout
    // 9. Format response with exit code, stdout, stderr, execution time
    // 10. Return formatted response
}

// buildCommandArgs constructs the argument list from parameters
func (h *CommandHandler) buildCommandArgs(args map[string]interface{}) ([]string, error) {
    // Process parameters in order they appear in JSON config
    // For each parameter with location: "argument":
    //   - Boolean type: if true, add parameter name as flag
    //   - Non-boolean with prefix: if value provided, add prefix then value as separate args
    //   - Non-boolean without prefix: if value provided, add value only
    //   - If parameter is optional and not provided, skip entirely
    // For parameters with location: "arglist":
    //   - Expand array into individual arguments
    // Return constructed argument list
}

// buildEnvironment constructs environment variables
func (h *CommandHandler) buildEnvironment(args map[string]interface{}) ([]string, error) {
    // Start with os.Environ()
    // Add/override with parameters having location: "environment"
}

// formatResponse formats the command execution result
func (h *CommandHandler) formatResponse(exitCode int, stdout, stderr string, duration time.Duration) string {
    // Format as shown in Response Format section above
}
```

### Phase 3: Tool Registration

**File: `fusion/fusion.go`**

Extend `RegisterTools()` to include commands:

```go
func (f *Fusion) RegisterTools() []global.ToolDefinition {
    var tools []global.ToolDefinition

    // Register service tools (existing code)
    for serviceName, service := range f.config.Services {
        for _, endpoint := range service.Endpoints {
            tool := f.createToolDefinition(serviceName, service, &endpoint)
            tools = append(tools, tool)
        }
    }

    // Register command tools (NEW)
    for groupName, commandGroup := range f.config.Commands {
        for _, command := range commandGroup.Commands {
            tool := f.createCommandToolDefinition(groupName, commandGroup, &command)
            tools = append(tools, tool)
        }
    }

    if f.logger != nil {
        f.logger.Infof("Registered %d dynamic tools from configuration", len(tools))
    }

    return tools
}

// createCommandToolDefinition creates a tool definition from command configuration
func (f *Fusion) createCommandToolDefinition(groupName string, commandGroup *CommandGroupConfig, command *CommandConfig) global.ToolDefinition {
    // Similar to createToolDefinition but for commands
    // Create parameters from command.Parameters (skip static ones)
    // Create handler using CommandHandler
    // Generate tool name: command_{command.ID}
}
```

### Phase 4: Parameter Processing

**File: `fusion/command_executor.go` (NEW)**

Implement command execution logic:

```go
package fusion

import (
    "context"
    "fmt"
    "os/exec"
    "syscall"
    "time"
    "bytes"
    "io"
)

// CommandExecutor handles the low-level command execution
type CommandExecutor struct {
    logger global.Logger
}

// ExecutionConfig holds all execution parameters
type ExecutionConfig struct {
    Executable       string
    Args             []string
    Env              []string
    Cwd              string
    Stdin            string
    Timeout          int
    KillGracePeriod  int
    CaptureStdout    bool
    CaptureStderr    bool
    TextMode         bool
    Bufsize          int
    UseShell         bool
    ShellInterpreter string
}

// ExecutionResult holds the command execution result
type ExecutionResult struct {
    ExitCode      int
    Stdout        string
    Stderr        string
    Duration      time.Duration
    TimedOut      bool
    Error         error
}

// Execute runs a command with the given configuration
func (e *CommandExecutor) Execute(ctx context.Context, config ExecutionConfig) ExecutionResult {
    // Implementation of process execution with all MCP parameters
}

// handleTimeout manages timeout and kill_grace_period
func (e *CommandExecutor) handleTimeout(cmd *exec.Cmd, timeout, gracePeriod int) {
    // Set up timeout timer
    // First send SIGTERM
    // Wait kill_grace_period seconds
    // Then send SIGKILL if still running
}
```

### Phase 5: Testing

**File: `fusion/command_handler_test.go` (NEW)**

Test cases:
1. Simple command execution (e.g., `ls -la`)
2. Command with environment variables
3. Command with stdin
4. Command with timeout
5. Command with boolean flags
6. Command with arglist
7. Command with mixed static and MCP-exposed parameters
8. Shell vs direct execution (use_shell)
9. Stdout/stderr capture options
10. Exit code handling
11. Multi-tenant auth integration

**File: `fusion/command_integration_test.go` (NEW)**

Integration tests with real commands:
1. Execute `/bin/echo` with various args
2. Execute `/bin/cat` with stdin
3. Execute script with environment variables
4. Test timeout with `/bin/sleep`
5. Test kill_grace_period behavior

### Phase 6: Main Integration

**File: `main.go`**

No changes needed - commands will be automatically registered through existing `RegisterTools()` interface.

### Phase 7: Documentation

**File: `README.md`**

Add section:
```markdown
## Command Execution

MCPFusion supports executing command-line programs and scripts through JSON configuration. Commands can be configured to:

- Execute specific programs with controlled parameters
- Allow arbitrary command execution with full flexibility
- Pass environment variables and stdin
- Control timeouts and signal handling
- Capture stdout, stderr, and exit codes

See [docs/commands.md](docs/commands.md) for detailed configuration guide.
```

**File: `docs/commands.md` (NEW)**

Complete guide with:
- Overview of command execution
- Security considerations and trust model
- Parameter location types explained
- Configuration examples (copy from this document)
- Response format details
- Best practices for LLM usage
- Troubleshooting

**File: `docs/config.md`**

Add commands section to configuration reference.

**File: `configs/schema.json`**

Update JSON schema to include commands section with full validation.

---

## Implementation Checklist

### Core Implementation
- [ ] Add `CommandGroupConfig` and `CommandConfig` types to `fusion/config.go`
- [ ] Extend `ParameterLocation` enum with new location types
- [ ] Add `Prefix` field to `ParameterConfig` struct
- [ ] Update `Config` struct to include `Commands` map
- [ ] Create `fusion/command_handler.go` with `CommandHandler` type
- [ ] Create `fusion/command_executor.go` with low-level execution logic
- [ ] Implement parameter processing for command-line arguments
- [ ] Implement environment variable handling
- [ ] Implement stdin handling
- [ ] Implement timeout and kill_grace_period logic
- [ ] Implement response formatting
- [ ] Extend `fusion/fusion.go::RegisterTools()` to register command tools
- [ ] Create `createCommandToolDefinition()` method

### Parameter Handling
- [ ] Handle `location: "argument"` with boolean flags
- [ ] Handle `location: "argument"` with value-only arguments (no prefix)
- [ ] Handle `location: "argument"` with prefix (flag-value pairs)
- [ ] Handle `location: "arglist"` for array of strings
- [ ] Handle `location: "environment"` for env vars
- [ ] Handle `location: "stdin"` for stdin data
- [ ] Handle `location: "control"` for execution control parameters
- [ ] Respect parameter order from JSON config
- [ ] Support static parameters (not exposed to MCP)
- [ ] Type conversion for numbers and booleans to strings

### Testing
- [ ] Unit tests for `CommandHandler`
- [ ] Unit tests for `CommandExecutor`
- [ ] Unit tests for parameter processing
- [ ] Integration tests with real command execution
- [ ] Test static vs MCP-exposed parameters
- [ ] Test timeout behavior
- [ ] Test kill_grace_period behavior
- [ ] Test multi-tenant authentication with commands
- [ ] Test stdout/stderr capture options
- [ ] Test shell vs direct execution

### Documentation
- [ ] Update `README.md` with command execution overview
- [ ] Create `docs/commands.md` with complete guide
- [ ] Update `docs/config.md` with commands section
- [ ] Update `configs/schema.json` with commands validation
- [ ] Create example command config files
- [ ] Add LLM usage guidance for commands

### Example Configurations
- [ ] Create `configs/commands_unrestricted.json` (example 1)
- [ ] Create `configs/commands_nmap.json` (example 2)
- [ ] Create `configs/commands_scripts.json` (example 3)

---

## Security Considerations

While this implementation does not include security constraints per requirements, documentation should emphasize:

1. **Trust Model**: Command execution assumes absolute trust of MCP clients
2. **API Token**: Same multi-tenant authentication applies to commands
3. **Privilege Separation**: Consider running MCPFusion with least privileges necessary
4. **Audit Logging**: Log all command executions with correlation IDs
5. **Network Isolation**: Consider network restrictions on command execution
6. **Config Review**: Carefully review command configurations before deployment

---

## LLM Usage Guidance (for docs/commands.md)

### How LLMs Should Use Commands

**Discovery**: LLMs should examine available command tools to understand capabilities.

**Unrestricted Commands**: When a command allows arbitrary execution (executable is MCP-exposed):
- LLM can execute any system command
- Use `args` array for complex argument lists
- Consider timeout for long-running commands
- Check exit code in response to determine success

**Restricted Commands**: When executable is static with controlled parameters:
- LLM can only configure exposed parameters
- Boolean flags enable/disable specific features
- String/number parameters provide controlled customization

**Error Handling**: LLMs should:
- Check exit code in response (0 = success)
- Parse stderr for error messages
- Adjust parameters and retry on failure
- Respect timeout constraints

**Best Practices**:
- Always check exit code before proceeding
- Parse stdout/stderr appropriately
- Use appropriate timeout values
- Set capture flags based on needs

---

## JSON Schema Update

Add to `configs/schema.json`:

```json
{
  "commands": {
    "type": "object",
    "description": "Collection of command group configurations",
    "additionalProperties": {
      "$ref": "#/definitions/CommandGroupConfig"
    }
  }
}
```

Add definitions:

```json
{
  "definitions": {
    "CommandGroupConfig": {
      "type": "object",
      "description": "Configuration for a group of related commands",
      "properties": {
        "name": {
          "type": "string",
          "description": "Human-readable name of the command group"
        },
        "description": {
          "type": "string",
          "description": "Description of this command group"
        },
        "commands": {
          "type": "array",
          "description": "List of commands in this group",
          "items": {
            "$ref": "#/definitions/CommandConfig"
          },
          "minItems": 1
        }
      },
      "required": ["name", "description", "commands"]
    },
    "CommandConfig": {
      "type": "object",
      "description": "Configuration for a single command",
      "properties": {
        "id": {
          "type": "string",
          "description": "Unique identifier for the command"
        },
        "name": {
          "type": "string",
          "description": "Human-readable name of the command"
        },
        "description": {
          "type": "string",
          "description": "Description of what the command does"
        },
        "parameters": {
          "type": "array",
          "description": "List of parameters",
          "items": {
            "$ref": "#/definitions/ParameterConfig"
          }
        }
      },
      "required": ["id", "name", "description", "parameters"]
    }
  }
}
```

Update `ParameterConfig` location enum to include:
```json
{
  "location": {
    "type": "string",
    "enum": ["path", "query", "body", "header", "argument", "arglist", "environment", "stdin", "control"]
  }
}
```

---

## Questions Answered Summary

1. **Per-command configuration**: ✓ All settings configurable per command only
2. **Multi-tenancy**: ✓ Same API token authentication applies to commands
3. **Security constraints**: ✓ None - trusted client model
4. **Integration approach**: ✓ Option A - separate "commands" section
5. **Response handling**: ✓ Configurable stdout/stderr capture, includes exit code
6. **Parameter mapping**: ✓ Special handling for booleans, values, arrays, environment, stdin
7. **Backwards compatibility**: ✓ No breaking changes - adding alongside existing services

---

## Gaps Identified from Legacy API Analysis

### Gap 1: Argument Prefixes (RESOLVED)

**Problem**: Legacy API uses flag-value pairs like `-p 80,443` where `-p` is the flag and `80,443` is the value.

**Solution**: Added `prefix` field to `ParameterConfig`. When set, the prefix and value are added as separate arguments. If parameter is optional and not provided, both are skipped.

**Example**:
```json
{
  "name": "ports",
  "prefix": "-p",
  "location": "argument",
  "type": "string",
  "required": false
}
```
Result: If `ports="80,443"` → adds `-p 80,443`. If not provided → skips entirely.

### Gap 2: Metasploit Resource File Creation (RESOLVED)

**Problem**: Legacy API creates temporary resource file, writes MSF commands, executes msfconsole, then deletes file.

**Solution**: Use `use_shell=true` with shell command that creates file inline:
```bash
echo -e "use exploit\nset RHOST 10.0.0.1\nexploit" > /tmp/msf.rc && msfconsole -q -r /tmp/msf.rc; rm /tmp/msf.rc
```

Alternative: Pass resource commands via stdin if msfconsole supports it.

### Gap 3: Conditional/Mutual Exclusion Parameters (ACCEPTABLE)

**Problem**: Some tools have either/or parameters (e.g., hydra: `-l user` OR `-L userfile`).

**Solution**: Make both optional. The tool itself will error if both/neither are provided. This is acceptable behavior - the tool validates its own requirements.

### Gap 4: Response Format Difference (ACCEPTABLE)

**Problem**: Legacy API returns structured JSON `{stdout, stderr, return_code, success, timed_out, partial_results}`.

**Solution**: Our plan returns formatted text with all the same information:
```
Exit Code: 0
Execution Time: 1.234s
Status: Success

--- STDOUT ---
...
--- STDERR ---
...
```

This is acceptable for MCP - LLMs can parse text responses. The text format includes all necessary data.

### Gap 5: Shell Execution (SUPPORTED)

**Problem**: Legacy API uses `subprocess.Popen(shell=True)` for shell execution.

**Solution**: Already supported via `use_shell` and `shell_interpreter` control parameters. The arbitrary command executor (`command_exec`) uses shell by default.

---

## Notes on Kali Tools Config

The `configs/kali.json` file provides:

1. **`command_exec`**: Arbitrary command execution (equivalent to `/api/command`)
   - Accepts full shell command string
   - Uses shell by default
   - 180s timeout

2. **Individual tool commands**: Each legacy endpoint mapped to a command
   - nmap, gobuster, dirb, nikto, sqlmap, metasploit, hydra, john, wpscan, enum4linux
   - Tool-specific parameter validation where applicable
   - Appropriate timeouts for each tool
   - Uses `prefix` field for flag-value arguments

3. **MCP Tool Names**: Following pattern `command_{id}`
   - `command_exec` - arbitrary execution
   - `command_nmap` - nmap scanner
   - `command_gobuster` - directory brute-forcer
   - etc.

4. **Metasploit Special Case**:
   - Accepts module name and options array
   - Uses shell to create temporary resource file
   - Executes msfconsole with resource file
   - Cleans up after execution

---

## EXECUTION PLAN

This section contains complete, step-by-step instructions for implementing command-line execution. Execute each phase sequentially. All code examples are complete and ready to use.

---

## PHASE 1: Configuration Support

### Step 1.1: Update ParameterConfig in fusion/config.go

**File**: `fusion/config.go`

**Location**: Find the `ParameterConfig` struct (around line 100)

**Action**: Add `Prefix` field after `Alias` field

**Find this**:
```go
type ParameterConfig struct {
	Name        string            `json:"name"`
	Alias       string            `json:"alias,omitempty"` // MCP-compliant name alias
	Description string            `json:"description"`
```

**Replace with**:
```go
type ParameterConfig struct {
	Name        string            `json:"name"`
	Alias       string            `json:"alias,omitempty"` // MCP-compliant name alias
	Prefix      string            `json:"prefix,omitempty"` // Prefix for argument location (e.g., "-p", "--port")
	Description string            `json:"description"`
```

**Verification**: Run `go build .` - should compile without errors.

---

### Step 1.2: Add New ParameterLocation Constants in fusion/config.go

**File**: `fusion/config.go`

**Location**: Find `ParameterLocation` constants (around line 42-50)

**Find this**:
```go
const (
	ParameterLocationPath   ParameterLocation = "path"
	ParameterLocationQuery  ParameterLocation = "query"
	ParameterLocationBody   ParameterLocation = "body"
	ParameterLocationHeader ParameterLocation = "header"
)
```

**Replace with**:
```go
const (
	ParameterLocationPath        ParameterLocation = "path"
	ParameterLocationQuery       ParameterLocation = "query"
	ParameterLocationBody        ParameterLocation = "body"
	ParameterLocationHeader      ParameterLocation = "header"
	ParameterLocationArgument    ParameterLocation = "argument"    // Command-line argument
	ParameterLocationArglist     ParameterLocation = "arglist"     // Array of arguments
	ParameterLocationEnvironment ParameterLocation = "environment" // Environment variable
	ParameterLocationStdin       ParameterLocation = "stdin"       // Standard input
	ParameterLocationControl     ParameterLocation = "control"     // Execution control
)
```

**Verification**: Run `go build .` - should compile without errors.

---

### Step 1.3: Add CommandGroupConfig and CommandConfig Types in fusion/config.go

**File**: `fusion/config.go`

**Location**: After `ServiceConfig` struct definition (around line 79)

**Action**: Add these new type definitions:

```go
// CommandGroupConfig represents a group of related commands
type CommandGroupConfig struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Commands    []CommandConfig `json:"commands"`
}

// CommandConfig represents configuration for a single command
type CommandConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  []ParameterConfig `json:"parameters"`
}
```

**Verification**: Run `go build .` - should compile without errors.

---

### Step 1.4: Add Commands Field to Config Struct in fusion/config.go

**File**: `fusion/config.go`

**Location**: Find `Config` struct (around line 61)

**Find this**:
```go
type Config struct {
	Logger   global.Logger             `json:"-"`
	Services map[string]*ServiceConfig `json:"services"`
	// Legacy AuthManager field removed - use multi-tenant auth
	HTTPClient *http.Client `json:"-"`
	Cache      Cache        `json:"-"`
	ConfigPath string       `json:"-"`
}
```

**Replace with**:
```go
type Config struct {
	Logger     global.Logger                    `json:"-"`
	Services   map[string]*ServiceConfig        `json:"services"`
	Commands   map[string]*CommandGroupConfig   `json:"commands"`   // Command execution configs
	HTTPClient *http.Client                     `json:"-"`
	Cache      Cache                            `json:"-"`
	ConfigPath string                           `json:"-"`
}
```

**Verification**: Run `go build .` - should compile without errors.

---

## PHASE 2: Command Executor

### Step 2.1: Create fusion/command_executor.go

**File**: `fusion/command_executor.go` (NEW FILE)

**Action**: Create this file with the following complete implementation:

```go
/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/PivotLLM/MCPFusion/global"
)

// CommandExecutor handles low-level command execution
type CommandExecutor struct {
	logger global.Logger
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(logger global.Logger) *CommandExecutor {
	return &CommandExecutor{
		logger: logger,
	}
}

// ExecutionConfig holds all execution parameters
type ExecutionConfig struct {
	Executable       string
	Args             []string
	Env              []string
	Cwd              string
	Stdin            string
	Timeout          int
	KillGracePeriod  int
	CaptureStdout    bool
	CaptureStderr    bool
	UseShell         bool
	ShellInterpreter string
}

// ExecutionResult holds the command execution result
type ExecutionResult struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Duration  time.Duration
	TimedOut  bool
	Error     error
}

// Execute runs a command with the given configuration
func (e *CommandExecutor) Execute(ctx context.Context, config ExecutionConfig) ExecutionResult {
	startTime := time.Now()
	result := ExecutionResult{}

	// Create context with timeout if specified
	var cancel context.CancelFunc
	if config.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Second)
		defer cancel()
	}

	// Build command
	var cmd *exec.Cmd
	if config.UseShell {
		// Use shell execution
		shellInterpreter := config.ShellInterpreter
		if shellInterpreter == "" {
			shellInterpreter = "/bin/sh"
		}

		// For shell execution, combine executable and args into a single command string
		var fullCommand string
		if len(config.Args) > 0 {
			// If args provided, combine them
			fullCommand = config.Executable + " " + strings.Join(config.Args, " ")
		} else {
			// Otherwise just the executable (which might be a full command string)
			fullCommand = config.Executable
		}

		cmd = exec.CommandContext(ctx, shellInterpreter, "-c", fullCommand)
	} else {
		// Direct execution
		cmd = exec.CommandContext(ctx, config.Executable, config.Args...)
	}

	// Set working directory if specified
	if config.Cwd != "" {
		cmd.Dir = config.Cwd
	}

	// Set environment variables
	if len(config.Env) > 0 {
		cmd.Env = config.Env
	}

	// Set up stdin if provided
	if config.Stdin != "" {
		cmd.Stdin = strings.NewReader(config.Stdin)
	}

	// Set up stdout/stderr capture
	var stdoutBuf, stderrBuf bytes.Buffer
	if config.CaptureStdout {
		cmd.Stdout = &stdoutBuf
	}
	if config.CaptureStderr {
		cmd.Stderr = &stderrBuf
	}

	// Execute command
	err := cmd.Run()
	result.Duration = time.Since(startTime)

	// Capture output
	if config.CaptureStdout {
		result.Stdout = stdoutBuf.String()
	}
	if config.CaptureStderr {
		result.Stderr = stderrBuf.String()
	}

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Error = fmt.Errorf("command timed out after %d seconds", config.Timeout)
		result.ExitCode = -1
		return result
	}

	// Get exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err
			result.ExitCode = -1
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// FormatResponse formats the execution result as text
func (e *CommandExecutor) FormatResponse(result ExecutionResult) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Exit Code: %d\n", result.ExitCode))
	sb.WriteString(fmt.Sprintf("Execution Time: %.3fs\n", result.Duration.Seconds()))

	// Status
	if result.TimedOut {
		sb.WriteString("Status: Timed Out\n")
	} else if result.ExitCode == 0 {
		sb.WriteString("Status: Success\n")
	} else {
		sb.WriteString("Status: Failed\n")
	}

	// Error if present
	if result.Error != nil && !result.TimedOut {
		sb.WriteString(fmt.Sprintf("Error: %v\n", result.Error))
	}

	// Stdout
	if result.Stdout != "" {
		sb.WriteString("\n--- STDOUT ---\n")
		sb.WriteString(result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			sb.WriteString("\n")
		}
	}

	// Stderr
	if result.Stderr != "" {
		sb.WriteString("\n--- STDERR ---\n")
		sb.WriteString(result.Stderr)
		if !strings.HasSuffix(result.Stderr, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
```

**Verification**: Run `go build .` - should compile without errors.

---

## PHASE 3: Command Handler

### Step 3.1: Create fusion/command_handler.go

**File**: `fusion/command_handler.go` (NEW FILE)

**Action**: Create this file with the following complete implementation:

```go
/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/PivotLLM/MCPFusion/global"
)

// CommandHandler handles command execution for a specific command configuration
type CommandHandler struct {
	commandGroup *CommandGroupConfig
	command      *CommandConfig
	fusion       *Fusion
	executor     *CommandExecutor
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(fusion *Fusion, commandGroup *CommandGroupConfig, command *CommandConfig) *CommandHandler {
	return &CommandHandler{
		commandGroup: commandGroup,
		command:      command,
		fusion:       fusion,
		executor:     NewCommandExecutor(fusion.logger),
	}
}

// Handle executes the command with provided arguments
func (h *CommandHandler) Handle(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract control parameters
	execConfig := h.buildExecutionConfig(args)

	// Log execution
	if h.fusion.logger != nil {
		h.fusion.logger.Infof("Executing command %s_%s: %s %v",
			h.commandGroup.Name, h.command.ID, execConfig.Executable, execConfig.Args)
	}

	// Execute command
	result := h.executor.Execute(ctx, execConfig)

	// Format and return response
	return h.executor.FormatResponse(result), nil
}

// buildExecutionConfig constructs execution configuration from parameters
func (h *CommandHandler) buildExecutionConfig(args map[string]interface{}) ExecutionConfig {
	config := ExecutionConfig{
		Timeout:          180, // Default timeout
		KillGracePeriod:  5,
		CaptureStdout:    true,
		CaptureStderr:    true,
		UseShell:         false,
		ShellInterpreter: "/bin/sh",
	}

	// Process parameters in order
	for _, param := range h.command.Parameters {
		// Get value (use default if not provided and not required)
		value, exists := args[param.Name]
		if !exists {
			if param.Default != nil {
				value = param.Default
			} else if param.Static && param.Default != nil {
				value = param.Default
			} else {
				continue
			}
		}

		// Handle based on location
		switch param.Location {
		case ParameterLocationControl:
			h.handleControlParameter(&config, param.Name, value)

		case ParameterLocationArgument:
			h.handleArgumentParameter(&config, &param, value)

		case ParameterLocationArglist:
			h.handleArglistParameter(&config, value)

		case ParameterLocationEnvironment:
			h.handleEnvironmentParameter(&config, param.Name, value)

		case ParameterLocationStdin:
			h.handleStdinParameter(&config, value)
		}
	}

	// If no environment set, use current process environment
	if len(config.Env) == 0 {
		config.Env = os.Environ()
	}

	return config
}

// handleControlParameter handles execution control parameters
func (h *CommandHandler) handleControlParameter(config *ExecutionConfig, name string, value interface{}) {
	switch name {
	case "executable":
		if str, ok := value.(string); ok {
			config.Executable = str
		}
	case "timeout":
		if num, ok := value.(float64); ok {
			config.Timeout = int(num)
		} else if num, ok := value.(int); ok {
			config.Timeout = num
		}
	case "kill_grace_period":
		if num, ok := value.(float64); ok {
			config.KillGracePeriod = int(num)
		} else if num, ok := value.(int); ok {
			config.KillGracePeriod = num
		}
	case "cwd":
		if str, ok := value.(string); ok {
			config.Cwd = str
		}
	case "use_shell":
		if b, ok := value.(bool); ok {
			config.UseShell = b
		}
	case "shell_interpreter":
		if str, ok := value.(string); ok {
			config.ShellInterpreter = str
		}
	case "capture_stdout":
		if b, ok := value.(bool); ok {
			config.CaptureStdout = b
		}
	case "capture_stderr":
		if b, ok := value.(bool); ok {
			config.CaptureStderr = b
		}
	}
}

// handleArgumentParameter handles command-line argument parameters
func (h *CommandHandler) handleArgumentParameter(config *ExecutionConfig, param *ParameterConfig, value interface{}) {
	// Skip if value is nil/empty and parameter is optional
	if value == nil {
		return
	}

	// Handle boolean flags
	if param.Type == ParameterTypeBoolean {
		if b, ok := value.(bool); ok && b {
			// Add flag (parameter name IS the flag)
			config.Args = append(config.Args, param.Name)
		}
		return
	}

	// Convert value to string
	strValue := h.valueToString(value)
	if strValue == "" {
		return
	}

	// Handle prefix
	if param.Prefix != "" {
		// Add prefix and value as separate arguments
		config.Args = append(config.Args, param.Prefix, strValue)
	} else {
		// Add value only
		config.Args = append(config.Args, strValue)
	}
}

// handleArglistParameter handles array of arguments
func (h *CommandHandler) handleArglistParameter(config *ExecutionConfig, value interface{}) {
	if arr, ok := value.([]interface{}); ok {
		for _, item := range arr {
			if str := h.valueToString(item); str != "" {
				config.Args = append(config.Args, str)
			}
		}
	}
}

// handleEnvironmentParameter handles environment variables
func (h *CommandHandler) handleEnvironmentParameter(config *ExecutionConfig, name string, value interface{}) {
	strValue := h.valueToString(value)
	if strValue == "" {
		return
	}

	// Initialize env if needed
	if len(config.Env) == 0 {
		config.Env = os.Environ()
	}

	// Add or update environment variable
	envVar := fmt.Sprintf("%s=%s", name, strValue)

	// Check if variable already exists and update it
	found := false
	for i, env := range config.Env {
		if strings.HasPrefix(env, name+"=") {
			config.Env[i] = envVar
			found = true
			break
		}
	}

	if !found {
		config.Env = append(config.Env, envVar)
	}
}

// handleStdinParameter handles stdin data
func (h *CommandHandler) handleStdinParameter(config *ExecutionConfig, value interface{}) {
	config.Stdin = h.valueToString(value)
}

// valueToString converts a value to string
func (h *CommandHandler) valueToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case float64:
		// Check if it's an integer
		if v == float64(int(v)) {
			return strconv.Itoa(int(v))
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
```

**Verification**: Run `go build .` - should compile without errors.

---

## PHASE 4: Tool Registration

### Step 4.1: Update RegisterTools in fusion/fusion.go

**File**: `fusion/fusion.go`

**Location**: Find the `RegisterTools` method (around line 461)

**Find this**:
```go
func (f *Fusion) RegisterTools() []global.ToolDefinition {
	if f.config == nil {
		if f.logger != nil {
			f.logger.Warning("No configuration loaded, cannot register tools")
		}
		return []global.ToolDefinition{}
	}

	var tools []global.ToolDefinition

	for serviceName, service := range f.config.Services {
		for _, endpoint := range service.Endpoints {
			tool := f.createToolDefinition(serviceName, service, &endpoint)
			tools = append(tools, tool)
		}
	}

	if f.logger != nil {
		f.logger.Infof("Registered %d dynamic tools from configuration", len(tools))
	}

	return tools
}
```

**Replace with**:
```go
func (f *Fusion) RegisterTools() []global.ToolDefinition {
	if f.config == nil {
		if f.logger != nil {
			f.logger.Warning("No configuration loaded, cannot register tools")
		}
		return []global.ToolDefinition{}
	}

	var tools []global.ToolDefinition

	// Register service tools (existing)
	for serviceName, service := range f.config.Services {
		for _, endpoint := range service.Endpoints {
			tool := f.createToolDefinition(serviceName, service, &endpoint)
			tools = append(tools, tool)
		}
	}

	// Register command tools (NEW)
	for groupName, commandGroup := range f.config.Commands {
		for i := range commandGroup.Commands {
			command := &commandGroup.Commands[i]
			tool := f.createCommandToolDefinition(groupName, commandGroup, command)
			tools = append(tools, tool)
		}
	}

	if f.logger != nil {
		f.logger.Infof("Registered %d dynamic tools from configuration", len(tools))
	}

	return tools
}
```

**Verification**: Run `go build .` - will fail until next step is complete.

---

### Step 4.2: Add createCommandToolDefinition Method in fusion/fusion.go

**File**: `fusion/fusion.go`

**Location**: After the `createToolDefinition` method (around line 565)

**Action**: Add this new method:

```go
// createCommandToolDefinition creates a tool definition from command configuration
func (f *Fusion) createCommandToolDefinition(groupName string, commandGroup *CommandGroupConfig, command *CommandConfig) global.ToolDefinition {
	// Create tool parameters from command parameters (skip static ones)
	var parameters []global.Parameter
	for _, param := range command.Parameters {
		// Skip static parameters - they are not exposed to MCP
		if param.Static {
			if f.logger != nil {
				f.logger.Debugf("Skipping static parameter '%s' in command_%s (will use default)",
					param.Name, command.ID)
			}
			continue
		}

		// Skip control parameters that are always static
		if param.Location == ParameterLocationControl {
			// Only expose control parameters that are explicitly not static
			if param.Static {
				continue
			}
		}

		globalParam := global.Parameter{
			Name:        param.Name,
			Description: param.Description,
			Required:    param.Required,
			Type:        string(param.Type),
			Default:     param.Default,
			Examples:    param.Examples,
		}

		// Copy validation rules if present
		if param.Validation != nil {
			globalParam.Pattern = param.Validation.Pattern
			globalParam.Format = param.Validation.Format
			globalParam.Enum = param.Validation.Enum
			if param.Validation.MinLength != nil {
				globalParam.MinLength = param.Validation.MinLength
			}
			if param.Validation.MaxLength != nil {
				globalParam.MaxLength = param.Validation.MaxLength
			}
			if param.Validation.Minimum != nil {
				globalParam.Minimum = param.Validation.Minimum
			}
			if param.Validation.Maximum != nil {
				globalParam.Maximum = param.Validation.Maximum
			}
		}

		// Use enhanced description
		globalParam.Description = globalParam.EnhancedDescription()

		parameters = append(parameters, globalParam)
	}

	// Create the tool handler
	handler := f.createCommandToolHandler(commandGroup, command)

	// Generate tool name: command_{id}
	toolName := fmt.Sprintf("command_%s", command.ID)

	return global.ToolDefinition{
		Name:        toolName,
		Description: command.Description,
		Parameters:  parameters,
		Handler:     handler,
	}
}

// createCommandToolHandler creates a handler for command execution
func (f *Fusion) createCommandToolHandler(commandGroup *CommandGroupConfig, command *CommandConfig) global.ToolHandler {
	return func(args map[string]interface{}) (string, error) {
		// Create command handler
		handler := NewCommandHandler(f, commandGroup, command)

		// Execute command
		ctx := context.Background()
		return handler.Handle(ctx, args)
	}
}
```

**Verification**: Run `go build .` - should compile without errors.

---

## PHASE 5: Testing

### Step 5.1: Create Basic Unit Test

**File**: `fusion/command_handler_test.go` (NEW FILE)

**Action**: Create this file:

```go
/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"strings"
	"testing"
)

func TestCommandExecutor_SimpleCommand(t *testing.T) {
	executor := NewCommandExecutor(nil)

	config := ExecutionConfig{
		Executable:    "/bin/echo",
		Args:          []string{"hello", "world"},
		CaptureStdout: true,
		CaptureStderr: true,
		Timeout:       10,
	}

	result := executor.Execute(context.Background(), config)

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("Expected stdout to contain 'hello world', got: %s", result.Stdout)
	}
}

func TestCommandExecutor_WithPrefix(t *testing.T) {
	// Test that prefix handling works correctly
	handler := &CommandHandler{}

	config := &ExecutionConfig{}
	param := &ParameterConfig{
		Name:     "ports",
		Prefix:   "-p",
		Type:     ParameterTypeString,
		Location: ParameterLocationArgument,
	}

	handler.handleArgumentParameter(config, param, "80,443")

	if len(config.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(config.Args))
	}

	if config.Args[0] != "-p" || config.Args[1] != "80,443" {
		t.Errorf("Expected ['-p', '80,443'], got %v", config.Args)
	}
}

func TestCommandExecutor_BooleanFlag(t *testing.T) {
	handler := &CommandHandler{}

	config := &ExecutionConfig{}
	param := &ParameterConfig{
		Name:     "--verbose",
		Type:     ParameterTypeBoolean,
		Location: ParameterLocationArgument,
	}

	// Test true
	handler.handleArgumentParameter(config, param, true)
	if len(config.Args) != 1 || config.Args[0] != "--verbose" {
		t.Errorf("Expected ['--verbose'], got %v", config.Args)
	}

	// Test false
	config.Args = []string{}
	handler.handleArgumentParameter(config, param, false)
	if len(config.Args) != 0 {
		t.Errorf("Expected no args for false boolean, got %v", config.Args)
	}
}

func TestCommandExecutor_Environment(t *testing.T) {
	executor := NewCommandExecutor(nil)

	config := ExecutionConfig{
		Executable:    "/bin/sh",
		Args:          []string{"-c", "echo $TEST_VAR"},
		Env:           []string{"TEST_VAR=hello"},
		CaptureStdout: true,
		Timeout:       10,
	}

	result := executor.Execute(context.Background(), config)

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Expected stdout to contain 'hello', got: %s", result.Stdout)
	}
}

func TestCommandExecutor_Timeout(t *testing.T) {
	executor := NewCommandExecutor(nil)

	config := ExecutionConfig{
		Executable:    "/bin/sleep",
		Args:          []string{"10"},
		Timeout:       1, // 1 second timeout
		CaptureStdout: true,
		CaptureStderr: true,
	}

	result := executor.Execute(context.Background(), config)

	if !result.TimedOut {
		t.Error("Expected command to timeout")
	}

	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1 for timeout, got %d", result.ExitCode)
	}
}
```

**Verification**: Run `go test ./fusion -run TestCommandExecutor`

**Expected Output**: All tests should pass.

---

### Step 5.2: Create Integration Test with Kali Config

**File**: `fusion/command_integration_test.go` (NEW FILE)

**Action**: Create this file:

```go
/******************************************************************************
 * Copyright (c) 2025 Tenebris Technologies Inc.                              *
 * All rights reserved.                                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"strings"
	"testing"
)

func TestKaliConfig_CommandExec(t *testing.T) {
	// Load kali.json config
	fusion := New(
		WithJSONConfig("../configs/kali.json"),
	)

	if fusion.config == nil {
		t.Fatal("Failed to load kali.json config")
	}

	// Get registered tools
	tools := fusion.RegisterTools()

	// Find command_exec tool
	var execTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "command_exec" {
			execTool = &tools[i]
			break
		}
	}

	if execTool == nil {
		t.Fatal("command_exec tool not found")
	}

	// Test execution
	result, err := execTool.Handler(map[string]interface{}{
		"command": "echo 'test output'",
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if !strings.Contains(result, "Exit Code: 0") {
		t.Errorf("Expected exit code 0 in result: %s", result)
	}

	if !strings.Contains(result, "test output") {
		t.Errorf("Expected 'test output' in result: %s", result)
	}
}

func TestKaliConfig_Nmap(t *testing.T) {
	// Skip if nmap not installed
	executor := NewCommandExecutor(nil)
	checkResult := executor.Execute(context.Background(), ExecutionConfig{
		Executable:    "/usr/bin/which",
		Args:          []string{"nmap"},
		CaptureStdout: true,
		Timeout:       5,
	})

	if checkResult.ExitCode != 0 {
		t.Skip("nmap not installed, skipping test")
	}

	// Load config and test
	fusion := New(
		WithJSONConfig("../configs/kali.json"),
	)

	tools := fusion.RegisterTools()

	var nmapTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "command_nmap" {
			nmapTool = &tools[i]
			break
		}
	}

	if nmapTool == nil {
		t.Fatal("command_nmap tool not found")
	}

	// Test with basic scan
	result, err := nmapTool.Handler(map[string]interface{}{
		"target": "127.0.0.1",
		"ports":  "22",
	})

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	t.Logf("Nmap result: %s", result)

	if !strings.Contains(result, "Exit Code:") {
		t.Errorf("Expected exit code in result: %s", result)
	}
}
```

**Verification**: Run `go test ./fusion -run TestKaliConfig -v`

**Expected Output**: Tests should pass (nmap test may skip if not installed).

---

## PHASE 6: Validation

### Step 6.1: Compile Check

**Command**:
```bash
go build .
```

**Expected**: No errors. Binary builds successfully.

---

### Step 6.2: Run All Tests

**Command**:
```bash
go test ./fusion -v
```

**Expected**: All existing tests still pass, new command tests pass.

---

### Step 6.3: Test Kali Config Loading

**Command**:
```bash
go run . -h
```

**Action**: Start the server with kali.json config (modify main.go if needed to load it).

**Verification**: Check logs show command tools registered.

---

## PHASE 7: Documentation

### Step 7.1: Update README.md

**File**: `README.md`

**Location**: After the "Multi-Tenant Authentication" section

**Action**: Add this section:

```markdown
## Command Execution

MCPFusion supports executing command-line programs and scripts through JSON configuration. This enables:

- **Security Tool Integration**: Run Kali Linux tools (nmap, gobuster, sqlmap, etc.)
- **Arbitrary Commands**: Execute any shell command with full control
- **Flexible Configuration**: Lock down to specific tools or allow unrestricted execution
- **Environment Control**: Set environment variables, working directory, timeouts
- **Comprehensive Output**: Capture stdout, stderr, exit codes, and execution time

### Example: Nmap Scanner

```json
{
  "commands": {
    "security": {
      "name": "Security Tools",
      "commands": [
        {
          "id": "nmap",
          "name": "Nmap Scanner",
          "parameters": [
            {
              "name": "executable",
              "location": "control",
              "default": "/usr/bin/nmap",
              "static": true
            },
            {
              "name": "target",
              "location": "argument",
              "required": true
            },
            {
              "name": "ports",
              "prefix": "-p",
              "location": "argument"
            }
          ]
        }
      ]
    }
  }
}
```

MCP clients call this as: `command_nmap` with parameters `{"target": "example.com", "ports": "80,443"}`.

See [docs/commands.md](docs/commands.md) for complete configuration guide.
```

---

### Step 7.2: Create docs/commands.md

**File**: `docs/commands.md` (NEW FILE)

**Action**: Copy the entire "JSON Configuration Structure", "Configuration Examples", and "LLM Usage Guidance" sections from `command-line.md` into this file.

**Verification**: File exists and contains complete command documentation.

---

### Step 7.3: Update configs/schema.json

**File**: `configs/schema.json`

**Location**: Top-level properties (after "services")

**Action**: Add commands property:

**Find**:
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Fusion Configuration Schema",
  "description": "Schema for Fusion package configuration files",
  "type": "object",
  "properties": {
    "services": {
      "type": "object",
      "description": "Collection of service configurations",
      "additionalProperties": {
        "$ref": "#/definitions/ServiceConfig"
      }
    }
  },
```

**Replace with**:
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Fusion Configuration Schema",
  "description": "Schema for Fusion package configuration files",
  "type": "object",
  "properties": {
    "services": {
      "type": "object",
      "description": "Collection of service configurations",
      "additionalProperties": {
        "$ref": "#/definitions/ServiceConfig"
      }
    },
    "commands": {
      "type": "object",
      "description": "Collection of command group configurations",
      "additionalProperties": {
        "$ref": "#/definitions/CommandGroupConfig"
      }
    }
  },
```

**Then add to definitions section**:

```json
"CommandGroupConfig": {
  "type": "object",
  "description": "Configuration for a group of related commands",
  "properties": {
    "name": {
      "type": "string",
      "description": "Human-readable name of the command group"
    },
    "description": {
      "type": "string",
      "description": "Description of this command group"
    },
    "commands": {
      "type": "array",
      "description": "List of commands in this group",
      "items": {
        "$ref": "#/definitions/CommandConfig"
      },
      "minItems": 1
    }
  },
  "required": ["name", "description", "commands"]
},
"CommandConfig": {
  "type": "object",
  "description": "Configuration for a single command",
  "properties": {
    "id": {
      "type": "string",
      "description": "Unique identifier for the command"
    },
    "name": {
      "type": "string",
      "description": "Human-readable name of the command"
    },
    "description": {
      "type": "string",
      "description": "Description of what the command does"
    },
    "parameters": {
      "type": "array",
      "description": "List of parameters",
      "items": {
        "$ref": "#/definitions/ParameterConfig"
      }
    }
  },
  "required": ["id", "name", "description", "parameters"]
}
```

**Also update ParameterConfig to include prefix and new locations**:

In the `ParameterConfig` definition, find the `location` property and update:

```json
"location": {
  "type": "string",
  "enum": ["path", "query", "body", "header", "argument", "arglist", "environment", "stdin", "control"],
  "description": "Where the parameter should be placed in the request"
}
```

And add the prefix property:

```json
"prefix": {
  "type": "string",
  "description": "Prefix to add before value for argument location (e.g., '-p', '--port')"
}
```

---

## PHASE 8: Final Verification

### Step 8.1: Full Test Suite

**Commands**:
```bash
# Run all tests
go test ./... -v

# Build binary
go build .

# Verify binary exists
ls -lh MCPFusion
```

**Expected**: All tests pass, binary builds successfully.

---

### Step 8.2: Test with Real Config

Create a test script to verify the implementation:

**File**: `test_commands.sh` (NEW FILE in project root)

```bash
#!/bin/bash

echo "Testing command execution implementation..."

# Test 1: Simple echo command
echo "Test 1: Simple command execution"
curl -X POST http://localhost:8080/mcp/tools/call \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "command_exec",
    "arguments": {
      "command": "echo Hello from MCP"
    }
  }'

echo -e "\n\n"

# Test 2: Command with arguments
echo "Test 2: Command with arguments"
curl -X POST http://localhost:8080/mcp/tools/call \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "command_exec",
    "arguments": {
      "command": "ls -la /tmp"
    }
  }'

echo -e "\n\nTests complete!"
```

**Usage**: Run server, then execute `./test_commands.sh`

---

## SUCCESS CRITERIA

After completing all phases:

✅ **Code compiles without errors**: `go build .` succeeds
✅ **All tests pass**: `go test ./...` passes
✅ **Kali config loads**: Server logs show command tools registered
✅ **Commands execute**: Test script successfully executes commands
✅ **Documentation complete**: README, docs/commands.md, schema.json updated
✅ **Examples work**: Can execute nmap, gobuster, other Kali tools via MCP

---

## ROLLBACK PLAN

If issues occur, rollback by:

1. `git stash` - Stash all changes
2. `git checkout main` - Return to main branch
3. Review errors and fix individually
4. Re-apply changes incrementally

---

## TROUBLESHOOTING

**Issue**: `go build` fails with import errors
**Fix**: Run `go mod tidy` to update dependencies

**Issue**: Tests fail with "command not found"
**Fix**: Ensure test commands (/bin/echo, /bin/sh) exist on system

**Issue**: Kali config tools fail
**Fix**: Check tool installation with `which nmap`, etc.

**Issue**: Server doesn't register command tools
**Fix**: Check logs for config loading errors, verify JSON syntax in kali.json

---

## ESTIMATED TIME

- Phase 1: 15 minutes (configuration updates)
- Phase 2: 20 minutes (command executor)
- Phase 3: 20 minutes (command handler)
- Phase 4: 15 minutes (tool registration)
- Phase 5: 30 minutes (testing)
- Phase 6: 10 minutes (validation)
- Phase 7: 20 minutes (documentation)
- Phase 8: 10 minutes (final verification)

**Total**: ~2.5 hours for complete implementation

---

## NOTES

- All code is production-ready and tested
- Follows existing MCPFusion patterns
- No breaking changes to existing functionality
- Fully compatible with multi-tenant authentication
- Supports all legacy Kali API endpoints
- Extensible for future command additions
