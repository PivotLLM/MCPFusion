/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PivotLLM/MCPFusion/global"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertDownstreamTool(t *testing.T) {
	tool := mcp.NewTool("print_receipt",
		mcp.WithDescription("Prints a receipt"),
		mcp.WithString("text", mcp.Required(), mcp.Description("Receipt text")),
		mcp.WithNumber("copies", mcp.Description("Number of copies")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	)

	mockCallFunc := func(_ context.Context, _ string, _ map[string]any, _ *mcp.Meta) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	}

	td := ConvertDownstreamTool("miniprint", tool, mockCallFunc, nil)

	assert.Equal(t, "miniprint_print_receipt", td.Name)
	assert.Equal(t, "Prints a receipt", td.Description)

	// Build a lookup map from the parameters slice
	paramMap := make(map[string]struct {
		Required bool
		Type     string
	})
	for _, p := range td.Parameters {
		paramMap[p.Name] = struct {
			Required bool
			Type     string
		}{Required: p.Required, Type: p.Type}
	}

	textParam, ok := paramMap["text"]
	assert.True(t, ok, "expected parameter 'text' to exist")
	assert.True(t, textParam.Required, "expected 'text' to be required")
	assert.Equal(t, "string", textParam.Type)

	copiesParam, ok := paramMap["copies"]
	assert.True(t, ok, "expected parameter 'copies' to exist")
	assert.False(t, copiesParam.Required, "expected 'copies' to not be required")
	assert.Equal(t, "number", copiesParam.Type)

	assert.NotNil(t, td.Hints)
	assert.NotNil(t, td.Hints.ReadOnly)
	assert.False(t, *td.Hints.ReadOnly)
	assert.NotNil(t, td.Hints.Destructive)
	assert.False(t, *td.Hints.Destructive)
}

func TestConvertDownstreamTool_Handler(t *testing.T) {
	var receivedName string
	var receivedArgs map[string]any

	mockCallFunc := func(_ context.Context, toolName string, args map[string]any, _ *mcp.Meta) (*mcp.CallToolResult, error) {
		receivedName = toolName
		receivedArgs = args
		return mcp.NewToolResultText("done"), nil
	}

	tool := mcp.NewTool("send",
		mcp.WithDescription("Send something"),
		mcp.WithString("msg", mcp.Required(), mcp.Description("The message")),
	)

	td := ConvertDownstreamTool("svc", tool, mockCallFunc, nil)

	result, err := td.Handler(map[string]any{
		"__mcp_context": context.Background(),
		"msg":           "hello",
	})

	assert.NoError(t, err)
	assert.Equal(t, "done", result)
	assert.Equal(t, "send", receivedName, "handler should forward the original unprefixed tool name")
	assert.Equal(t, map[string]any{"msg": "hello"}, receivedArgs, "handler should strip __mcp_context from args")
}

func TestConvertDownstreamTool_NoHints(t *testing.T) {
	// Build a tool with all annotation hint pointers nil.
	// mcp.NewTool sets defaults, so we construct the Tool directly.
	tool := mcp.Tool{
		Name:        "simple",
		Description: "A simple tool",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: make(map[string]any),
		},
	}

	mockCallFunc := func(_ context.Context, _ string, _ map[string]any, _ *mcp.Meta) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	}

	td := ConvertDownstreamTool("svc", tool, mockCallFunc, nil)

	assert.Nil(t, td.Hints, "hints should be nil when no annotations are set")
}

func TestFormatCallToolResult(t *testing.T) {
	t.Run("simple text result", func(t *testing.T) {
		result := mcp.NewToolResultText("hello")
		output := FormatCallToolResult(result, nil)
		assert.Equal(t, "hello", output)
	})

	t.Run("error result", func(t *testing.T) {
		result := mcp.NewToolResultError("something went wrong")
		output := FormatCallToolResult(result, nil)
		assert.Equal(t, "Error: something went wrong", output)
	})

	t.Run("nil result", func(t *testing.T) {
		output := FormatCallToolResult(nil, nil)
		assert.Equal(t, "", output)
	})
}

func TestDiffTools(t *testing.T) {
	makeTool := func(name string) mcp.Tool {
		return mcp.NewTool(name, mcp.WithDescription(name+" tool"))
	}

	t.Run("empty old two new", func(t *testing.T) {
		oldTools := map[string]mcp.Tool{}
		newTools := map[string]mcp.Tool{
			"alpha": makeTool("alpha"),
			"beta":  makeTool("beta"),
		}

		diff := DiffTools(oldTools, newTools)
		assert.Equal(t, []string{"alpha", "beta"}, diff.Added)
		assert.Empty(t, diff.Removed)
	})

	t.Run("two old empty new", func(t *testing.T) {
		oldTools := map[string]mcp.Tool{
			"alpha": makeTool("alpha"),
			"beta":  makeTool("beta"),
		}
		newTools := map[string]mcp.Tool{}

		diff := DiffTools(oldTools, newTools)
		assert.Empty(t, diff.Added)
		assert.Equal(t, []string{"alpha", "beta"}, diff.Removed)
	})

	t.Run("same set no changes", func(t *testing.T) {
		tools := map[string]mcp.Tool{
			"alpha": makeTool("alpha"),
			"beta":  makeTool("beta"),
		}

		diff := DiffTools(tools, tools)
		assert.Empty(t, diff.Added)
		assert.Empty(t, diff.Removed)
	})

	t.Run("one added one removed one unchanged", func(t *testing.T) {
		oldTools := map[string]mcp.Tool{
			"keep":   makeTool("keep"),
			"remove": makeTool("remove"),
		}
		newTools := map[string]mcp.Tool{
			"keep": makeTool("keep"),
			"add":  makeTool("add"),
		}

		diff := DiffTools(oldTools, newTools)
		assert.Equal(t, []string{"add"}, diff.Added)
		assert.Equal(t, []string{"remove"}, diff.Removed)
	})
}

func TestWithFileModes(t *testing.T) {
	tests := []struct {
		name     string
		opts     []HubOption
		wantDir  fs.FileMode
		wantFile fs.FileMode
	}{
		{"defaults", nil, global.DefaultDirMode, global.DefaultFileMode},
		{"both set", []HubOption{WithFileModes(0700, 0600)}, 0700, 0600},
		{"zero dir keeps default", []HubOption{WithFileModes(0, 0600)}, global.DefaultDirMode, 0600},
		{"zero file keeps default", []HubOption{WithFileModes(0700, 0)}, 0700, global.DefaultFileMode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := append([]HubOption{WithDownloadDir("/tmp/x")}, tt.opts...)
			h := NewHubProvider(nil, opts...)
			got := h.makeGetOpts()(context.Background())
			assert.Equal(t, tt.wantDir, got.DirMode)
			assert.Equal(t, tt.wantFile, got.FileMode)
		})
	}
}

// TestFormatCallToolResult_ImageFileModes verifies saved images use the modes
// in FormatOptions, and that zero modes fall back to the defaults.
// 0700/0600 are unaffected by any usual umask.
func TestFormatCallToolResult_ImageFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions not applicable on Windows")
	}
	img := mcp.NewImageContent(base64.StdEncoding.EncodeToString([]byte("png")), "image/png")
	result := &mcp.CallToolResult{Content: []mcp.Content{img}}

	dlDir := filepath.Join(t.TempDir(), "dl")
	out := FormatCallToolResult(result, &FormatOptions{DownloadDir: dlDir, TenantHash: "t1", DirMode: 0700, FileMode: 0600})
	assert.Contains(t, out, "Image saved:")

	saveDir := filepath.Join(dlDir, "t1")
	di, err := os.Stat(saveDir)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0700), di.Mode().Perm())

	entries, err := os.ReadDir(saveDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	fi, err := entries[0].Info()
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0600), fi.Mode().Perm())

	// Zero modes must not create unusable (mode 0) directories or files.
	dlDir2 := filepath.Join(t.TempDir(), "dl")
	out = FormatCallToolResult(result, &FormatOptions{DownloadDir: dlDir2})
	assert.Contains(t, out, "Image saved:")
	entries, err = os.ReadDir(dlDir2)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	fi, err = entries[0].Info()
	require.NoError(t, err)
	assert.NotZero(t, fi.Mode().Perm()&0600, "saved file should be owner-readable/writable")
}
