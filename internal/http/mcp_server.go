package httpapi

import (
	"github.com/SolaTyolo/mcphub/internal/models"
)

func mcpServerFromRequest(req models.CreateMCPServerRequest) (*models.MCPServer, error) {
	transport := req.Transport
	if transport == "" {
		transport = models.TransportStdio
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	srv := &models.MCPServer{
		Name:      req.Name,
		Transport: transport,
		Command:   req.Command,
		URL:       req.URL,
		Args:      req.Args,
		Env:       req.Env,
		Enabled:   enabled,
	}
	if err := srv.Validate(); err != nil {
		return nil, err
	}
	return srv, nil
}
