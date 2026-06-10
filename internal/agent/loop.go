package agent

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/SolaTyolo/mcphub/internal/config"
	"github.com/SolaTyolo/mcphub/internal/llm"
	"github.com/SolaTyolo/mcphub/internal/logx"
	"github.com/SolaTyolo/mcphub/internal/mcp"
	"github.com/SolaTyolo/mcphub/internal/models"
)

const tag = "agent"

type Service struct {
	cfg    config.Config
	router *llm.Router
	pool   *mcp.Pool
}

func NewService(cfg config.Config, router *llm.Router, pool *mcp.Pool) *Service {
	return &Service{cfg: cfg, router: router, pool: pool}
}

func (s *Service) Chat(ctx context.Context, ag *models.Agent, servers []*models.MCPServer, req models.ChatRequest) (*models.ChatResponse, error) {
	if err := s.pool.ConnectAll(ctx, servers); err != nil {
		logx.Error(tag, "mcp connect agent=%s: %v", ag.Name, err)
		return nil, err
	}

	toolDefs, err := s.pool.ListTools(ctx)
	if err != nil {
		logx.Error(tag, "list tools agent=%s: %v", ag.Name, err)
		return nil, err
	}

	provider := s.router.Resolve(ag, req)
	oaiTools := llm.ToolsFromMCP(toolDefs)

	messages, err := llm.BuildAgentMessages(ag, req.Messages)
	if err != nil {
		return nil, err
	}

	logx.Info(tag, "chat start agent=%s messages=%d mcp_servers=%d tools=%d endpoint=%s model=%s",
		ag.Name, len(req.Messages), len(servers), len(toolDefs), provider.Endpoint(), provider.Model())

	var executed []models.ToolCall

	for round := 0; round < s.cfg.AgentMaxRounds; round++ {
		logx.Info(tag, "llm request agent=%s round=%d/%d", ag.Name, round+1, s.cfg.AgentMaxRounds)
		resp, err := provider.Chat(ctx, messages, oaiTools, llm.ChatOptions{})
		if err != nil {
			logx.Error(tag, "llm chat agent=%s round=%d: %v", ag.Name, round+1, err)
			return nil, fmt.Errorf("llm chat: %w", err)
		}
		if len(resp.Choices) == 0 {
			logx.Error(tag, "empty llm response agent=%s round=%d", ag.Name, round+1)
			return nil, fmt.Errorf("empty llm response")
		}
		choice := resp.Choices[0].Message
		messages = append(messages, choice)

		if len(choice.ToolCalls) == 0 {
			content := llm.AssistantContent(&choice)
			if ag.ResponseSchema != nil {
				formatted, err := s.formatWithSchema(ctx, provider, messages, content, ag.ResponseSchema)
				if err != nil {
					logx.Warn(tag, "schema format failed agent=%s: %v", ag.Name, err)
				} else {
					content = formatted
				}
			}
			logx.Info(tag, "chat done agent=%s rounds=%d tool_calls=%d content_len=%d",
				ag.Name, round+1, len(executed), len(content))
			return &models.ChatResponse{
				Message: models.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				ToolCalls: executed,
				Endpoint:  provider.Endpoint(),
				Model:     provider.Model(),
			}, nil
		}

		logx.Info(tag, "tool calls agent=%s round=%d count=%d", ag.Name, round+1, len(choice.ToolCalls))
		for _, tc := range choice.ToolCalls {
			args := llm.ParseToolArguments(tc.Function.Arguments)
			logx.Info(tag, "call tool agent=%s name=%s args=%s",
				ag.Name, tc.Function.Name, logx.Truncate(tc.Function.Arguments, 200))
			result, err := s.pool.CallTool(ctx, tc.Function.Name, args)
			record := models.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Result:    result,
			}
			if err != nil {
				logx.Error(tag, "tool failed agent=%s name=%s: %v", ag.Name, tc.Function.Name, err)
				record.Result = err.Error()
			} else {
				logx.Info(tag, "tool ok agent=%s name=%s result=%s",
					ag.Name, tc.Function.Name, logx.Truncate(result, 200))
			}
			executed = append(executed, record)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    record.Result,
				ToolCallID: tc.ID,
			})
		}
	}

	logx.Error(tag, "max rounds exceeded agent=%s rounds=%d tool_calls=%d",
		ag.Name, s.cfg.AgentMaxRounds, len(executed))
	return nil, fmt.Errorf("agent exceeded max rounds (%d)", s.cfg.AgentMaxRounds)
}

func (s *Service) formatWithSchema(ctx context.Context, provider llm.Provider, messages []openai.ChatCompletionMessage, draft string, schema map[string]any) (string, error) {
	formatMsgs := append(messages,
		openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "Format your previous answer as JSON matching the required schema.",
		},
	)
	resp, err := provider.Chat(ctx, formatMsgs, nil, llm.ChatOptions{
		ResponseFormat: llm.JSONSchemaFormat(schema),
	})
	if err != nil {
		return draft, err
	}
	if len(resp.Choices) == 0 {
		return draft, fmt.Errorf("empty format response")
	}
	content := llm.AssistantContent(&resp.Choices[0].Message)
	if content == "" {
		return draft, fmt.Errorf("empty formatted content")
	}
	return content, nil
}

func (s *Service) TestServer(ctx context.Context, srv *models.MCPServer) ([]mcp.ToolDef, error) {
	logx.Info(tag, "test mcp server=%s transport=%s target=%s",
		srv.Name, srv.Transport, mcp.ConnectTarget(srv))
	if err := s.pool.ConnectAll(ctx, []*models.MCPServer{srv}); err != nil {
		logx.Error(tag, "test connect server=%s: %v", srv.Name, err)
		return nil, err
	}
	tools, err := s.pool.ListTools(ctx)
	if err != nil {
		logx.Error(tag, "test list tools server=%s: %v", srv.Name, err)
		return nil, err
	}
	logx.Info(tag, "test ok server=%s tools=%d", srv.Name, len(tools))
	return tools, nil
}
