# MCPFusion Code Review — 2026-03-31

## Scope

Security and reliability review of the MCPFusion Go project. Primary focus: identifying the root cause of intermittent outbound HTTP request hangs that require a server restart to resolve.

---

## Executive Summary

The primary cause of the timeout/hang issue is **HTTP response body exhaustion leading to connection pool starvation**. The retry logic in `fusion/retry.go` does not drain response bodies before retrying on 5xx, 429, or 408 responses, causing connections to remain "in use" even after the retry begins. Over time this exhausts the `MaxConnsPerHost` connection pool limit, causing all subsequent requests to the same host to hang until the 60-second outbound context deadline fires.

Secondary contributing factors are covered below.

---

## Critical Issues

### CRIT-1: Response Body Not Drained Before Retry

**File:** `fusion/retry.go` — retry loop (approx. lines 78–135)
**Severity:** Critical — Root cause of connection pool exhaustion

When the HTTP client receives a 5xx, 429, or 408 response and decides to retry, the response body from the failed attempt is stored in `lastResp` but is never drained before the next request is made. Go's `net/http` documentation is explicit: if the response body is not read to completion and closed, the underlying TCP connection cannot be returned to the pool.

The pattern at issue:

```go
resp, err := client.Do(clonedReq)
shouldRetry, _ := r.shouldRetry(err, resp, attempt)

if !shouldRetry {
    return resp, err
}

if lastResp != nil {
    _ = lastResp.Body.Close()   // Closes the PREVIOUS response body only
}
lastResp = resp                  // Stores CURRENT response — body never drained
```

`Body.Close()` on an unread body does not return the connection to the pool — it signals the intention to abandon the connection. The TCP connection stays checked out until either the remote end closes it or the idle timeout fires. With `MaxConnsPerHost: 50`, after approximately 10–17 retryable errors the pool is fully saturated. New requests block indefinitely on `client.Do()` and hit the 60-second `outboundCtx` deadline.

Restarting the server creates a fresh transport/pool, which explains why restart resolves the issue temporarily.

**Fix:** Before storing `resp` in `lastResp` or before the next loop iteration, drain and close the body:

```go
func drainAndClose(resp *http.Response) {
    if resp != nil && resp.Body != nil {
        _, _ = io.Copy(io.Discard, resp.Body)
        _ = resp.Body.Close()
    }
}
```

Call this on the current `resp` before the retry delay. Also ensure `lastResp` is drained/closed when context is cancelled mid-delay (the `ctx.Done()` branch).

---

### CRIT-2: Per-Request HTTP Client Cloning Fragments the Connection Pool

**File:** `fusion/handler.go` — `ForceNewConnection` and custom timeout path (approx. lines 501–529)
**Severity:** Critical — Accelerates pool exhaustion

When `ForceNewConnection` is set or a custom timeout is specified, a new `*http.Client` with a cloned transport is created per request:

```go
transport := h.fusion.httpClient.Transport.(*http.Transport).Clone()
httpClient = &http.Client{
    Transport: transport,
    Timeout:   ...,
}
```

Each cloned transport has its own independent connection pool. These clients are used once and discarded, but the underlying `*http.Transport` keeps idle connections open until its `IdleConnTimeout` fires. If hundreds of such requests occur, hundreds of isolated connection pools each hold onto sockets. This:

1. Fragments connections away from the global reusable pool.
2. Exhausts system-level ephemeral port and file descriptor limits.
3. When those limits are hit, even the global pool cannot open new connections.

The `Timeout: timeout + time.Minute` logic (line 526) compounds this — it adds an extra 60 seconds to every custom-timeout client, meaning per-request transports hold idle connections much longer than expected.

**Fix:** Cache a small set of per-configuration HTTP clients keyed on (timeout, forceNew). Do not create one per request. At minimum, call `transport.CloseIdleConnections()` when the per-request client is done if truly ephemeral semantics are required.

---

## High Severity Issues

### HIGH-1: Timeout Context Cancellation During Response Body Read Corrupts Connections

**File:** `fusion/handler.go` — `handleResponse()` (approx. lines 130–145, 641–650)
**Severity:** High

The `outboundCtx` has a 60-second deadline. Inside `handleResponse()`, the entire response body is read with `io.ReadAll(resp.Body)`. If the context deadline fires mid-read (e.g., a slow upstream sending a large response), `io.ReadAll` returns a partial read. The deferred `resp.Body.Close()` then closes an incompletely read body.

As with CRIT-1, closing without fully reading does not return the connection to the pool. The connection is abandoned. This is a less frequent but real contributor to pool exhaustion.

**Fix:** Use `io.LimitReader` to cap response size before reading (see LOW-3 below), and consider reading the body within the context so that cancellation terminates the read promptly. When the read is cancelled, drain with a short deadline before closing.

---

### HIGH-2: Circuit Breaker TOCTOU Race Condition

**File:** `hub/client.go` — `isCircuitOpen()` (approx. lines 150–191)
**Severity:** High — Correctness issue

The circuit breaker transitions from OPEN to HALF-OPEN without holding the mutex for the full check-and-update operation:

```go
func (cb *CircuitBreaker) isCircuitOpen() bool {
    cb.mu.RLock()
    state := cb.state
    lastFail := cb.lastFailureTime
    cb.mu.RUnlock()                            // Lock released here

    if state == cbOpen && time.Since(lastFail) > cbOpenDuration {
        cb.mu.Lock()
        cb.state = cbHalfOpen                  // State transition with stale check
        cb.mu.Unlock()
    }
    ...
}
```

Between the `RUnlock` and the subsequent `Lock`, another goroutine may have already transitioned the state. Multiple goroutines can simultaneously see OPEN, all transition to HALF-OPEN, and all let their requests through — defeating the purpose of HALF-OPEN probe limiting. This is a classic TOCTOU race.

**Fix:** Use a single `Lock()` for the entire check-and-update:

```go
cb.mu.Lock()
defer cb.mu.Unlock()
if cb.state == cbOpen && time.Since(cb.lastFailureTime) > cbOpenDuration {
    cb.state = cbHalfOpen
}
return cb.state == cbOpen
```

---

### HIGH-3: Background Goroutines Not Properly Cancellable on Shutdown

**File:** `fusion/auth_strategies.go` (device flow polling, approx. lines 169–170)
**Severity:** High — Goroutine leak

Device flow token polling launches a goroutine with `context.Background()`:

```go
go s.backgroundTokenPolling(context.Background(), ...)
```

This goroutine runs for the full device flow timeout (typically 10 minutes) regardless of whether the server is shutting down. If authentication is frequently attempted (e.g., repeated client connections), these goroutines accumulate. Each holds references to auth state and makes periodic HTTP requests to OAuth endpoints.

**Fix:** Pass a cancellable context derived from a server-lifetime context so the goroutine is cancelled on shutdown.

---

### HIGH-4: Retry Loop Does Not Drain Body on Context Cancellation Branch

**File:** `fusion/retry.go` — context cancel during sleep (approx. lines 120–132)
**Severity:** High

When context is cancelled during the inter-retry delay:

```go
case <-ctx.Done():
    if resp != nil {
        _ = resp.Body.Close()   // Close without draining
    }
    return lastResp, ctx.Err()
```

`resp.Body.Close()` is called on the current attempt's response without draining. Same root cause as CRIT-1 — the connection is abandoned rather than returned to the pool.

**Fix:** Drain `resp.Body` before closing in this branch, same as the main retry fix.

---

## Medium Severity Issues

### MED-1: Metrics Reporter Goroutine Context Handling

**File:** `fusion/metrics.go` — `RegisterMetricsReporting()` (approx. lines 343–355)
**Severity:** Medium

The periodic metrics logging goroutine checks `ctx.Done()` correctly, but if the context is cancelled while the goroutine is in the middle of writing metrics output, the write completes before the cancellation is honoured. This is benign in most cases but could cause a final metrics write to a closed/unavailable sink. Consider using `select` with `ctx.Done()` at every blocking point.

---

### MED-2: No Early Context Cancellation Check in Handle()

**File:** `fusion/handler.go` — `Handle()` entry (approx. line 73)
**Severity:** Medium

`Handle()` does not check whether the incoming `ctx` is already cancelled before beginning request construction. If a client disconnects before the handler starts (e.g., under heavy load), the server still builds and dispatches the full outbound request, wasting resources.

**Fix:** Add `if err := ctx.Err(); err != nil { return "", err }` at the top of `Handle()`.

---

### MED-3: Command Executor Goroutine Cancel Race

**File:** `fusion/command_executor.go` — background monitor goroutine (approx. lines 77–91)
**Severity:** Medium

The goroutine that monitors parent context and cancels the command context captures `cancel` by value at goroutine start. If the goroutine is scheduled to run before `cancel` is fully initialized (possible on a heavily loaded scheduler), it may call nil. In practice Go closures capture by reference so this is low probability, but the pattern is fragile.

---

### MED-4: Retry on 5xx Without Backoff Validation

**File:** `fusion/retry.go` — `shouldRetry()` (approx. lines 155–180)
**Severity:** Medium

All 5xx responses are retried. If a PwnDoc instance returns persistent 5xx (e.g., database failure), MCPFusion will hammer it at maximum rate for `MaxAttempts` retries. The exponential backoff logic exists but its configuration should be validated to ensure the base delay is non-zero and jitter is applied correctly to avoid thundering herd after transient outages.

---

## Low Severity Issues

### LOW-1: Response Body Size Not Limited Before Read

**File:** `fusion/handler.go` — `handleResponse()` (approx. line 644)
**Severity:** Low

```go
bodyBytes, err := io.ReadAll(resp.Body)
```

The entire response body is buffered in memory before checking `MaxResponseBytes`. A malicious or misbehaving upstream could send gigabytes of data, causing OOM before the size check is reached.

**Fix:** Wrap the body with `io.LimitReader` before calling `io.ReadAll`:

```go
limited := io.LimitReader(resp.Body, int64(h.fusion.MaxResponseBytes())+1)
bodyBytes, err := io.ReadAll(limited)
```

---

### LOW-2: Knowledge Search Has No Query Length Limit

**File:** `db/knowledge.go` — `SearchKnowledge()` (approx. lines 455–536)
**Severity:** Low

The search query is accepted without a length limit. A very long query causes the BoltDB scan to perform `strings.Contains` with a large needle on every stored key and value, which is a CPU/memory DoS vector from authenticated clients.

**Fix:** Validate and cap query length (e.g., 512 characters) at the MCP tool handler level before calling into the DB layer.

---

### LOW-3: Silent Error in Response Body Close Defer

**File:** `fusion/auth_strategies.go` (approx. lines 267–270)
**Severity:** Low

```go
defer func(Body io.ReadCloser) {
    err := Body.Close()
    if err != nil {
        // empty — error silently discarded
    }
}(resp.Body)
```

While close errors on response bodies are rarely actionable, the empty handler suppresses potentially useful diagnostic information. Log at debug level.

---

### LOW-4: Unvalidated Path Parameter Lengths

**File:** `fusion/mapper.go` — path parameter substitution (approx. lines 32–91)
**Severity:** Low

Path parameters are URL-encoded via `url.QueryEscape()` before substitution, which prevents injection. However there is no upper-bound check on parameter length. An extremely long parameter (e.g., a 10MB string) would be URL-encoded and sent as a path segment, likely rejected by the upstream but wasting CPU and memory in the encoding step.

---

## Connection Pool Exhaustion — Failure Mode Walkthrough

For clarity, here is the precise sequence that produces the observed symptom:

1. **Normal operation:** Global `*http.Transport` maintains a pool capped at `MaxConnsPerHost: 50` per upstream host.
2. **Retryable error occurs (5xx/429/408):** `client.Do()` returns a response. The body is stored in `lastResp` without draining.
3. **Retry issued:** A new connection is taken from the pool. The previous connection remains "checked out" because its response body is open.
4. **Repetition:** Each retryable error permanently removes a connection from the available pool. With `MaxAttempts: 3`, each operation that exhausts retries costs up to 3 pool slots.
5. **Pool saturated:** After ~17 distinct retryable-error sequences (50 connections ÷ 3 per retry), the pool is exhausted. Subsequent `client.Do()` calls block waiting for a connection.
6. **Outbound context fires:** The 60-second `outboundCtx` deadline fires, causing the blocked `client.Do()` to return a timeout error. This is the symptom observed in logs.
7. **All requests affected:** Every request to the same upstream host (e.g., PwnDoc) now times out immediately at the `client.Do()` call. The log shows "start of outbound HTTP session" followed by timeout because `client.Do()` is reached but blocks.
8. **Restart:** Creating a new server process creates a fresh `*http.Transport` with an empty connection pool. All connections become available again. Issue temporarily resolved.

The leak is slow if errors are infrequent. The pool may take hours or days to fill under low error rates, which matches the description of the issue occurring "every so often."

---

## Priority Remediation Order

| Priority | Issue | File | Impact |
|---|---|---|---|
| 1 | CRIT-1: Drain response bodies before retry | `fusion/retry.go` | Root cause of hangs |
| 2 | CRIT-2: Stop creating per-request HTTP clients | `fusion/handler.go` | Accelerates exhaustion |
| 3 | HIGH-1: Context cancellation mid-body-read | `fusion/handler.go` | Secondary exhaustion |
| 4 | HIGH-4: Drain body on context cancel in retry | `fusion/retry.go` | Related to CRIT-1 |
| 5 | HIGH-2: Circuit breaker TOCTOU race | `hub/client.go` | Correctness |
| 6 | HIGH-3: Goroutine leak in device flow | `fusion/auth_strategies.go` | Resource leak |
| 7 | LOW-1: Unbounded response body read | `fusion/handler.go` | DoS from upstream |
| 8 | MED-2: No early context check | `fusion/handler.go` | Wasted work on disconnect |
| 9 | LOW-2: Knowledge search query length | `db/knowledge.go` | Auth'd DoS |

---

*Review performed 2026-03-31. No code was modified during this review.*
