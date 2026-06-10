package models

import (
	"encoding/json"
	"fmt"
)

type ContentPart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *ImageURLPart  `json:"image_url,omitempty"`
}

type ImageURLPart struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type ChatMessage struct {
	Role       string        `json:"role"`
	Content    any           `json:"content,omitempty"`
	ToolCalls  []ToolCall    `json:"toolCalls,omitempty"`
	ToolCallID string        `json:"toolCallId,omitempty"`
}

func (m ChatMessage) TextContent() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []ContentPart:
		for _, p := range v {
			if p.Type == "text" {
				return p.Text
			}
		}
	case []any:
		for _, item := range v {
			if mp, ok := item.(map[string]any); ok && mp["type"] == "text" {
				if t, ok := mp["text"].(string); ok {
					return t
				}
			}
		}
	}
	return ""
}

func (m ChatMessage) HasImage() bool {
	switch v := m.Content.(type) {
	case []ContentPart:
		for _, p := range v {
			if p.Type == "image_url" && p.ImageURL != nil && p.ImageURL.URL != "" {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if mp, ok := item.(map[string]any); ok && mp["type"] == "image_url" {
				return true
			}
		}
	}
	return false
}

func HasImages(msgs []ChatMessage) bool {
	for _, m := range msgs {
		if m.HasImage() {
			return true
		}
	}
	return false
}

func NewTextMessage(role, text string) ChatMessage {
	return ChatMessage{Role: role, Content: text}
}

func NewMultimodalMessage(role, text, imageDataURL string) ChatMessage {
	parts := []ContentPart{}
	if text != "" {
		parts = append(parts, ContentPart{Type: "text", Text: text})
	}
	if imageDataURL != "" {
		parts = append(parts, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURLPart{URL: imageDataURL},
		})
	}
	return ChatMessage{Role: role, Content: parts}
}

func (m *ChatMessage) UnmarshalJSON(data []byte) error {
	type raw struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  []ToolCall      `json:"toolCalls"`
		ToolCallID string          `json:"toolCallId"`
	}
	var aux raw
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.Role = aux.Role
	m.ToolCalls = aux.ToolCalls
	m.ToolCallID = aux.ToolCallID
	if len(aux.Content) == 0 {
		return nil
	}
	if aux.Content[0] == '"' {
		var s string
		if err := json.Unmarshal(aux.Content, &s); err != nil {
			return err
		}
		m.Content = s
		return nil
	}
	var parts []ContentPart
	if err := json.Unmarshal(aux.Content, &parts); err != nil {
		return fmt.Errorf("content must be string or content part array: %w", err)
	}
	m.Content = parts
	return nil
}

func (m ChatMessage) MarshalJSON() ([]byte, error) {
	type out struct {
		Role       string     `json:"role"`
		Content    any        `json:"content,omitempty"`
		ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
		ToolCallID string     `json:"toolCallId,omitempty"`
	}
	return json.Marshal(out{
		Role:       m.Role,
		Content:    m.Content,
		ToolCalls:  m.ToolCalls,
		ToolCallID: m.ToolCallID,
	})
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
	Result    string `json:"result,omitempty"`
}

type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
	Model    string        `json:"model,omitempty"`
}

type ChatResponse struct {
	Message       ChatMessage `json:"message"`
	ToolCalls     []ToolCall  `json:"toolCalls,omitempty"`
	Transcription string      `json:"transcription,omitempty"`
	Endpoint      string      `json:"endpoint"`
	Model         string      `json:"model"`
}
