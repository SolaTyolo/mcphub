package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SolaTyolo/mcphub/internal/agent"
	"github.com/SolaTyolo/mcphub/internal/attachment"
	"github.com/SolaTyolo/mcphub/internal/config"
	"github.com/SolaTyolo/mcphub/internal/logx"
	"github.com/SolaTyolo/mcphub/internal/mcp"
	"github.com/SolaTyolo/mcphub/internal/models"
	"github.com/SolaTyolo/mcphub/internal/storage"
	"github.com/SolaTyolo/mcphub/internal/stt"
)

type Server struct {
	cfg         config.Config
	store       storage.Store
	attachments attachment.Store
	agent       *agent.Service
	pool        *mcp.Pool
	stt         *stt.Client
	static      http.Handler
}

func NewServer(cfg config.Config, store storage.Store, attachments attachment.Store, agentSvc *agent.Service, pool *mcp.Pool, sttClient *stt.Client, static http.Handler) *Server {
	return &Server{cfg: cfg, store: store, attachments: attachments, agent: agentSvc, pool: pool, stt: sttClient, static: static}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)

	auth := s.withAPIKeyAuth

	mux.HandleFunc("POST /api/agents", auth(s.handleCreateAgent))
	mux.HandleFunc("GET /api/agents", auth(s.handleListAgents))
	mux.HandleFunc("GET /api/agents/{id}", auth(s.handleGetAgent))
	mux.HandleFunc("PUT /api/agents/{id}", auth(s.handleUpdateAgent))
	mux.HandleFunc("DELETE /api/agents/{id}", auth(s.handleDeleteAgent))
	mux.HandleFunc("POST /api/agents/{id}/chat", auth(s.handleAgentChat))

	mux.HandleFunc("POST /api/mcp-servers", auth(s.handleCreateMCPServer))
	mux.HandleFunc("GET /api/mcp-servers", auth(s.handleListMCPServers))
	mux.HandleFunc("PUT /api/mcp-servers/{serverId}", auth(s.handleUpdateMCPServer))
	mux.HandleFunc("DELETE /api/mcp-servers/{serverId}", auth(s.handleDeleteMCPServer))
	mux.HandleFunc("POST /api/mcp-servers/{serverId}/test", auth(s.handleTestMCPServer))

	mux.HandleFunc("POST /api/transcribe", auth(s.handleTranscribe))
	mux.HandleFunc("POST /api/attachments", auth(s.handleUploadAttachment))

	if s.static != nil {
		mux.Handle("/", s.static)
	}
	return accessLogMiddleware(corsMiddleware(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateMCPServer(w http.ResponseWriter, r *http.Request) {
	var req models.CreateMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	srv, err := mcpServerFromRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateMCPServer(srv)
	if err != nil {
		logx.Error(httpTag, "create mcp server name=%q: %v", req.Name, err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.pool.Invalidate()
	logx.Info(httpTag, "mcp server created id=%s name=%q transport=%s enabled=%t",
		created.ID, created.Name, created.Transport, created.Enabled)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListMCPServers(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.ListMCPServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []*models.MCPServer{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleUpdateMCPServer(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("serverId")
	var req models.CreateMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	srv, err := mcpServerFromRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	srv.ID = serverID
	updated, err := s.store.UpdateMCPServer(serverID, srv)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.pool.Invalidate()
	logx.Info(httpTag, "mcp server updated id=%s name=%q enabled=%t",
		updated.ID, updated.Name, updated.Enabled)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("serverId")
	if err := s.store.DeleteMCPServer(serverID); err != nil {
		logx.Error(httpTag, "delete mcp server id=%s: %v", serverID, err)
		writeStoreError(w, err)
		return
	}
	s.pool.Invalidate()
	logx.Info(httpTag, "mcp server deleted id=%s", serverID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestMCPServer(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("serverId")
	srv, err := s.store.GetMCPServer(serverID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.pool.Invalidate()
	tools, err := s.agent.TestServer(r.Context(), srv)
	if err != nil {
		logx.Error(httpTag, "test mcp server id=%s name=%q: %v", serverID, srv.Name, err)
		writeError(w, http.StatusBadGateway, err)
		return
	}
	logx.Info(httpTag, "test mcp server ok id=%s name=%q tools=%d", serverID, srv.Name, len(tools))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"tools": tools,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	if status >= 500 {
		logx.Error(httpTag, "response status=%d: %v", status, err)
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}
