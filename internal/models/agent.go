package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/SolaTyolo/mcphub/internal/config"
)

type Agent struct {
	ID                  string         `json:"id" yaml:"id"`
	Name                string         `json:"name" yaml:"name"`
	SystemPrompt        string         `json:"systemPrompt" yaml:"systemPrompt"`
	ResponseSchema      map[string]any `json:"responseSchema,omitempty" yaml:"responseSchema,omitempty"`
	ResponseDescription string         `json:"responseDescription,omitempty" yaml:"responseDescription,omitempty"`
	LLMBaseURL          string         `json:"llmBaseUrl,omitempty" yaml:"llmBaseUrl,omitempty"`
	LLMAPIKey           string         `json:"-" yaml:"llmApiKey,omitempty"`
	LLMModel            string         `json:"llmModel,omitempty" yaml:"llmModel,omitempty"`
	VisionModel         string         `json:"visionModel,omitempty" yaml:"visionModel,omitempty"`
	MCPServerIDs        []string       `json:"mcpServerIds,omitempty" yaml:"mcpServerIds,omitempty"`
	Enabled             bool           `json:"enabled" yaml:"enabled"`
	CreatedAt           time.Time      `json:"createdAt" yaml:"createdAt"`
}

type AgentLLMConfig struct {
	LLMBaseURL  string `json:"llmBaseUrl,omitempty"`
	LLMAPIKey   string `json:"llmApiKey,omitempty"`
	LLMModel    string `json:"llmModel,omitempty"`
	VisionModel string `json:"visionModel,omitempty"`
}

type CreateAgentRequest struct {
	Name                string         `json:"name"`
	SystemPrompt        string         `json:"systemPrompt,omitempty"`
	ResponseSchema      map[string]any `json:"responseSchema,omitempty"`
	ResponseDescription string         `json:"responseDescription,omitempty"`
	AgentLLMConfig
	MCPServerIDs []string `json:"mcpServerIds,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
}

type UpdateAgentRequest struct {
	Name                *string        `json:"name,omitempty"`
	SystemPrompt        *string        `json:"systemPrompt,omitempty"`
	ResponseSchema      map[string]any `json:"responseSchema,omitempty"`
	ResponseDescription *string        `json:"responseDescription,omitempty"`
	LLMBaseURL          *string        `json:"llmBaseUrl,omitempty"`
	LLMAPIKey           *string        `json:"llmApiKey,omitempty"`
	LLMModel            *string        `json:"llmModel,omitempty"`
	VisionModel         *string        `json:"visionModel,omitempty"`
	MCPServerIDs        []string       `json:"mcpServerIds,omitempty"`
	Enabled             *bool          `json:"enabled,omitempty"`
}

func (a *Agent) Validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("name is required")
	}
	a.Name = strings.TrimSpace(a.Name)
	return nil
}

func AgentFromRequest(req CreateAgentRequest) (*Agent, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	ag := &Agent{
		Name:                req.Name,
		SystemPrompt:        req.SystemPrompt,
		ResponseSchema:      req.ResponseSchema,
		ResponseDescription: req.ResponseDescription,
		LLMBaseURL:          req.LLMBaseURL,
		LLMAPIKey:           req.LLMAPIKey,
		LLMModel:            req.LLMModel,
		VisionModel:         req.VisionModel,
		MCPServerIDs:        req.MCPServerIDs,
		Enabled:             enabled,
	}
	if err := ag.Validate(); err != nil {
		return nil, err
	}
	return ag, nil
}

// EffectiveLLM resolves agent overrides with global defaults.
func (a *Agent) EffectiveLLM(cfg config.Config) (baseURL, apiKey, textModel, visionModel string) {
	baseURL = cfg.LLMBaseURL
	if a.LLMBaseURL != "" {
		baseURL = a.LLMBaseURL
	}
	apiKey = cfg.LLMAPIKey
	if a.LLMAPIKey != "" {
		apiKey = a.LLMAPIKey
	}
	textModel = cfg.LLMModel
	if a.LLMModel != "" {
		textModel = a.LLMModel
	}
	visionModel = cfg.LLMVisionModel
	if a.VisionModel != "" {
		visionModel = a.VisionModel
	}
	if visionModel == "" {
		visionModel = textModel
	}
	return baseURL, apiKey, textModel, visionModel
}
