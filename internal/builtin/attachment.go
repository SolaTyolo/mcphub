package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SolaTyolo/mcphub/internal/attachment"
	"github.com/SolaTyolo/mcphub/internal/docparse"
	"github.com/SolaTyolo/mcphub/internal/markitdown"
	"github.com/SolaTyolo/mcphub/internal/models"
)

const Prefix = "mcphub__"

type ToolDef struct {
	Name         string
	Description  string
	InputSchema  map[string]any
}

type Runner struct {
	store    attachment.Store
	markdown *markitdown.Client
}

func NewRunner(store attachment.Store, markdown *markitdown.Client) *Runner {
	return &Runner{store: store, markdown: markdown}
}

func (r *Runner) ToolDefs(session []models.Attachment) []ToolDef {
	if len(session) == 0 || r.markdown == nil || !r.markdown.Enabled() {
		return nil
	}
	return []ToolDef{
		{
			Name:        Prefix + "list_attachments",
			Description: "List files attached to the current chat session. Use before parse_document when attachment IDs are unknown.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        Prefix + "parse_document",
			Description: "Extract Markdown from an uploaded attachment by attachment_id via MarkItDown (Office, PDF, CSV, JSON, images, audio, and more).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"attachment_id": map[string]any{
						"type":        "string",
						"description": "Attachment ID from list_attachments or the chat attachment list",
					},
				},
				"required": []string{"attachment_id"},
			},
		},
	}
}

func IsBuiltin(name string) bool {
	return strings.HasPrefix(name, Prefix)
}

func (r *Runner) Call(ctx context.Context, name string, args map[string]any, session []models.Attachment) (string, error) {
	switch name {
	case Prefix + "list_attachments":
		return r.listAttachments(session)
	case Prefix + "parse_document":
		id, _ := args["attachment_id"].(string)
		return r.parseDocument(ctx, strings.TrimSpace(id), session)
	default:
		return "", fmt.Errorf("unknown builtin tool: %s", name)
	}
}

func (r *Runner) listAttachments(session []models.Attachment) (string, error) {
	if len(session) == 0 {
		return "No attachments in this chat.", nil
	}
	type row struct {
		ID          string `json:"id"`
		Filename    string `json:"filename"`
		ContentType string `json:"contentType,omitempty"`
		Size        int64  `json:"size,omitempty"`
		Parseable   bool   `json:"parseable"`
	}
	out := make([]row, 0, len(session))
	for _, a := range session {
		out = append(out, row{
			ID:          a.ID,
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        a.Size,
			Parseable:   r.markdown != nil && r.markdown.Enabled() && docparse.Supported(a.Filename),
		})
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *Runner) parseDocument(ctx context.Context, id string, session []models.Attachment) (string, error) {
	if id == "" {
		return "", fmt.Errorf("attachment_id is required")
	}
	if !sessionHas(session, id) {
		return "", fmt.Errorf("attachment %q is not in this chat session", id)
	}

	rc, meta, err := r.store.Get(ctx, id)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	md, err := docparse.ToMarkdown(ctx, r.markdown, rc, meta.Filename)
	if err != nil {
		return "", err
	}
	return docparse.FormatUserMessage("", meta.Filename, md), nil
}

func sessionHas(session []models.Attachment, id string) bool {
	for _, a := range session {
		if a.ID == id {
			return true
		}
	}
	return false
}

func AttachmentHint(session []models.Attachment, parsingEnabled bool) string {
	if len(session) == 0 {
		return ""
	}
	var b strings.Builder
	if parsingEnabled {
		b.WriteString("Attached files in this chat (use mcphub__list_attachments / mcphub__parse_document when needed):\n")
	} else {
		b.WriteString("Attached files in this chat (document parsing disabled; set MARKITDOWN_MCP_URL to enable):\n")
	}
	for _, a := range session {
		parseable := "no"
		if parsingEnabled && docparse.Supported(a.Filename) {
			parseable = "yes"
		}
		fmt.Fprintf(&b, "- id=%s name=%q size=%d parseable=%s\n", a.ID, a.Filename, a.Size, parseable)
	}
	return b.String()
}
