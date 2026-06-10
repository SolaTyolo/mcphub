package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/SolaTyolo/mcphub/internal/models"
)

type fileSnapshot struct {
	Agents     []*models.Agent     `yaml:"agents"`
	MCPServers []*models.MCPServer `yaml:"mcpServers"`
}

type FileStore struct {
	path string
	mu   sync.Mutex
	snap fileSnapshot
}

func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("file store path is required")
	}
	s := &FileStore{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.snap = fileSnapshot{Agents: []*models.Agent{}, MCPServers: []*models.MCPServer{}}
			return s.persistLocked()
		}
		return err
	}
	if len(data) == 0 {
		s.snap = fileSnapshot{Agents: []*models.Agent{}, MCPServers: []*models.MCPServer{}}
		return nil
	}
	var snap fileSnapshot
	if err := yaml.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse %s: %w", s.path, err)
	}
	if snap.Agents == nil {
		snap.Agents = []*models.Agent{}
	}
	if snap.MCPServers == nil {
		snap.MCPServers = []*models.MCPServer{}
	}
	s.snap = snap
	return nil
}

func (s *FileStore) persistLocked() error {
	out, err := yaml.Marshal(s.snap)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *FileStore) CreateAgent(a *models.Agent) (*models.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if agentByName(s.snap.Agents, a.Name) != nil {
		return nil, duplicateNameErr("agent", a.Name)
	}
	now := time.Now().UTC()
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	a.CreatedAt = now
	copy := *a
	s.snap.Agents = append(s.snap.Agents, &copy)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneAgent(&copy), nil
}

func (s *FileStore) ListAgents() ([]*models.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneAgents(s.snap.Agents), nil
}

func (s *FileStore) GetAgent(id string) (*models.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ag := agentByID(s.snap.Agents, id)
	if ag == nil {
		return nil, ErrNotFound
	}
	return cloneAgent(ag), nil
}

func (s *FileStore) UpdateAgent(id string, req *models.UpdateAgentRequest) (*models.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ag := agentByID(s.snap.Agents, id)
	if ag == nil {
		return nil, ErrNotFound
	}
	merged, err := mergeAgentUpdate(ag, req)
	if err != nil {
		return nil, err
	}
	if merged.Name != ag.Name && agentByName(s.snap.Agents, merged.Name) != nil {
		return nil, duplicateNameErr("agent", merged.Name)
	}
	*ag = *merged
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneAgent(ag), nil
}

func (s *FileStore) DeleteAgent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, a := range s.snap.Agents {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	s.snap.Agents = append(s.snap.Agents[:idx], s.snap.Agents[idx+1:]...)
	return s.persistLocked()
}

func (s *FileStore) ListEnabledMCPServersForAgent(agentID string) ([]*models.MCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ag := agentByID(s.snap.Agents, agentID)
	if ag == nil {
		return nil, ErrNotFound
	}
	var out []*models.MCPServer
	for _, sid := range ag.MCPServerIDs {
		srv := mcpByID(s.snap.MCPServers, sid)
		if srv != nil && srv.Enabled {
			out = append(out, cloneMCPServer(srv))
		}
	}
	return out, nil
}

func (s *FileStore) CreateMCPServer(srv *models.MCPServer) (*models.MCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if mcpByName(s.snap.MCPServers, srv.Name) != nil {
		return nil, duplicateNameErr("mcp server", srv.Name)
	}
	now := time.Now().UTC()
	if srv.ID == "" {
		srv.ID = uuid.NewString()
	}
	srv.CreatedAt = now
	copy := *srv
	s.snap.MCPServers = append(s.snap.MCPServers, &copy)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneMCPServer(&copy), nil
}

func (s *FileStore) ListMCPServers() ([]*models.MCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneMCPServers(s.snap.MCPServers), nil
}

func (s *FileStore) GetMCPServer(serverID string) (*models.MCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv := mcpByID(s.snap.MCPServers, serverID)
	if srv == nil {
		return nil, ErrNotFound
	}
	return cloneMCPServer(srv), nil
}

func (s *FileStore) UpdateMCPServer(serverID string, srv *models.MCPServer) (*models.MCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := mcpByID(s.snap.MCPServers, serverID)
	if existing == nil {
		return nil, ErrNotFound
	}
	if srv.Name != existing.Name && mcpByName(s.snap.MCPServers, srv.Name) != nil {
		return nil, duplicateNameErr("mcp server", srv.Name)
	}
	existing.Name = srv.Name
	existing.Transport = srv.Transport
	existing.Command = srv.Command
	existing.URL = srv.URL
	existing.Args = srv.Args
	existing.Env = srv.Env
	existing.Enabled = srv.Enabled
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneMCPServer(existing), nil
}

func (s *FileStore) DeleteMCPServer(serverID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, srv := range s.snap.MCPServers {
		if srv.ID == serverID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	s.snap.MCPServers = append(s.snap.MCPServers[:idx], s.snap.MCPServers[idx+1:]...)
	for _, ag := range s.snap.Agents {
		ag.MCPServerIDs = removeID(ag.MCPServerIDs, serverID)
	}
	return s.persistLocked()
}

func removeID(ids []string, id string) []string {
	var out []string
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}

func cloneAgent(a *models.Agent) *models.Agent {
	copy := *a
	if a.ResponseSchema != nil {
		b, _ := yaml.Marshal(a.ResponseSchema)
		_ = yaml.Unmarshal(b, &copy.ResponseSchema)
	}
	if len(a.MCPServerIDs) > 0 {
		copy.MCPServerIDs = append([]string(nil), a.MCPServerIDs...)
	}
	return &copy
}

func cloneAgents(list []*models.Agent) []*models.Agent {
	out := make([]*models.Agent, len(list))
	for i, a := range list {
		out[i] = cloneAgent(a)
	}
	return out
}

func cloneMCPServer(s *models.MCPServer) *models.MCPServer {
	copy := *s
	if len(s.Args) > 0 {
		copy.Args = append([]string(nil), s.Args...)
	}
	if s.Env != nil {
		copy.Env = map[string]string{}
		for k, v := range s.Env {
			copy.Env[k] = v
		}
	}
	return &copy
}

func cloneMCPServers(list []*models.MCPServer) []*models.MCPServer {
	out := make([]*models.MCPServer, len(list))
	for i, s := range list {
		out[i] = cloneMCPServer(s)
	}
	return out
}
