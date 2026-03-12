/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_filenameFromContentDisposition(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "empty header",
			header: "",
			want:   "",
		},
		{
			name:   "malformed header",
			header: "not-valid-mime-type!!!",
			want:   "",
		},
		{
			name:   "quoted filename",
			header: `attachment; filename="report.docx"`,
			want:   "report.docx",
		},
		{
			name:   "unquoted filename",
			header: "attachment; filename=report.docx",
			want:   "report.docx",
		},
		{
			name:   "path traversal with slash",
			header: `attachment; filename="../etc/passwd"`,
			// filepath.Base strips the path component, leaving just the filename
			want: "passwd",
		},
		{
			name:   "double-dot alone",
			header: "attachment; filename=..",
			want:   "",
		},
		{
			name:   "single-dot alone",
			header: "attachment; filename=.",
			want:   "",
		},
		{
			name:   "filename with backslash",
			header: `attachment; filename=".\\secret"`,
			// mime.ParseMediaType unescapes the quoted string, yielding ".\secret"
			// (one backslash). On Linux, backslash is not a path separator, so
			// filepath.Base returns the full value and it is not rejected by the guard.
			want: ".\\secret",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := filenameFromContentDisposition(tc.header)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_sanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "clean alphanumeric string",
			input: "report2024",
			want:  "report2024",
		},
		{
			name:  "string with slash",
			input: "path/to/file",
			want:  "path_to_file",
		},
		{
			name:  "string with colon",
			input: "service:endpoint",
			want:  "service_endpoint",
		},
		{
			name:  "string with spaces",
			input: "my file name",
			want:  "my_file_name",
		},
		{
			name:  "string with multiple unsafe characters",
			input: `a/b\c:d*e?f"g<h>i|j k`,
			want:  "a_b_c_d_e_f_g_h_i_j_k",
		},
		{
			name:  "dot is preserved",
			input: ".docx",
			want:  ".docx",
		},
		{
			name:  "alphanumeric with dots and dashes",
			input: "my-report.v1.docx",
			want:  "my-report.v1.docx",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeFilename(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
