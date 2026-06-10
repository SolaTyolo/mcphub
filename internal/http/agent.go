package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/SolaTyolo/mcphub/internal/logx"
	"github.com/SolaTyolo/mcphub/internal/models"
	"github.com/SolaTyolo/mcphub/internal/stt"
)

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var req models.CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ag, err := models.AgentFromRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.CreateAgent(ag)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, errors.New("agent name already exists"))
			return
		}
		logx.Error(httpTag, "create agent name=%q: %v", req.Name, err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	logx.Info(httpTag, "agent created id=%s name=%q mcp_servers=%d", created.ID, created.Name, len(created.MCPServerIDs))
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListAgents(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.ListAgents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []*models.Agent{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	ag, err := s.store.GetAgent(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ag)
}

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	var req models.UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	updated, err := s.store.UpdateAgent(r.PathValue("id"), &req)
	if err != nil {
		if strings.Contains(err.Error(), "cannot be empty") {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		logx.Error(httpTag, "update agent id=%s: %v", r.PathValue("id"), err)
		writeStoreError(w, err)
		return
	}
	s.pool.Invalidate()
	logx.Info(httpTag, "agent updated id=%s name=%q", updated.ID, updated.Name)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteAgent(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	s.pool.Invalidate()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")
	ag, err := s.store.GetAgent(agentID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !ag.Enabled {
		writeError(w, http.StatusBadRequest, errors.New("agent is disabled"))
		return
	}

	var req models.ChatRequest
	var transcription string

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		req, transcription, err = s.parseMultipartChat(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("messages required"))
		return
	}

	servers, err := s.store.ListEnabledMCPServersForAgent(agentID)
	if err != nil {
		logx.Error(httpTag, "chat list mcp servers agent=%s: %v", agentID, err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	resp, err := s.agent.Chat(r.Context(), ag, servers, req)
	if err != nil {
		logx.Error(httpTag, "chat failed agent=%s messages=%d: %v", agentID, len(req.Messages), err)
		writeError(w, http.StatusBadGateway, err)
		return
	}
	resp.Transcription = transcription
	logx.Info(httpTag, "chat ok agent=%s endpoint=%s model=%s tool_calls=%d transcribed=%t",
		agentID, resp.Endpoint, resp.Model, len(resp.ToolCalls), transcription != "")
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) parseMultipartChat(r *http.Request) (models.ChatRequest, string, error) {
	const maxMemory = 32 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return models.ChatRequest{}, "", err
	}

	var req models.ChatRequest
	if raw := r.FormValue("messages"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &req); err != nil {
			return models.ChatRequest{}, "", err
		}
	}
	if model := strings.TrimSpace(r.FormValue("model")); model != "" {
		req.Model = model
	}

	var transcription string
	textPrompt := strings.TrimSpace(r.FormValue("text"))

	if file, header, err := r.FormFile("audio"); err == nil {
		defer file.Close()
		if s.stt == nil || !s.stt.Enabled() {
			return models.ChatRequest{}, "", errors.New("speech-to-text not configured (set WHISPER_BASE_URL)")
		}
		text, err := s.stt.Transcribe(r.Context(), file, stt.TranscribeOptions{
			Language: r.FormValue("language"),
			Filename: header.Filename,
		})
		if err != nil {
			return models.ChatRequest{}, "", err
		}
		if text == "" {
			return models.ChatRequest{}, "", errors.New("empty transcription")
		}
		transcription = text
		req.Messages = append(req.Messages, models.NewTextMessage("user", text))
	}

	if file, _, err := r.FormFile("image"); err == nil {
		defer file.Close()
		dataURL, err := fileToDataURL(file)
		if err != nil {
			return models.ChatRequest{}, "", err
		}
		req.Messages = append(req.Messages, models.NewMultimodalMessage("user", textPrompt, dataURL))
	} else if textPrompt != "" && transcription == "" {
		req.Messages = append(req.Messages, models.NewTextMessage("user", textPrompt))
	}

	if len(req.Messages) == 0 {
		return models.ChatRequest{}, "", errors.New("messages, audio, image, or text required")
	}
	return req, transcription, nil
}

func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if s.stt == nil || !s.stt.Enabled() {
		writeError(w, http.StatusServiceUnavailable, errors.New("speech-to-text not configured"))
		return
	}

	contentType := r.Header.Get("Content-Type")
	var reader io.Reader
	var filename string
	var language string

	if strings.HasPrefix(contentType, "multipart/form-data") {
		const maxMemory = 32 << 20
		if err := r.ParseMultipartForm(maxMemory); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		file, header, err := r.FormFile("audio")
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("audio file required"))
			return
		}
		defer file.Close()
		reader = file
		filename = header.Filename
		language = r.FormValue("language")
	} else {
		writeError(w, http.StatusBadRequest, errors.New("multipart/form-data with audio field required"))
		return
	}

	text, err := s.stt.Transcribe(r.Context(), reader, stt.TranscribeOptions{
		Language: language,
		Filename: filename,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": text})
}
