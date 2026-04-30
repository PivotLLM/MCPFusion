/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to build a ParameterConfig with a given name, location, and optional transform/fileNameParam
func makeParam(name string, location ParameterLocation, targetName string, fileNameParam string) ParameterConfig {
	p := ParameterConfig{
		Name:          name,
		Location:      location,
		FileNameParam: fileNameParam,
	}
	if targetName != "" {
		p.Transform = &TransformConfig{TargetName: targetName}
	}
	return p
}

// ---- hasFileParams tests ----

func TestHasFileParams_NoFileParams(t *testing.T) {
	params := []ParameterConfig{
		makeParam("cardId", ParameterLocationPath, "", ""),
		makeParam("url", ParameterLocationQuery, "", ""),
	}
	args := map[string]interface{}{
		"cardId": "abc123",
		"url":    "https://example.com",
	}
	if hasFileParams(params, args) {
		t.Error("expected false when no file-location params are defined")
	}
}

func TestHasFileParams_FileParamNotInArgs(t *testing.T) {
	params := []ParameterConfig{
		makeParam("cardId", ParameterLocationPath, "", ""),
		makeParam("file_content", ParameterLocationFile, "file", "file_name"),
	}
	args := map[string]interface{}{
		"cardId": "abc123",
		// file_content not provided
	}
	if hasFileParams(params, args) {
		t.Error("expected false when file-location param is defined but not present in args")
	}
}

func TestHasFileParams_FileParamInArgs(t *testing.T) {
	params := []ParameterConfig{
		makeParam("cardId", ParameterLocationPath, "", ""),
		makeParam("file_content", ParameterLocationFile, "file", "file_name"),
	}
	args := map[string]interface{}{
		"cardId":       "abc123",
		"file_content": "hello world",
	}
	if !hasFileParams(params, args) {
		t.Error("expected true when file-location param is present in args")
	}
}

// ---- buildMultipartBody tests ----

// parseMultipart is a test helper that reads all parts from the generated body.
// Returns a map of partName -> {filename, content} and the raw content-type.
type partInfo struct {
	filename string
	content  string
	header   textproto.MIMEHeader
}

func parseMultipartBody(t *testing.T, body string, contentType string) map[string]partInfo {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse content-type %q: %v", contentType, err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("expected multipart content-type, got %q", mediaType)
	}

	mr := multipart.NewReader(strings.NewReader(body), params["boundary"])
	parts := make(map[string]partInfo)

	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, readErr := part.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}

		name := part.FormName()
		filename := part.FileName()
		parts[name] = partInfo{
			filename: filename,
			content:  sb.String(),
			header:   part.Header,
		}
		part.Close()
	}

	return parts
}

func TestBuildMultipartBody_FileFieldNameAndContent(t *testing.T) {
	params := []ParameterConfig{
		makeParam("file_content", ParameterLocationFile, "file", ""),
	}
	args := map[string]interface{}{
		"file_content": "hello world",
	}

	reader, ct, err := buildMultipartBody(params, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	parts := parseMultipartBody(t, sb.String(), ct)

	// field name should be resolved via transform.TargetName = "file"
	info, ok := parts["file"]
	if !ok {
		t.Fatalf("expected part named 'file', got parts: %v", parts)
	}
	if info.content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", info.content)
	}
}

func TestBuildMultipartBody_FileNameParamUsed(t *testing.T) {
	params := []ParameterConfig{
		makeParam("file_content", ParameterLocationFile, "file", "file_name"),
	}
	args := map[string]interface{}{
		"file_content": "data",
		"file_name":    "report.md",
	}

	reader, ct, err := buildMultipartBody(params, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	parts := parseMultipartBody(t, sb.String(), ct)
	info, ok := parts["file"]
	if !ok {
		t.Fatalf("expected part named 'file', got parts: %v", parts)
	}
	if info.filename != "report.md" {
		t.Errorf("expected filename %q, got %q", "report.md", info.filename)
	}
}

func TestBuildMultipartBody_FallbackFileName(t *testing.T) {
	// fileNameParam is set but not present in args — should fall back to "attachment.txt"
	params := []ParameterConfig{
		makeParam("file_content", ParameterLocationFile, "file", "file_name"),
	}
	args := map[string]interface{}{
		"file_content": "data",
		// file_name intentionally absent
	}

	reader, ct, err := buildMultipartBody(params, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	parts := parseMultipartBody(t, sb.String(), ct)
	info, ok := parts["file"]
	if !ok {
		t.Fatalf("expected part named 'file', got parts: %v", parts)
	}
	if info.filename != "attachment.txt" {
		t.Errorf("expected fallback filename %q, got %q", "attachment.txt", info.filename)
	}
}

func TestBuildMultipartBody_FallbackFileName_NoFileNameParam(t *testing.T) {
	// fileNameParam is empty — should fall back to "attachment.txt"
	params := []ParameterConfig{
		makeParam("file_content", ParameterLocationFile, "file", ""),
	}
	args := map[string]interface{}{
		"file_content": "data",
	}

	reader, ct, err := buildMultipartBody(params, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	parts := parseMultipartBody(t, sb.String(), ct)
	info, ok := parts["file"]
	if !ok {
		t.Fatalf("expected part named 'file', got parts: %v", parts)
	}
	if info.filename != "attachment.txt" {
		t.Errorf("expected fallback filename %q, got %q", "attachment.txt", info.filename)
	}
}

func TestBuildMultipartBody_BodyParamsAsFormFields(t *testing.T) {
	params := []ParameterConfig{
		makeParam("file_content", ParameterLocationFile, "file", ""),
		makeParam("description", ParameterLocationBody, "", ""),
		makeParam("tag", ParameterLocationBody, "label", ""),
	}
	args := map[string]interface{}{
		"file_content": "content here",
		"description":  "my description",
		"tag":          "urgent",
	}

	reader, ct, err := buildMultipartBody(params, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	parts := parseMultipartBody(t, sb.String(), ct)

	if _, ok := parts["file"]; !ok {
		t.Error("expected file part")
	}

	desc, ok := parts["description"]
	if !ok {
		t.Fatal("expected form field 'description'")
	}
	if desc.content != "my description" {
		t.Errorf("expected description %q, got %q", "my description", desc.content)
	}

	// tag has transform targetName = "label"
	label, ok := parts["label"]
	if !ok {
		t.Fatal("expected form field 'label' (transformed from 'tag')")
	}
	if label.content != "urgent" {
		t.Errorf("expected label content %q, got %q", "urgent", label.content)
	}
}

func TestBuildMultipartBody_FilePath(t *testing.T) {
	// Write a temp file with known content
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "document.docx")
	content := []byte{0x50, 0x4B, 0x03, 0x04} // ZIP magic bytes (DOCX is a ZIP)
	if err := os.WriteFile(tmpFile, content, 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	params := []ParameterConfig{
		makeParam("file_path", ParameterLocationFilePath, "file", ""),
	}
	args := map[string]interface{}{
		"file_path": tmpFile,
	}

	reader, ct, err := buildMultipartBody(params, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	parts := parseMultipartBody(t, sb.String(), ct)

	info, ok := parts["file"]
	if !ok {
		t.Fatalf("expected part named 'file', got parts: %v", parts)
	}
	// Filename should be derived from the path basename
	if info.filename != "document.docx" {
		t.Errorf("expected filename %q, got %q", "document.docx", info.filename)
	}
	// Content should match the raw bytes
	if info.content != string(content) {
		t.Errorf("file content mismatch")
	}
}

func TestBuildMultipartBody_FilePath_NameOverride(t *testing.T) {
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "temp.bin")
	if err := os.WriteFile(tmpFile, []byte("data"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	params := []ParameterConfig{
		makeParam("file_path", ParameterLocationFilePath, "file", "file_name"),
	}
	args := map[string]interface{}{
		"file_path": tmpFile,
		"file_name": "report.xlsx",
	}

	reader, ct, err := buildMultipartBody(params, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	parts := parseMultipartBody(t, sb.String(), ct)
	info, ok := parts["file"]
	if !ok {
		t.Fatalf("expected part named 'file'")
	}
	if info.filename != "report.xlsx" {
		t.Errorf("expected overridden filename %q, got %q", "report.xlsx", info.filename)
	}
}
