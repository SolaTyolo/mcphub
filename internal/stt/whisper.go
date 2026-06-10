package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/SolaTyolo/mcphub/internal/config"
)

type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewClient(cfg config.Config) *Client {
	if strings.TrimSpace(cfg.WhisperBaseURL) == "" {
		return nil
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.WhisperBaseURL, "/"),
		apiKey:  cfg.WhisperAPIKey,
		model:   cfg.WhisperModel,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c != nil
}

type TranscribeOptions struct {
	Language string
	Filename string
}

func (c *Client) Transcribe(ctx context.Context, audio io.Reader, opts TranscribeOptions) (string, error) {
	if c == nil {
		return "", fmt.Errorf("speech-to-text not configured (set WHISPER_BASE_URL)")
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	if err := w.WriteField("model", c.model); err != nil {
		return "", err
	}
	if opts.Language != "" {
		if err := w.WriteField("language", opts.Language); err != nil {
			return "", err
		}
	}
	filename := opts.Filename
	if filename == "" {
		filename = "audio.wav"
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, audio); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("whisper api status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Text), nil
}
