package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SolaTyolo/mcphub/internal/models"
)

type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(ctx context.Context, dsn string) (*PGStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &PGStore{pool: pool}, nil
}

func (s *PGStore) Close() {
	s.pool.Close()
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *PGStore) CreateAgent(a *models.Agent) (*models.Agent, error) {
	now := time.Now().UTC()
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	a.CreatedAt = now
	schemaJSON, err := json.Marshal(a.ResponseSchema)
	if err != nil {
		return nil, err
	}
	if a.ResponseSchema == nil {
		schemaJSON = nil
	}

	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())

	_, err = tx.Exec(context.Background(),
		`insert into agents (id, name, system_prompt, response_schema, response_description,
         llm_base_url, llm_api_key, llm_model, vision_model, enabled, created_at)
         values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		a.ID, a.Name, a.SystemPrompt, schemaJSON, nullIfEmpty(a.ResponseDescription),
		nullIfEmpty(a.LLMBaseURL), nullIfEmpty(a.LLMAPIKey), nullIfEmpty(a.LLMModel), nullIfEmpty(a.VisionModel),
		a.Enabled, a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := s.setAgentMCPServersTx(tx, a.ID, a.MCPServerIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(context.Background()); err != nil {
		return nil, err
	}
	return s.GetAgent(a.ID)
}

func (s *PGStore) setAgentMCPServersTx(tx pgx.Tx, agentID string, serverIDs []string) error {
	if _, err := tx.Exec(context.Background(), `delete from agent_mcp_servers where agent_id = $1`, agentID); err != nil {
		return err
	}
	for _, sid := range serverIDs {
		sid = strings.TrimSpace(sid)
		if sid == "" {
			continue
		}
		if _, err := tx.Exec(context.Background(),
			`insert into agent_mcp_servers (agent_id, server_id) values ($1,$2)`, agentID, sid); err != nil {
			return err
		}
	}
	return nil
}

func (s *PGStore) ListAgents() ([]*models.Agent, error) {
	rows, err := s.pool.Query(context.Background(),
		`select id, name, system_prompt, response_schema, response_description,
         llm_base_url, llm_api_key, llm_model, vision_model, enabled, created_at
         from agents order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Agent
	for rows.Next() {
		ag, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		ids, err := s.listAgentMCPServerIDs(ag.ID)
		if err != nil {
			return nil, err
		}
		ag.MCPServerIDs = ids
		list = append(list, ag)
	}
	return list, rows.Err()
}

func (s *PGStore) GetAgent(id string) (*models.Agent, error) {
	row := s.pool.QueryRow(context.Background(),
		`select id, name, system_prompt, response_schema, response_description,
         llm_base_url, llm_api_key, llm_model, vision_model, enabled, created_at
         from agents where id = $1`, id)
	ag, err := scanAgentRow(row)
	if err != nil {
		return nil, err
	}
	ag.MCPServerIDs, err = s.listAgentMCPServerIDs(id)
	if err != nil {
		return nil, err
	}
	return ag, nil
}

func (s *PGStore) listAgentMCPServerIDs(agentID string) ([]string, error) {
	rows, err := s.pool.Query(context.Background(),
		`select server_id from agent_mcp_servers where agent_id = $1 order by server_id`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *PGStore) UpdateAgent(id string, req *models.UpdateAgentRequest) (*models.Agent, error) {
	existing, err := s.GetAgent(id)
	if err != nil {
		return nil, err
	}

	name := existing.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
	}
	systemPrompt := existing.SystemPrompt
	if req.SystemPrompt != nil {
		systemPrompt = *req.SystemPrompt
	}
	responseDescription := existing.ResponseDescription
	if req.ResponseDescription != nil {
		responseDescription = *req.ResponseDescription
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	responseSchema := existing.ResponseSchema
	if req.ResponseSchema != nil {
		responseSchema = req.ResponseSchema
	}
	mcpServerIDs := existing.MCPServerIDs
	if req.MCPServerIDs != nil {
		mcpServerIDs = req.MCPServerIDs
	}
	baseURL := existing.LLMBaseURL
	if req.LLMBaseURL != nil {
		baseURL = strings.TrimSpace(*req.LLMBaseURL)
	}
	apiKey := existing.LLMAPIKey
	if req.LLMAPIKey != nil {
		apiKey = strings.TrimSpace(*req.LLMAPIKey)
	}
	llmModel := existing.LLMModel
	if req.LLMModel != nil {
		llmModel = strings.TrimSpace(*req.LLMModel)
	}
	visionModel := existing.VisionModel
	if req.VisionModel != nil {
		visionModel = strings.TrimSpace(*req.VisionModel)
	}

	schemaJSON, err := json.Marshal(responseSchema)
	if err != nil {
		return nil, err
	}
	if responseSchema == nil {
		schemaJSON = nil
	}

	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())

	tag, err := tx.Exec(context.Background(),
		`update agents set name=$2, system_prompt=$3, response_schema=$4, response_description=$5,
         llm_base_url=$6, llm_api_key=$7, llm_model=$8, vision_model=$9, enabled=$10 where id=$1`,
		id, name, systemPrompt, schemaJSON, nullIfEmpty(responseDescription),
		nullIfEmpty(baseURL), nullIfEmpty(apiKey), nullIfEmpty(llmModel), nullIfEmpty(visionModel), enabled,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	if err := s.setAgentMCPServersTx(tx, id, mcpServerIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(context.Background()); err != nil {
		return nil, err
	}
	return s.GetAgent(id)
}

func (s *PGStore) DeleteAgent(id string) error {
	tag, err := s.pool.Exec(context.Background(), `delete from agents where id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) ListEnabledMCPServersForAgent(agentID string) ([]*models.MCPServer, error) {
	rows, err := s.pool.Query(context.Background(),
		`select m.id, m.name, m.transport, m.command, m.url, m.args, m.env, m.enabled, m.created_at
         from mcp_servers m
         join agent_mcp_servers am on am.server_id = m.id
         where am.agent_id = $1 and m.enabled = true
         order by m.name`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMCPServers(rows)
}

func (s *PGStore) CreateMCPServer(srv *models.MCPServer) (*models.MCPServer, error) {
	now := time.Now().UTC()
	if srv.ID == "" {
		srv.ID = uuid.NewString()
	}
	srv.CreatedAt = now
	argsJSON, _ := json.Marshal(srv.Args)
	envJSON, _ := json.Marshal(srv.Env)
	if srv.Env == nil {
		envJSON = []byte("{}")
	}
	_, err := s.pool.Exec(context.Background(),
		`insert into mcp_servers (id, name, transport, command, url, args, env, enabled, created_at)
         values ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		srv.ID, srv.Name, srv.Transport, nullIfEmpty(srv.Command), nullIfEmpty(srv.URL), argsJSON, envJSON, srv.Enabled, srv.CreatedAt,
	)
	return srv, err
}

func (s *PGStore) ListMCPServers() ([]*models.MCPServer, error) {
	rows, err := s.pool.Query(context.Background(),
		`select id, name, transport, command, url, args, env, enabled, created_at
         from mcp_servers order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMCPServers(rows)
}

func (s *PGStore) GetMCPServer(serverID string) (*models.MCPServer, error) {
	row := s.pool.QueryRow(context.Background(),
		`select id, name, transport, command, url, args, env, enabled, created_at
         from mcp_servers where id = $1`, serverID)
	return scanMCPServerRow(row)
}

func (s *PGStore) UpdateMCPServer(serverID string, srv *models.MCPServer) (*models.MCPServer, error) {
	existing, err := s.GetMCPServer(serverID)
	if err != nil {
		return nil, err
	}
	argsJSON, _ := json.Marshal(srv.Args)
	envJSON, _ := json.Marshal(srv.Env)
	tag, err := s.pool.Exec(context.Background(),
		`update mcp_servers set name=$2, transport=$3, command=$4, url=$5, args=$6, env=$7, enabled=$8
         where id=$1`,
		serverID, srv.Name, srv.Transport, nullIfEmpty(srv.Command), nullIfEmpty(srv.URL), argsJSON, envJSON, srv.Enabled,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	srv.ID = existing.ID
	srv.CreatedAt = existing.CreatedAt
	return srv, nil
}

func (s *PGStore) DeleteMCPServer(serverID string) error {
	tag, err := s.pool.Exec(context.Background(), `delete from mcp_servers where id = $1`, serverID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAgent(rows rowScanner) (*models.Agent, error) {
	var ag models.Agent
	var schemaRaw []byte
	var desc, baseURL, apiKey, llmModel, visionModel *string
	if err := rows.Scan(&ag.ID, &ag.Name, &ag.SystemPrompt, &schemaRaw, &desc,
		&baseURL, &apiKey, &llmModel, &visionModel, &ag.Enabled, &ag.CreatedAt); err != nil {
		return nil, err
	}
	if len(schemaRaw) > 0 {
		_ = json.Unmarshal(schemaRaw, &ag.ResponseSchema)
	}
	if desc != nil {
		ag.ResponseDescription = *desc
	}
	if baseURL != nil {
		ag.LLMBaseURL = *baseURL
	}
	if apiKey != nil {
		ag.LLMAPIKey = *apiKey
	}
	if llmModel != nil {
		ag.LLMModel = *llmModel
	}
	if visionModel != nil {
		ag.VisionModel = *visionModel
	}
	return &ag, nil
}

func scanAgentRow(row rowScanner) (*models.Agent, error) {
	ag, err := scanAgent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return ag, nil
}

func scanMCPServers(rows pgx.Rows) ([]*models.MCPServer, error) {
	var list []*models.MCPServer
	for rows.Next() {
		srv, err := scanMCPServer(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, srv)
	}
	return list, rows.Err()
}

func scanMCPServer(rows rowScanner) (*models.MCPServer, error) {
	var srv models.MCPServer
	var command, url *string
	var argsRaw, envRaw []byte
	if err := rows.Scan(&srv.ID, &srv.Name, &srv.Transport, &command, &url,
		&argsRaw, &envRaw, &srv.Enabled, &srv.CreatedAt); err != nil {
		return nil, err
	}
	if command != nil {
		srv.Command = *command
	}
	if url != nil {
		srv.URL = *url
	}
	_ = json.Unmarshal(argsRaw, &srv.Args)
	_ = json.Unmarshal(envRaw, &srv.Env)
	return &srv, nil
}

func scanMCPServerRow(row rowScanner) (*models.MCPServer, error) {
	srv, err := scanMCPServer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return srv, nil
}
