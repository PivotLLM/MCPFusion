/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tenebris-tech/mlogger"
)

// newLimitedHandler is newHandleResponseHandler with a response size limit.
func newLimitedHandler(t *testing.T, limit int) *HTTPHandler {
	t.Helper()
	config := &Config{
		Services: map[string]*ServiceConfig{
			"testsvc": {
				Name:    "testsvc",
				BaseURL: "http://localhost",
				Auth:    AuthConfig{Type: AuthTypeBearer, Config: map[string]any{"token": "test-token"}},
				Endpoints: []EndpointConfig{{
					ID: "testep", Name: "Test Endpoint", Description: "Test", Method: "GET", Path: "/test",
					Parameters: []ParameterConfig{}, Response: ResponseConfig{Type: ResponseTypeJSON},
				}},
			},
		},
	}
	f, err := New(WithConfig(config), WithLogger(mlogger.NewMemoryLogger()), WithMaxResponseBytes(limit))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	svc := config.Services["testsvc"]
	return NewHTTPHandler(f, svc, &svc.Endpoints[0])
}

// respond runs body through handleResponse and decodes the JSON it returns.
func respond(t *testing.T, h *HTTPHandler, body any) (string, any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	result, err := h.handleResponse(context.Background(), syntheticResponse(http.StatusOK, "application/json", raw), "test", nil)
	if err != nil {
		t.Fatalf("handleResponse: %v", err)
	}
	var decoded any
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		return result, nil
	}
	return result, decoded
}

// A message whose body is far over the limit comes back as valid JSON within
// the limit: the body is cut and marked, every other member is as it was, and
// "_truncated" names the cut value with its sizes.
func TestTruncate_LongTextValueIsShortened(t *testing.T) {
	const limit = 20_000
	h := newLimitedHandler(t, limit)
	body := strings.Repeat("Jira notice. ", 10_000) // 130,000 characters
	message := map[string]any{
		"id":      "m1",
		"subject": "SO-288 updated",
		"from":    map[string]any{"emailAddress": map[string]any{"address": "bob@example.com"}},
		"body":    map[string]any{"contentType": "html", "content": body},
		"toRecipients": []any{
			map[string]any{"emailAddress": map[string]any{"address": "alice@example.com"}},
		},
		"isRead": false,
		"size":   1480000,
	}
	result, decoded := respond(t, h, message)
	if len(result) > limit {
		t.Fatalf("result is %d bytes, over the limit of %d", len(result), limit)
	}
	obj, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("result is not a JSON object: %.200s", result)
	}
	if obj["id"] != "m1" || obj["subject"] != "SO-288 updated" || obj["isRead"] != false || obj["size"] != float64(1480000) {
		t.Errorf("members other than the long one changed: %v", obj)
	}
	if got := obj["toRecipients"].([]any)[0].(map[string]any)["emailAddress"].(map[string]any)["address"]; got != "alice@example.com" {
		t.Errorf("toRecipients changed: %v", obj["toRecipients"])
	}
	content := obj["body"].(map[string]any)["content"].(string)
	if !strings.HasPrefix(content, "Jira notice. Jira notice. ") || !strings.Contains(content, "... [truncated: first ") ||
		!strings.HasSuffix(content, " of 130000 characters]") {
		t.Errorf("cut content = %.60s ... %s", content, content[max(0, len(content)-70):])
	}
	// As much as fits is kept: the result is close to the limit.
	if len(result) < limit*9/10 {
		t.Errorf("result is %d bytes of a %d limit: more of the value could have been kept", len(result), limit)
	}

	notice, ok := obj[truncationKey].(map[string]any)
	if !ok {
		t.Fatalf("no %s member: %v", truncationKey, obj)
	}
	fields := notice["fields"].([]any)
	if len(fields) != 1 {
		t.Fatalf("fields = %v", fields)
	}
	field := fields[0].(map[string]any)
	kept := int(field["kept_chars"].(float64))
	if field["path"] != "body.content" || field["original_chars"] != float64(130000) || kept < minTruncatedChars ||
		!strings.Contains(content, fmt.Sprintf("first %d of 130000", kept)) {
		t.Errorf("field = %v", field)
	}
	if notice["limit_bytes"] != float64(limit) || notice["original_bytes"].(float64) < 130000 {
		t.Errorf("notice = %v", notice)
	}
}

// A response under the limit is returned as it is, with no notice.
func TestTruncate_UnderLimitUntouched(t *testing.T) {
	h := newLimitedHandler(t, 20_000)
	result, decoded := respond(t, h, map[string]any{"id": "m1", "body": strings.Repeat("x", 5_000)})
	obj := decoded.(map[string]any)
	if _, noticed := obj[truncationKey]; noticed || obj["body"] != strings.Repeat("x", 5_000) {
		t.Errorf("a response under the limit was changed: %.200s", result)
	}
}

// The longest values are cut first: a short value beside a long one is kept
// whole, and two long ones are cut to the same length.
func TestTruncate_LongestFirst(t *testing.T) {
	h := newLimitedHandler(t, 12_000)
	_, decoded := respond(t, h, map[string]any{
		"short": strings.Repeat("s", 1_000),
		"long":  strings.Repeat("a", 40_000),
		"also":  strings.Repeat("b", 60_000),
	})
	obj := decoded.(map[string]any)
	if obj["short"] != strings.Repeat("s", 1_000) {
		t.Error("the short value was cut")
	}
	kept := func(key string) int {
		return strings.Index(obj[key].(string), " ... [truncated")
	}
	if kept("long") < minTruncatedChars || kept("long") != kept("also") {
		t.Errorf("kept %d of long and %d of also; want the same", kept("long"), kept("also"))
	}
	fields := obj[truncationKey].(map[string]any)["fields"].([]any)
	if len(fields) != 2 {
		t.Errorf("fields = %v", fields)
	}
}

// A list response keeps its shape and every element: only the long values
// inside are cut, with their markers.
func TestTruncate_ListKeepsEveryElement(t *testing.T) {
	const limit = 30_000
	h := newLimitedHandler(t, limit)
	list := make([]any, 20)
	for i := range list {
		list[i] = map[string]any{"id": fmt.Sprintf("m%d", i), "body": strings.Repeat("y", 10_000)}
	}
	result, decoded := respond(t, h, list)
	got, ok := decoded.([]any)
	if !ok || len(got) != 20 || len(result) > limit {
		t.Fatalf("result: %d bytes, %T with %d elements", len(result), decoded, len(got))
	}
	for i, el := range got {
		m := el.(map[string]any)
		if m["id"] != fmt.Sprintf("m%d", i) || !strings.Contains(m["body"].(string), "[truncated: first ") {
			t.Fatalf("element %d = %.80v", i, m)
		}
	}
}

// A response that is large because it holds many records, not long text, is
// refused: no record is ever dropped.
func TestTruncate_ManyRecordsRefused(t *testing.T) {
	h := newLimitedHandler(t, 10_000)
	list := make([]any, 2_000)
	for i := range list {
		list[i] = map[string]any{"id": fmt.Sprintf("message-%d", i), "subject": "Weekly report"}
	}
	result, decoded := respond(t, h, list)
	if decoded != nil || !strings.HasPrefix(result, "Response too large (") || !strings.Contains(result, "Request fewer records or fields") {
		t.Errorf("result = %.200s", result)
	}
	// So is one whose long values are too many to fit even at the minimum.
	for i := range list {
		list[i] = map[string]any{"id": i, "body": strings.Repeat("z", 5_000)}
	}
	if result, decoded = respond(t, h, list); decoded != nil || !strings.HasPrefix(result, "Response too large (") {
		t.Errorf("result = %.200s", result)
	}
}

// A cut never splits a character, and an object that already has a
// "_truncated" member keeps its own.
func TestTruncate_CharactersAndExistingKey(t *testing.T) {
	h := newLimitedHandler(t, 8_000)
	_, decoded := respond(t, h, map[string]any{"text": strings.Repeat("é日本", 20_000), truncationKey: "theirs"})
	obj := decoded.(map[string]any)
	text := obj["text"].(string)
	if !utf8.ValidString(text) || !strings.Contains(text, "[truncated: first ") {
		t.Errorf("cut text is not valid UTF-8 or not marked: %.60s", text)
	}
	if obj[truncationKey] != "theirs" {
		t.Errorf("%s = %v, want the response's own", truncationKey, obj[truncationKey])
	}
}

// The notice lists at most maxTruncationFields values and counts the rest.
func TestTruncate_NoticeIsBounded(t *testing.T) {
	fields := make([]truncatedField, maxTruncationFields+7)
	n := newTruncationNotice(100, 200, fields)
	if len(n.Fields) != maxTruncationFields || n.MoreFields != 7 {
		t.Errorf("notice lists %d fields and %d more", len(n.Fields), n.MoreFields)
	}
	if got := len(truncationMarker(9_999_999_999, 9_999_999_999)); got > truncationMarkerRoom {
		t.Errorf("a marker can be %d characters, more than truncationMarkerRoom", got)
	}
}
