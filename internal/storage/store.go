package storage

import (
	"github.com/SolaTyolo/mcphub/internal/models"
)

type Store interface {
	CreateAgent(a *models.Agent) (*models.Agent, error)
	ListAgents() ([]*models.Agent, error)
	GetAgent(id string) (*models.Agent, error)
	UpdateAgent(id string, req *models.UpdateAgentRequest) (*models.Agent, error)
	DeleteAgent(id string) error
	ListEnabledMCPServersForAgent(agentID string) ([]*models.MCPServer, error)

	CreateMCPServer(s *models.MCPServer) (*models.MCPServer, error)
	ListMCPServers() ([]*models.MCPServer, error)
	GetMCPServer(serverID string) (*models.MCPServer, error)
	UpdateMCPServer(serverID string, s *models.MCPServer) (*models.MCPServer, error)
	DeleteMCPServer(serverID string) error
}
