/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"
)

// Fitting an oversized JSON response into the response size limit.
//
// A response over the limit used to be refused whole, so one long text value
// (the HTML body of an e-mail, a log, a base64 attachment) made its whole
// record unreadable. It is now shortened instead: the longest text values are
// cut, longest first, until the response fits, and the result stays valid
// JSON with every key, number, list and object as the API returned them.
//
// Only text values are cut. Nothing is ever removed: no list element, no
// object member. A response that is too large because it holds many records
// (and not long text) therefore still does not fit and is refused as before,
// since dropping records silently would hide data from a caller that pages or
// counts them.
//
// A cut value ends with a marker saying how much of it was kept, and when the
// response is a JSON object a "_truncated" member lists every cut value with
// its path and sizes. A response that is a JSON list keeps its shape, so only
// the markers show the cuts.

const (
	// truncationKey is the member added to a shortened object response.
	truncationKey = "_truncated"
	// minTruncatedChars is the least a text value is cut to: a response that
	// is still over the limit with every long value cut to this is refused.
	minTruncatedChars = 256
	// maxTruncationFields caps the cut values listed under truncationKey.
	maxTruncationFields = 50
)

// truncatedField is one text value that was cut.
type truncatedField struct {
	Path          string `json:"path"`
	OriginalChars int    `json:"original_chars"`
	KeptChars     int    `json:"kept_chars"`
}

// truncationNotice is the value of the truncationKey member.
type truncationNotice struct {
	Note          string           `json:"note"`
	LimitBytes    int              `json:"limit_bytes"`
	OriginalBytes int              `json:"original_bytes"`
	Fields        []truncatedField `json:"fields"`
	MoreFields    int              `json:"more_fields,omitempty"`
}

// marshalResponse is how a JSON response is encoded for the caller.
func marshalResponse(data any) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

// fitResponse shortens the long text values of data until its encoding is at
// most limit bytes. originalBytes is the size of data's own encoding, for the
// notice. It returns the encoding and the values it cut, or ok false when the
// response does not fit even with every long text value cut to
// minTruncatedChars. data is not modified.
func fitResponse(data any, limit, originalBytes int) (out []byte, cut []truncatedField, ok bool) {
	longest := longestString(data)
	if longest <= minTruncatedChars+truncationMarkerRoom {
		return nil, nil, false // no text to cut: the response is large for another reason
	}
	encode := func(keep int) ([]byte, []truncatedField, bool) {
		var fields []truncatedField
		shortened := cutStrings(data, "", keep, &fields)
		if obj, isObject := shortened.(map[string]any); isObject {
			if _, taken := obj[truncationKey]; !taken {
				obj[truncationKey] = newTruncationNotice(limit, originalBytes, fields)
			}
		}
		encoded, err := marshalResponse(shortened)
		if err != nil {
			return nil, nil, false
		}
		return encoded, fields, len(encoded) <= limit
	}
	if _, _, fits := encode(minTruncatedChars); !fits {
		return nil, nil, false
	}
	// The largest keep that fits: the encoding only grows with keep, so a
	// binary search finds it.
	low, high := minTruncatedChars, longest-1 // low fits; high may not
	for low < high {
		mid := low + (high-low+1)/2
		if _, _, fits := encode(mid); fits {
			low = mid
		} else {
			high = mid - 1
		}
	}
	out, cut, _ = encode(low)
	return out, cut, true
}

// newTruncationNotice describes the cut values, listing at most
// maxTruncationFields of them.
func newTruncationNotice(limit, originalBytes int, fields []truncatedField) truncationNotice {
	n := truncationNotice{
		Note: "This response was larger than the response size limit, so its longest text values were shortened. " +
			"Everything else is complete. Request fewer fields to get a value in full.",
		LimitBytes:    limit,
		OriginalBytes: originalBytes,
		Fields:        fields,
	}
	if len(fields) > maxTruncationFields {
		n.Fields, n.MoreFields = fields[:maxTruncationFields], len(fields)-maxTruncationFields
	}
	return n
}

// longestString is the length in characters of the longest text value in v.
func longestString(v any) int {
	switch node := v.(type) {
	case string:
		return utf8.RuneCountInString(node)
	case map[string]any:
		longest := 0
		for _, e := range node {
			longest = max(longest, longestString(e))
		}
		return longest
	case []any:
		longest := 0
		for _, e := range node {
			longest = max(longest, longestString(e))
		}
		return longest
	}
	return 0
}

// cutStrings returns a copy of v in which every text value longer than keep
// characters holds its first keep characters and a marker; each cut is added
// to fields with its path (dots between members, [i] for a list element).
func cutStrings(v any, path string, keep int, fields *[]truncatedField) any {
	switch node := v.(type) {
	case string:
		chars := utf8.RuneCountInString(node)
		if chars <= keep+truncationMarkerRoom {
			return node // cutting it would not make it shorter
		}
		*fields = append(*fields, truncatedField{Path: path, OriginalChars: chars, KeptChars: keep})
		return firstRunes(node, keep) + truncationMarker(chars, keep)
	case map[string]any:
		out := make(map[string]any, len(node)+1)
		for k, e := range node {
			child := k
			if path != "" {
				child = path + "." + k
			}
			out[k] = cutStrings(e, child, keep, fields)
		}
		return out
	case []any:
		out := make([]any, len(node))
		for i, e := range node {
			out[i] = cutStrings(e, path+"["+strconv.Itoa(i)+"]", keep, fields)
		}
		return out
	}
	return v
}

// truncationMarkerRoom is at least the length of a truncationMarker: a value
// is cut only when it is longer than what it would be cut to.
const truncationMarkerRoom = 80

// truncationMarker ends a cut text value.
func truncationMarker(original, kept int) string {
	return fmt.Sprintf(" ... [truncated: first %d of %d characters]", kept, original)
}

// firstRunes is the first n characters of s.
func firstRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
