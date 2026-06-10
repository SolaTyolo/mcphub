package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/SolaTyolo/mcphub/internal/models"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(ctx context.Context, dsn string) (*SQLiteStore, error) {
	if !strings.Contains(dsn, "://") {
		dsn = "file:" + dsn + "?cache=shared&mode=rwc"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &SQLiteStore{db: db}
	if err := s.db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) CreateAgent(a *models.Agent) (*models.Agent, error) {
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
		schemaJSON = []byte(nil)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`insert into agents (id, name, system_prompt, response_schema, response_description,
         llm_base_url, llm_api_key, llm_model, vision_model, enabled, created_at)
         values (?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.Name, a.SystemPrompt, nullBytes(schemaJSON), nullStr(a.ResponseDescription),
		nullStr(a.LLMBaseURL), nullStr(a.LLMAPIKey), nullStr(a.LLMModel), nullStr(a.VisionModel),
		boolInt(a.Enabled), a.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	if err := setAgentMCPServersSQLTx(tx, a.ID, a.MCPServerIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetAgent(a.ID)
}

func setAgentMCPServersSQLTx(tx *sql.Tx, agentID string, serverIDs []string) error {
	if _, err := tx.Exec(`delete from agent_mcp_servers where agent_id = ?`, agentID); err != nil {
		return err
	}
	for _, sid := range serverIDs {
		sid = strings.TrimSpace(sid)
		if sid == "" {
			continue
		}
		if _, err := tx.Exec(`insert into agent_mcp_servers (agent_id, server_id) values (?,?)`, agentID, sid); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) ListAgents() ([]*models.Agent, error) {
	rows, err := s.db.Query(
		`select id, name, system_prompt, response_schema, response_description,
         llm_base_url, llm_api_key, llm_model, vision_model, enabled, created_at
         from agents order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Agent
	for rows.Next() {
		ag, err := scanAgentSQL(rows)
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

func (s *SQLiteStore) GetAgent(id string) (*models.Agent, error) {
	row := s.db.QueryRow(
		`select id, name, system_prompt, response_schema, response_description,
         llm_base_url, llm_api_key, llm_model, vision_model, enabled, created_at
         from agents where id = ?`, id)
	ag, err := scanAgentSQLRow(row)
	if err != nil {
		return nil, err
	}
	ag.MCPServerIDs, err = s.listAgentMCPServerIDs(id)
	if err != nil {
		return nil, err
	}
	return ag, nil
}

func (s *SQLiteStore) listAgentMCPServerIDs(agentID string) ([]string, error) {
	rows, err := s.db.Query(`select server_id from agent_mcp_servers where agent_id = ? order by server_id`, agentID)
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

func (s *SQLiteStore) UpdateAgent(id string, req *models.UpdateAgentRequest) (*models.Agent, error) {
	existing, err := s.GetAgent(id)
	if err != nil {
		return nil, err
	}
	merged, err := mergeAgentUpdate(existing, req)
	if err != nil {
		return nil, err
	}

	schemaJSON, err := json.Marshal(merged.ResponseSchema)
	if err != nil {
		return nil, err
	}
	if merged.ResponseSchema == nil {
		schemaJSON = []byte(nil)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`update agents set name=?, system_prompt=?, response_schema=?, response_description=?,
         llm_base_url=?, llm_api_key=?, llm_model=?, vision_model=?, enabled=? where id=?`,
		merged.Name, merged.SystemPrompt, nullBytes(schemaJSON), nullStr(merged.ResponseDescription),
		nullStr(merged.LLMBaseURL), nullStr(merged.LLMAPIKey), nullStr(merged.LLMModel), nullStr(merged.VisionModel),
		boolInt(merged.Enabled), id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	if err := setAgentMCPServersSQLTx(tx, id, merged.MCPServerIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetAgent(id)
}

func (s *SQLiteStore) DeleteAgent(id string) error {
	res, err := s.db.Exec(`delete from agents where id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) ListEnabledMCPServersForAgent(agentID string) ([]*models.MCPServer, error) {
	rows, err := s.db.Query(
		`select m.id, m.name, m.transport, m.command, m.url, m.args, m.env, m.enabled, m.created_at
         from mcp_servers m
         join agent_mcp_servers am on am.server_id = m.id
         where am.agent_id = ? and m.enabled = 1
         order by m.name`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMCPServersSQL(rows)
}

func (s *SQLiteStore) CreateMCPServer(srv *models.MCPServer) (*models.MCPServer, error) {
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
	_, err := s.db.Exec(
		`insert into mcp_servers (id, name, transport, command, url, args, env, enabled, created_at)
         values (?,?,?,?,?,?,?,?,?)`,
		srv.ID, srv.Name, srv.Transport, nullStr(srv.Command), nullStr(srv.URL),
		string(argsJSON), string(envJSON), boolInt(srv.Enabled), srv.CreatedAt.Format(time.RFC3339),
	)
	return srv, err
}

func (s *SQLiteStore) ListMCPServers() ([]*models.MCPServer, error) {
	rows, err := s.db.Query(
		`select id, name, transport, command, url, args, env, enabled, created_at
         from mcp_servers order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMCPServersSQL(rows)
}

func (s *SQLiteStore) GetMCPServer(serverID string) (*models.MCPServer, error) {
	row := s.db.QueryRow(
		`select id, name, transport, command, url, args, env, enabled, created_at
         from mcp_servers where id = ?`, serverID)
	return scanMCPServerSQLRow(row)
}

func (s *SQLiteStore) UpdateMCPServer(serverID string, srv *models.MCPServer) (*models.MCPServer, error) {
	existing, err := s.GetMCPServer(serverID)
	if err != nil {
		return nil, err
	}
	argsJSON, _ := json.Marshal(srv.Args)
	envJSON, _ := json.Marshal(srv.Env)
	res, err := s.db.Exec(
		`update mcp_servers set name=?, transport=?, command=?, url=?, args=?, env=?, enabled=?
         where id=?`,
		srv.Name, srv.Transport, nullStr(srv.Command), nullStr(srv.URL),
		string(argsJSON), string(envJSON), boolInt(srv.Enabled), serverID,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	srv.ID = existing.ID
	srv.CreatedAt = existing.CreatedAt
	return srv, nil
}

func (s *SQLiteStore) DeleteMCPServer(serverID string) error {
	res, err := s.db.Exec(`delete from mcp_servers where id = ?`, serverID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanAgentSQL(rows *sql.Rows) (*models.Agent, error) {
	var ag models.Agent
	var schemaRaw sql.NullString
	var desc, baseURL, apiKey, llmModel, visionModel sql.NullString
	var enabled int
	var createdAt string
	if err := rows.Scan(&ag.ID, &ag.Name, &ag.SystemPrompt, &schemaRaw, &desc,
		&baseURL, &apiKey, &llmModel, &visionModel, &enabled, &createdAt); err != nil {
		return nil, err
	}
	if schemaRaw.Valid && schemaRaw.String != "" {
		_ = json.Unmarshal([]byte(schemaRaw.String), &ag.ResponseSchema)
	}
	ag.ResponseDescription = nullStrVal(desc)
	ag.LLMBaseURL = nullStrVal(baseURL)
	ag.LLMAPIKey = nullStrVal(apiKey)
	ag.LLMModel = nullStrVal(llmModel)
	ag.VisionModel = nullStrVal(visionModel)
	ag.Enabled = enabled != 0
	ag.CreatedAt = parseTime(createdAt)
	return &ag, nil
}

func scanAgentSQLRow(row *sql.Row) (*models.Agent, error) {
	var ag models.Agent
	var schemaRaw sql.NullString
	var desc, baseURL, apiKey, llmModel, visionModel sql.NullString
	var enabled int
	var createdAt string
	if err := row.Scan(&ag.ID, &ag.Name, &ag.SystemPrompt, &schemaRaw, &desc,
		&baseURL, &apiKey, &llmModel, &visionModel, &enabled, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if schemaRaw.Valid && schemaRaw.String != "" {
		_ = json.Unmarshal([]byte(schemaRaw.String), &ag.ResponseSchema)
	}
	ag.ResponseDescription = nullStrVal(desc)
	ag.LLMBaseURL = nullStrVal(baseURL)
	ag.LLMAPIKey = nullStrVal(apiKey)
	ag.LLMModel = nullStrVal(llmModel)
	ag.VisionModel = nullStrVal(visionModel)
	ag.Enabled = enabled != 0
	ag.CreatedAt = parseTime(createdAt)
	return &ag, nil
}

func scanMCPServersSQL(rows *sql.Rows) ([]*models.MCPServer, error) {
	var list []*models.MCPServer
	for rows.Next() {
		srv, err := scanMCPServerSQL(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, srv)
	}
	return list, rows.Err()
}

func scanMCPServerSQL(rows *sql.Rows) (*models.MCPServer, error) {
	var srv models.MCPServer
	var command, url sql.NullString
	var argsRaw, envRaw string
	var enabled int
	var createdAt string
	if err := rows.Scan(&srv.ID, &srv.Name, &srv.Transport, &command, &url,
		&argsRaw, &envRaw, &enabled, &createdAt); err != nil {
		return nil, err
	}
	srv.Command = nullStrVal(command)
	srv.URL = nullStrVal(url)
	_ = json.Unmarshal([]byte(argsRaw), &srv.Args)
	_ = json.Unmarshal([]byte(envRaw), &srv.Env)
	srv.Enabled = enabled != 0
	srv.CreatedAt = parseTime(createdAt)
	return &srv, nil
}

func scanMCPServerSQLRow(row *sql.Row) (*models.MCPServer, error) {
	var srv models.MCPServer
	var command, url sql.NullString
	var argsRaw, envRaw string
	var enabled int
	var createdAt string
	if err := row.Scan(&srv.ID, &srv.Name, &srv.Transport, &command, &url,
		&argsRaw, &envRaw, &enabled, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	srv.Command = nullStrVal(command)
	srv.URL = nullStrVal(url)
	_ = json.Unmarshal([]byte(argsRaw), &srv.Args)
	_ = json.Unmarshal([]byte(envRaw), &srv.Env)
	srv.Enabled = enabled != 0
	srv.CreatedAt = parseTime(createdAt)
	return &srv, nil
}

func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

func nullStrVal(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
