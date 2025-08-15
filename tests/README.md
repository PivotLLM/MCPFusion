# MCPFusion Tests

This directory contains individual test scripts for each Microsoft 365 MCP function.

## Test Scripts

- `test_profile.sh` - Tests Microsoft 365 profile API functions
- `test_calendar.sh` - Tests calendar API functions (summary and details)
- `test_mail.sh` - Tests mail/inbox API functions
- `test_contacts.sh` - Tests contacts API functions
- `run_all_tests.sh` - Master test runner that executes all tests

## Running Tests

### Run All Tests
```bash
cd tests
./run_all_tests.sh
```

### Run Individual Tests
```bash
cd tests
./test_profile.sh > profile_output.log
./test_calendar.sh > calendar_output.log
./test_mail.sh > mail_output.log
./test_contacts.sh > contacts_output.log
```

## Test Output

- Each test run creates timestamped `.log` files with complete request/response data
- Log files are automatically gitignored (`.log` pattern in `.gitignore`)
- Log files include:
  - Test metadata (timestamp, server URL, parameters)
  - Full probe tool output with request details
  - Complete JSON API responses

## Prerequisites

- MCPFusion server running on port 8888
- MCPProbe tool available at `/Users/eric/source/MCPProbe/probe`
- Valid Microsoft 365 OAuth authentication
