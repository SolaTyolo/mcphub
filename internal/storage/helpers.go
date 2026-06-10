package storage

import (
	"fmt"
	"strings"

	"github.com/SolaTyolo/mcphub/internal/models"
)

func mergeAgentUpdate(existing *models.Agent, req *models.UpdateAgentRequest) (*models.Agent, error) {
	out := *existing
	if req.Name != nil {
		out.Name = strings.TrimSpace(*req.Name)
		if out.Name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
	}
	if req.SystemPrompt != nil {
		out.SystemPrompt = *req.SystemPrompt
	}
	if req.ResponseDescription != nil {
		out.ResponseDescription = *req.ResponseDescription
	}
	if req.Enabled != nil {
		out.Enabled = *req.Enabled
	}
	if req.ResponseSchema != nil {
		out.ResponseSchema = req.ResponseSchema
	}
	if req.MCPServerIDs != nil {
		out.MCPServerIDs = req.MCPServerIDs
	}
	if req.LLMBaseURL != nil {
		out.LLMBaseURL = strings.TrimSpace(*req.LLMBaseURL)
	}
	if req.LLMAPIKey != nil {
		out.LLMAPIKey = strings.TrimSpace(*req.LLMAPIKey)
	}
	if req.LLMModel != nil {
		out.LLMModel = strings.TrimSpace(*req.LLMModel)
	}
	if req.VisionModel != nil {
		out.VisionModel = strings.TrimSpace(*req.VisionModel)
	}
	return &out, nil
}

func agentByName(list []*models.Agent, name string) *models.Agent {
	for _, a := range list {
		if a.Name == name {
			return a
		}
	}
	return nil
}

func mcpByName(list []*models.MCPServer, name string) *models.MCPServer {
	for _, s := range list {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func agentByID(list []*models.Agent, id string) *models.Agent {
	for _, a := range list {
		if a.ID == id {
			return a
		}
	}
	return nil
}

func mcpByID(list []*models.MCPServer, id string) *models.MCPServer {
	for _, s := range list {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func duplicateNameErr(kind, name string) error {
	return fmt.Errorf("%s name %q already exists", kind, name)
}
