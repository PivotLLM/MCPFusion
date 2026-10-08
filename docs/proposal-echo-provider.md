# Echo Provider — Implementation Specification

## Overview

The echo provider is a built-in MCPFusion tool provider designed exclusively for
stress testing, load testing, and diagnostics. It has no external dependencies
and is never enabled in production configurations. It exposes a small set of
controllable tools that simulate various workload characteristics — fast
responses, slow responses, large payloads, errors, and concurrent counter
operations.

## Activation

The echo provider must **not** be active by default. It should be enabled via a
dedicated JSON config block:

```json
{
  "echo": {
    "enabled": true
  }
}
```

Alternatively, a command-line flag (`--echo`) may be supported for convenience
during local development. When not explicitly enabled, none of the echo tools
should be registered or visible to MCP clients.

## Tools

### `echo`

Returns the input message unchanged. Used to verify routing and serialization
under load.

**Parameters:**
| Name      | Type   | Required | Description                  |
|-----------|--------|----------|------------------------------|
| `message` | string | yes      | Arbitrary string to echo back |

**Response:** `{"message": "<input message>"}`

---

### `delay`

Sleeps for the specified duration, then returns. Used to simulate slow upstream
responses and test timeout handling, goroutine lifecycle, and connection
persistence.

**Parameters:**
| Name      | Type    | Required | Description                       |
|-----------|---------|----------|-----------------------------------|
| `seconds` | number  | yes      | How long to sleep (max 60)        |

**Response:** `{"slept_seconds": <n>}`

**Notes:**
- Cap at 60 seconds to prevent runaway goroutines in test environments.
- The goroutine must respect context cancellation — if the client disconnects
  during the delay, the sleep should abort cleanly. This is specifically
  intended to test that MCPFusion does not leak goroutines on client disconnect.

---

### `random_data`

Returns a response body of the specified size filled with random bytes encoded
as a hex string. Used to test large response handling, buffering, and memory
behaviour under load.

**Parameters:**
| Name    | Type    | Required | Description                         |
|---------|---------|----------|-------------------------------------|
| `bytes` | integer | yes      | Number of bytes to return (max 1MB) |

**Response:** `{"bytes": <n>, "data": "<hex string>"}`

**Notes:**
- Cap at 1,048,576 bytes (1 MiB) to prevent accidental OOM in test
  environments.
- Data does not need to be cryptographically random; `math/rand` is fine.

---

### `error`

Returns a tool-level error with the provided message. Used to test MCPFusion's
error propagation path under load and verify that errors are handled correctly
without leaking resources.

**Parameters:**
| Name      | Type   | Required | Description                   |
|-----------|--------|----------|-------------------------------|
| `message` | string | no       | Error message (default: "echo provider error") |

**Response:** Returns an MCP tool error (not a successful result).

---

### `counter`

Atomically increments a global in-memory counter and returns the new value.
Under concurrent load, every response must be unique and monotonically
increasing — no two callers should receive the same count. This is specifically
designed to surface race conditions.

**Parameters:** None

**Response:** `{"count": <n>}`

**Notes:**
- Use `sync/atomic` or a mutex-protected integer. Do not use a plain `int`.
- The counter resets to zero when MCPFusion restarts.
- If a race condition exists, concurrent callers may return duplicate values,
  which test scripts can detect.

---

## Implementation Notes

### Package structure

Place the provider in a new package: `providers/echo/` following the same
pattern as other providers. Register it in `main.go` only when the echo config
block is present and `enabled: true`.

### No persistence

The echo provider holds no persistent state except the in-memory counter. It
must not write to BoltDB, create files, or make network calls.

### Logging

Use `mlogger` at debug level for individual tool calls. At info level, log only
when the provider is registered (so it is visible in logs that the test mode is
active).

### Context propagation

All tools must accept a `context.Context` and respect cancellation, particularly
the `delay` tool. This is a primary goal of the echo provider — to verify that
MCPFusion correctly propagates context cancellation to in-flight handlers.

---

## pprof Endpoint (Separate Recommendation)

Independently of the echo provider, MCPFusion should expose Go's standard
`net/http/pprof` endpoints when started with a `--pprof <addr>` flag
(e.g. `--pprof :6060`). This allows heap and goroutine profiles to be captured
before, during, and after stress tests:

```bash
# Before stress test
go tool pprof http://localhost:6060/debug/pprof/heap

# After stress test — compare to baseline
go tool pprof http://localhost:6060/debug/pprof/heap

# Check for leaked goroutines
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

This should **never** be enabled in production and should require an explicit
flag. pprof is not part of the echo provider — it is a separate diagnostic
feature.

---

## Usage with Stress Test Script

Once the echo provider is enabled, `tests/test_stress.sh` will automatically
detect it and run Phase 6, which includes:

| Phase              | Tool          | Repeat | Concurrent | Purpose                        |
|--------------------|---------------|--------|------------|--------------------------------|
| Echo warm-up       | `echo`        | 50     | 10         | Baseline routing throughput    |
| Echo medium        | `echo`        | 200    | 25         | Sustained concurrency          |
| Echo maximum       | `echo`        | 1000   | 50         | Peak load                      |
| Delay (light)      | `delay` (2s)  | 10     | 5          | Goroutine lifecycle            |
| Delay (concurrent) | `delay` (5s)  | 20     | 10         | Concurrent slow upstreams      |
| Large (10KB)       | `random_data` | 100    | 20         | Response buffering             |
| Large (100KB)      | `random_data` | 50     | 10         | Memory under large payloads    |
| Error handling     | `error`       | 100    | 20         | Error path under load          |
| Race detection     | `counter`     | 1000   | 50         | Concurrent write correctness   |
