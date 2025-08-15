# MCPFusion Tests

This directory contains individual test scripts for each Microsoft 365 MCP function.

## Test Scripts

- `test_profile.sh` - Tests Microsoft 365 profile API functions
- `test_calendar_summary.sh` - Tests calendar summary API functions
- `test_calendar_details.sh` - Tests calendar details API functions
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
./test_calendar_summary.sh > calendar_summary_output.log
./test_calendar_details.sh > calendar_details_output.log
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

## Test Coverage

Each test covers multiple scenarios:

### Profile Tests
1. Basic profile retrieval with default fields
2. Custom field selection

### Calendar Summary Tests  
1. Calendar summary for last 30 days
2. Calendar summary for next 30 days
3. Calendar summary for specific date range (July 2024)

### Calendar Details Tests
1. Calendar details for last 30 days
2. Calendar details with custom field selection
3. Calendar details for next 30 days
4. Calendar details with minimal field selection

### Mail Tests
1. Default inbox messages
2. Limited message count (5 messages)
3. Unread messages only
4. Custom field selection

### Contacts Tests
1. Default contacts list
2. Limited contact count (10 contacts)
3. Custom field selection

## Prerequisites

- MCPFusion server running on port 8888
- MCPProbe tool available at `/Users/eric/source/MCPProbe/probe`
- Valid Microsoft 365 OAuth authentication
