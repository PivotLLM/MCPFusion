# PwnDoc Integration Tests

This directory contains integration test scripts for the PwnDoc MCP tools.

## Test Scripts

### Core API Tests
- `test_audits.sh` - Tests audit listing and retrieval
- `test_findings.sh` - Tests finding CRUD operations
- `test_clients.sh` - Tests client and company management

## Prerequisites

- MCPFusion server running with PwnDoc configuration
- MCPProbe tool available
- Valid PwnDoc credentials configured in MCPFusion environment

## Environment Setup

Create a `.env` file in this directory:

```bash
APIKEY=your-mcpfusion-api-token
SERVER_URL=http://127.0.0.1:9999
```

## Running Tests

### Run Individual Tests
```bash
cd tests/PwnDoc
./test_audits.sh
./test_findings.sh
./test_clients.sh
```

## Test Coverage

### Audit Tests
1. List all audits
2. Get specific audit details
3. Get audit findings

### Finding Tests
1. Create a new finding
2. Update finding details
3. Search findings

### Client Tests
1. List all clients
2. List all companies

## Authentication

PwnDoc uses session JWT authentication. MCPFusion handles authentication automatically based on the configuration in `pwndoc.json`. The tests only require an MCPFusion API token for authorization.
