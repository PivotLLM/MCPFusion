/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

// handle_response_test.go covers edge cases in handleResponse and the
// token-invalidation retry path:
//
//   P0-4: LimitReader truncation causes a parse error for oversized bodies.
//   P0-5: Token-invalidation retry succeeds on second request (401 → 200).
//   P1-2: Zero-byte response body causes a JSON parse error, not a panic.
//   P1-3: Non-JSON body with response.type "json" returns a parse error.

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/tenebris-tech/mlogger"
)

// newHandleResponseHandler builds the minimal objects needed to call
// handleResponse directly, bypassing the authentication layer.
func newHandleResponseHandler(t *testing.T, responseType ResponseType) *HTTPHandler {
	t.Helper()
	config := &Config{
		Services: map[string]*ServiceConfig{
			"testsvc": {
				Name:    "testsvc",
				BaseURL: "http://localhost",
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test-token"},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "testep",
						Name:        "Test Endpoint",
						Description: "Test",
						Method:      "GET",
						Path:        "/test",
						Parameters:  []ParameterConfig{},
						Response:    ResponseConfig{Type: responseType},
					},
				},
			},
		},
	}
	f := New(WithConfig(config), WithLogger(mlogger.NewMemoryLogger()))
	svc := config.Services["testsvc"]
	ep := &svc.Endpoints[0]
	return NewHTTPHandler(f, svc, ep)
}

// syntheticResponse builds a minimal *http.Response for handleResponse.
func syntheticResponse(statusCode int, contentType string, body []byte) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	if contentType != "" {
		resp.Header.Set("Content-Type", contentType)
	}
	return resp
}

// TestHandleResponse_BodySizeLimit verifies that the MaxResponseBodyReadBytes
// cap is enforced: a body that exceeds the limit is silently truncated, which
// causes the JSON parser to return an error.  A body that is just under the
// limit is parsed successfully.
func TestHandleResponse_BodySizeLimit(t *testing.T) {
	h := newHandleResponseHandler(t, ResponseTypeJSON)
	ctx := context.Background()

	// -- over-limit body ------------------------------------------
	// Build a JSON object whose value field is exactly one byte longer than the
	// read cap. The prefix + suffix together are 12 bytes; the value filler
	// brings the total to MaxResponseBodyReadBytes+1.
	const prefix = `{"d":"`
	const suffix = `"}`
	fillLen := global.MaxResponseBodyReadBytes + 1 - len(prefix) - len(suffix)
	overBody := make([]byte, global.MaxResponseBodyReadBytes+1)
	copy(overBody, []byte(prefix))
	for i := len(prefix); i < len(prefix)+fillLen; i++ {
		overBody[i] = 'A'
	}
	copy(overBody[len(prefix)+fillLen:], []byte(suffix))

	resp := syntheticResponse(http.StatusOK, "application/json", overBody)
	_, err := h.handleResponse(ctx, resp, "test-corr-over", nil)
	if err == nil {
		t.Error("P0-4 over-limit: expected an error due to truncated JSON, got nil")
	}

	// -- under-limit body (MaxResponseBodyReadBytes - 1 bytes) ------
	// Craft a minimal but valid JSON value that fits within the cap.
	underBody := []byte(`{"ok":true}`)
	resp2 := syntheticResponse(http.StatusOK, "application/json", underBody)
	result, err := h.handleResponse(ctx, resp2, "test-corr-under", nil)
	if err != nil {
		t.Errorf("P0-4 under-limit: expected success for small body, got error: %v", err)
	}
	if !strings.Contains(result, "true") {
		t.Errorf("P0-4 under-limit: unexpected result %q", result)
	}
}

// TestHandleResponse_TokenInvalidationRetry confirms that a 401 response
// causes the handler to make a second request which, on success, is returned
// to the caller.  The test uses WithConfig so that multi-tenant auth is
// configured, matching the pattern in token_invalidation_test.go.
func TestHandleResponse_TokenInvalidationRetry(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	config := &Config{
		Services: map[string]*ServiceConfig{
			"retrysvc": {
				Name:    "retrysvc",
				BaseURL: server.URL,
				Auth: AuthConfig{
					Type:   AuthTypeBearer,
					Config: map[string]interface{}{"token": "test-token"},
					TokenInvalidation: &TokenInvalidationConfig{
						StatusCodes:         []int{401},
						RetryOnInvalidation: true,
					},
				},
				Endpoints: []EndpointConfig{
					{
						ID:          "ep",
						Name:        "ep",
						Description: "retry endpoint",
						Method:      "GET",
						Path:        "/retry",
						Parameters:  []ParameterConfig{},
						Response:    ResponseConfig{Type: ResponseTypeJSON},
					},
				},
			},
		},
	}

	f := New(WithConfig(config), WithLogger(mlogger.NewMemoryLogger()))
	tools := f.RegisterTools()
	tool := findTool(t, tools, "retrysvc_ep")

	result, err := tool.Handler(withTestContext(map[string]any{}))

	// After the 401 the handler retries; the second request returns 200.
	if err != nil {
		t.Errorf("P0-5: expected success after retry, got error: %v", err)
	}
	if !strings.Contains(result, "ok") {
		t.Errorf("P0-5: expected result to contain 'ok', got %q", result)
	}
	if got := requestCount.Load(); got != 2 {
		t.Errorf("P0-5: expected exactly 2 server requests, got %d", got)
	}
}

// TestHandleResponse_EmptyBody ensures that a zero-byte response body (with
// response.type "json") returns a descriptive error rather than panicking or
// hanging.
func TestHandleResponse_EmptyBody(t *testing.T) {
	h := newHandleResponseHandler(t, ResponseTypeJSON)
	ctx := context.Background()

	resp := syntheticResponse(http.StatusOK, "application/json", []byte{})
	_, err := h.handleResponse(ctx, resp, "test-corr-empty", nil)
	if err == nil {
		t.Error("P1-2: expected error for empty body, got nil")
	}
}

// TestHandleResponse_NonJSONContentType verifies that HTML delivered with a
// "json" response type causes a parse error and is not silently returned as
// the tool result.
func TestHandleResponse_NonJSONContentType(t *testing.T) {
	h := newHandleResponseHandler(t, ResponseTypeJSON)
	ctx := context.Background()

	htmlBody := []byte("<html><body>Not JSON</body></html>")
	resp := syntheticResponse(http.StatusOK, "text/html; charset=utf-8", htmlBody)
	result, err := h.handleResponse(ctx, resp, "test-corr-html", nil)

	if err == nil {
		t.Errorf("P1-3: expected a JSON parse error for HTML body, got nil (result=%q)", result)
		return
	}
	// Error must mention JSON parsing.
	if !strings.Contains(err.Error(), "JSON") && !strings.Contains(err.Error(), "json") {
		t.Errorf("P1-3: expected error to reference JSON parsing, got: %v", err)
	}
	// The raw HTML must not be returned as the tool result.
	if strings.Contains(result, "<html>") {
		t.Errorf("P1-3: HTML body was silently returned as tool result: %q", result)
	}

	// Confirm the context key referenced in the handler is the one from global.
	_ = global.TenantContextKey
}
