package llm

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/SolaTyolo/mcphub/internal/config"
	"github.com/SolaTyolo/mcphub/internal/mcp"
	"github.com/SolaTyolo/mcphub/internal/models"
)

type ChatOptions struct {
	ResponseFormat *openai.ChatCompletionResponseFormat
}

type Provider interface {
	Chat(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool, opts ChatOptions) (openai.ChatCompletionResponse, error)
	Endpoint() string
	Model() string
}

type Router struct {
	cfg config.Config
}

func NewRouter(cfg config.Config) *Router {
	return &Router{cfg: cfg}
}

func (r *Router) Default() Provider {
	return r.newProvider(r.cfg.LLMBaseURL, r.cfg.LLMAPIKey, r.cfg.LLMModel)
}

func (r *Router) Resolve(ag *models.Agent, req models.ChatRequest) Provider {
	base, key, textModel, visionModel := ag.EffectiveLLM(r.cfg)
	model := textModel
	if req.Model != "" {
		model = req.Model
	} else if models.HasImages(req.Messages) {
		model = visionModel
	}
	return r.newProvider(base, key, model)
}

func (r *Router) newProvider(baseURL, apiKey, model string) Provider {
	return &openAIProvider{
		endpoint: baseURL,
		model:    model,
		client: openai.NewClientWithConfig(func() openai.ClientConfig {
			cfg := openai.DefaultConfig(apiKey)
			cfg.BaseURL = baseURL
			return cfg
		}()),
	}
}

type openAIProvider struct {
	endpoint string
	model    string
	client   *openai.Client
}

func (p *openAIProvider) Endpoint() string { return p.endpoint }
func (p *openAIProvider) Model() string    { return p.model }

func (p *openAIProvider) Chat(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool, opts ChatOptions) (openai.ChatCompletionResponse, error) {
	req := openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: messages,
	}
	if len(tools) > 0 {
		req.Tools = tools
	}
	if opts.ResponseFormat != nil {
		req.ResponseFormat = opts.ResponseFormat
	}
	return p.client.CreateChatCompletion(ctx, req)
}

func JSONSchemaFormat(schema map[string]any) *openai.ChatCompletionResponseFormat {
	raw, _ := json.Marshal(schema)
	return &openai.ChatCompletionResponseFormat{
		Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
		JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
			Name:   "response",
			Schema: json.RawMessage(raw),
			Strict: true,
		},
	}
}

func ToolsFromMCP(defs []mcp.ToolDef) []openai.Tool {
	var tools []openai.Tool
	for _, d := range defs {
		params := d.InputSchema
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  params,
			},
		})
	}
	return tools
}

func ParseToolArguments(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return map[string]any{"input": raw}
	}
	return args
}

func AssistantContent(msg *openai.ChatCompletionMessage) string {
	if msg.Content != "" {
		return msg.Content
	}
	return ""
}

func ToChatMessages(msgs []models.ChatMessage) ([]openai.ChatCompletionMessage, error) {
	var out []openai.ChatCompletionMessage
	for _, m := range msgs {
		switch m.Role {
		case "user", "assistant", "system":
			oai, err := messageToOAI(m.Role, m.Content)
			if err != nil {
				return nil, err
			}
			out = append(out, oai)
		case "tool":
			out = append(out, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    m.TextContent(),
				ToolCallID: m.ToolCallID,
			})
		default:
			return nil, fmt.Errorf("unsupported role: %s", m.Role)
		}
	}
	return out, nil
}

func messageToOAI(role string, content any) (openai.ChatCompletionMessage, error) {
	msg := openai.ChatCompletionMessage{Role: role}
	switch v := content.(type) {
	case string:
		msg.Content = v
	case []models.ContentPart:
		parts, err := partsToOAI(v)
		if err != nil {
			return msg, err
		}
		msg.MultiContent = parts
	case nil:
	default:
		return msg, fmt.Errorf("unsupported message content type %T", content)
	}
	return msg, nil
}

func partsToOAI(parts []models.ContentPart) ([]openai.ChatMessagePart, error) {
	var out []openai.ChatMessagePart
	for _, p := range parts {
		switch p.Type {
		case "text":
			out = append(out, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: p.Text,
			})
		case "image_url":
			if p.ImageURL == nil || p.ImageURL.URL == "" {
				return nil, fmt.Errorf("image_url part missing url")
			}
			out = append(out, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    p.ImageURL.URL,
					Detail: openai.ImageURLDetail(p.ImageURL.Detail),
				},
			})
		default:
			return nil, fmt.Errorf("unsupported content part type: %s", p.Type)
		}
	}
	return out, nil
}

func BuildAgentMessages(ag *models.Agent, clientMsgs []models.ChatMessage) ([]openai.ChatCompletionMessage, error) {
	var systemParts []string
	if ag.SystemPrompt != "" {
		systemParts = append(systemParts, ag.SystemPrompt)
	}
	if ag.ResponseDescription != "" {
		systemParts = append(systemParts, "Respond in this format:\n"+ag.ResponseDescription)
	}
	if len(systemParts) > 0 {
		clientMsgs = append([]models.ChatMessage{
			models.NewTextMessage("system", joinParts(systemParts)),
		}, clientMsgs...)
	}
	return ToChatMessages(clientMsgs)
}

func joinParts(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += "\n\n" + p
	}
	return out
}
