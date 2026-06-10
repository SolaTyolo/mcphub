package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/SolaTyolo/mcphub/internal/attachment"
	"github.com/SolaTyolo/mcphub/internal/models"
)

func (s *Server) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 32 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("file field required"))
		return
	}
	defer file.Close()

	att, err := s.storeAttachment(r, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, att)
}

func (s *Server) storeAttachment(r *http.Request, filename, contentType string, file io.Reader) (models.Attachment, error) {
	data, err := readLimitedBytes(file, s.cfg.AttachmentMaxBytes)
	if err != nil {
		return models.Attachment{}, err
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	meta, err := s.attachments.Put(r.Context(), filename, contentType, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return models.Attachment{}, err
	}
	return attachmentToModel(meta), nil
}

func (s *Server) resolveAttachmentIDs(r *http.Request, ids []string) ([]models.Attachment, error) {
	out := make([]models.Attachment, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		meta, err := s.attachments.Head(r.Context(), id)
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", id, err)
		}
		out = append(out, attachmentToModel(meta))
	}
	return out, nil
}

func mergeAttachments(existing []models.Attachment, extra ...models.Attachment) []models.Attachment {
	seen := make(map[string]struct{}, len(existing)+len(extra))
	out := make([]models.Attachment, 0, len(existing)+len(extra))
	for _, a := range existing {
		if _, ok := seen[a.ID]; ok {
			continue
		}
		seen[a.ID] = struct{}{}
		out = append(out, a)
	}
	for _, a := range extra {
		if _, ok := seen[a.ID]; ok {
			continue
		}
		seen[a.ID] = struct{}{}
		out = append(out, a)
	}
	return out
}

func attachmentToModel(meta *attachment.Meta) models.Attachment {
	return models.Attachment{
		ID:          meta.ID,
		Filename:    meta.Filename,
		ContentType: meta.ContentType,
		Size:        meta.Size,
	}
}

func readLimitedBytes(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return nil, fmt.Errorf("invalid max size")
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("attachment exceeds %d MiB limit", max>>20)
	}
	return data, nil
}

func decodeAttachmentIDs(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func formatAttachmentUserMessage(prompt string, attachments []models.Attachment) string {
	var b strings.Builder
	if p := strings.TrimSpace(prompt); p != "" {
		b.WriteString(p)
		b.WriteString("\n\n")
	}
	b.WriteString("Uploaded attachments (use mcphub__parse_document when needed):\n")
	for _, a := range attachments {
		fmt.Fprintf(&b, "- id=%s name=%q\n", a.ID, a.Filename)
	}
	return b.String()
}
