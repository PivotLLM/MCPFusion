# MCPFusion Tests

This directory contains functional test scripts for all MCPFusion MCP tools. Tests use [MCPProbe](https://github.com/PivotLLM/MCPProbe) to invoke tools over a live MCP connection.

## Prerequisites

- MCPFusion server running (default: `http://localhost:8888`)
- `probe` binary available in `$PATH` or at the path configured in each test's `.env`
- A valid MCPFusion API token

## Authentication Setup

All tests load credentials from a `.env` file in their respective directory.

```bash
# Create the root-level .env for core tests
cat > tests/.env <<EOF
APIKEY=your-token-here
SERVER_URL=http://localhost:8888
EOF

# Create .env files for subdirectory test suites
cp tests/.env tests/M365/.env
cp tests/.env tests/Google/.env
cp tests/.env tests/PwnDoc/.env
```

**Security Note**: `.env` files contain sensitive tokens. They are gitignored and must never be committed.

## Test Structure

```
tests/
├── run_all_tests.sh          # Runs all core tests
├── test_health.sh            # health_status tool
├── test_knowledge.sh         # knowledge_set / get / delete / search / rename
├── test_knowledge_key.sh     # knowledge_get with key-based retrieval
├── test_perf.sh              # perf_echo / delay / random_data / error / counter
├── test_stdio.sh             # Stdio transport connectivity
├── test_stress.sh            # Concurrent load / stress testing
├── M365/
│   ├── run_all_tests.sh      # Runs all M365 tests
│   ├── test_profile.sh
│   ├── test_calendar_summary.sh
│   ├── test_calendar_details.sh
│   ├── test_calendar_search.sh
│   ├── test_calendars_list.sh
│   ├── test_calendar_events.sh
│   ├── test_mail.sh
│   ├── test_mail_search.sh
│   ├── test_mail_folders.sh
│   ├── test_mail_folder_messages.sh
│   ├── test_mail_draft_create.sh
│   ├── test_mail_draft_list.sh
│   ├── test_mail_draft_get.sh (via test_individual_items.sh)
│   ├── test_mail_draft_update.sh
│   ├── test_mail_draft_delete.sh
│   ├── test_mail_draft_reply.sh
│   ├── test_mail_draft_reply_all.sh
│   ├── test_mail_draft_forward.sh
│   ├── test_files_search.sh
│   ├── test_files_enhanced.sh
│   ├── test_files_content_download.sh
│   ├── test_files_folder_navigation.sh
│   ├── test_files_path_access.sh
│   ├── test_files_id_navigation.sh
│   ├── test_files_recent.sh
│   ├── test_contacts.sh
│   ├── test_individual_items.sh
│   └── test_object_array_params.sh
├── Google/
│   ├── run_all_tests.sh      # Runs all Google tests
│   ├── test_profile.sh
│   ├── test_contacts_list.sh
│   ├── test_contacts_get.sh
│   ├── test_contacts_search.sh
│   ├── test_gmail_draft_create.sh
│   ├── test_gmail_draft_list.sh
│   ├── test_gmail_draft_get.sh
│   ├── test_gmail_draft_update.sh
│   └── test_gmail_draft_delete.sh
└── PwnDoc/
    ├── run_all_tests.sh      # Runs all PwnDoc tests
    ├── test_audits.sh
    ├── test_findings.sh
    ├── test_clients.sh
    └── test_object_array_params.sh
└── UnifyEM/
    ├── run_all_tests.sh      # Runs all UnifyEM tests
    ├── test_ping.sh          # unifyem_ping, unifyem_auth_setup
    └── test_agents.sh        # unifyem_agent_list, unifyem_agent_list_by_tag, unifyem_request_list
```

## Running Tests

### Core tests

```bash
cd tests
./run_all_tests.sh
```

### Individual core tests

```bash
cd tests
./test_health.sh
./test_knowledge.sh
./test_perf.sh
./test_stress.sh
```

### Microsoft 365 tests

Requires valid M365 OAuth authentication (run `microsoft365_auth_setup` or equivalent first).

```bash
cd tests/M365
./run_all_tests.sh

# Or individually:
./test_profile.sh
./test_mail.sh
```

### Google tests

Requires valid Google OAuth authentication (run `google_auth_setup` or equivalent first).

```bash
cd tests/Google
./run_all_tests.sh
```

### UnifyEM tests

Requires a running UnifyEM server (`UNIFYEM_URL` set for MCPFusion) and per-user credentials stored for the API token: call `unifyem_auth_setup` and run the `fusion-auth` command it returns, entering your UnifyEM administrator username and password. Set `UEM_TEST_TAG` in `.env` to a tag that exists on at least one agent.

```bash
cd tests/UnifyEM
./run_all_tests.sh
```

### PwnDoc tests

Requires a running PwnDoc instance configured in `configs/`.

```bash
cd tests/PwnDoc
./run_all_tests.sh
```

## Test Output

Each test produces timestamped `.log` files in the test directory. Log files are gitignored (`.log` pattern).

## Core Test Coverage

### `test_health.sh`
- Server health status
- Uptime and version fields

### `test_knowledge.sh`
- Store a value (`knowledge_set`)
- Retrieve a value (`knowledge_get`)
- Update a value
- Delete a key and verify it is gone
- Search by domain
- Verify empty domain after cleanup

### `test_knowledge_key.sh`
- Key-based retrieval with explicit domain/key
- Search for specific domain entries

### `test_perf.sh`
- Echo: round-trip with message payload
- Delay: configurable sleep duration
- Random data: generate N bytes of random base64 data
- Error: tool that returns a controlled error response
- Counter: atomic increment and value retrieval

> **Note**: Perf tools must be enabled on the server (`MCP_FUSION_PERF=true` or `--perf` flag). Tests gracefully skip if the tools are not available.

### `test_stress.sh`
- Concurrent load test using MCPProbe's built-in stress mode
- Phase 1–5: connectivity, echo, delay, random data, error injection
- Phase 6: concurrent multi-tool stress across perf tools
- Configurable concurrency and iteration count

### `test_stdio.sh`
- Verifies stdio transport connectivity
