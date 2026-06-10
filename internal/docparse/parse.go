package docparse

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/SolaTyolo/mcphub/internal/markitdown"
)

const MaxDocumentBytes = 32 << 20 // 32 MiB

var supportedExts = map[string]struct{}{
	".pdf": {}, ".docx": {}, ".pptx": {}, ".xlsx": {}, ".xls": {},
	".csv": {}, ".json": {}, ".xml": {}, ".html": {}, ".htm": {},
	".epub": {}, ".ipynb": {}, ".msg": {}, ".zip": {},
	".txt": {}, ".md": {}, ".rtf": {},
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".webp": {}, ".bmp": {}, ".tiff": {}, ".tif": {},
	".wav": {}, ".mp3": {}, ".m4a": {},
}

// Supported reports whether filename is supported by MarkItDown.
func Supported(filename string) bool {
	_, ok := supportedExts[strings.ToLower(filepath.Ext(filename))]
	return ok
}

// SupportedExts returns human-readable supported inputs.
func SupportedExts() string {
	return "PDF, Office (DOCX/XLSX/PPTX/XLS), CSV, JSON, XML, HTML, EPUB, IPYNB, MSG, ZIP, images, audio, text"
}

// ToMarkdown reads a document and returns Markdown text for LLM input.
func ToMarkdown(ctx context.Context, client *markitdown.Client, r io.Reader, filename string) (string, error) {
	if client == nil || !client.Enabled() {
		return "", fmt.Errorf("document parsing not configured (set MARKITDOWN_MCP_URL)")
	}
	if !Supported(filename) {
		ext := strings.ToLower(filepath.Ext(filename))
		return "", fmt.Errorf("unsupported document type %q (supported: %s)", ext, SupportedExts())
	}

	data, err := readLimited(r, MaxDocumentBytes)
	if err != nil {
		return "", err
	}

	md, err := client.Convert(ctx, data, filename)
	if err != nil {
		return "", fmt.Errorf("parse document: %w", err)
	}
	return md, nil
}

// FormatUserMessage wraps extracted document text with an optional user prompt.
func FormatUserMessage(prompt, filename, body string) string {
	var b strings.Builder
	if p := strings.TrimSpace(prompt); p != "" {
		b.WriteString(p)
		b.WriteString("\n\n")
	}
	b.WriteString("--- document: ")
	b.WriteString(filepath.Base(filename))
	b.WriteString(" ---\n")
	b.WriteString(body)
	return b.String()
}

func readLimited(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return nil, fmt.Errorf("invalid max size")
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("document exceeds %d MiB limit", max>>20)
	}
	return data, nil
}
