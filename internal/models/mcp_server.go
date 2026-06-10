package models

import (
	"fmt"
	"strings"
	"time"
)

type MCPServer struct {
	ID        string            `json:"id" yaml:"id"`
	Name      string            `json:"name" yaml:"name"`
	Transport string            `json:"transport" yaml:"transport"`
	Command   string            `json:"command,omitempty" yaml:"command,omitempty"`
	URL       string            `json:"url,omitempty" yaml:"url,omitempty"`
	Args      []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Enabled   bool              `json:"enabled" yaml:"enabled"`
	CreatedAt time.Time         `json:"createdAt" yaml:"createdAt"`
}

type CreateMCPServerRequest struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	URL       string            `json:"url,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Enabled   *bool             `json:"enabled,omitempty"`
}

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

func (s *MCPServer) Validate() error {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	transport := strings.TrimSpace(s.Transport)
	if transport == "" {
		transport = TransportStdio
	}
	s.Transport = transport

	switch transport {
	case TransportStdio:
		if strings.TrimSpace(s.Command) == "" {
			return fmt.Errorf("command is required for stdio transport")
		}
	case TransportHTTP:
		if strings.TrimSpace(s.URL) == "" {
			return fmt.Errorf("url is required for http transport")
		}
	default:
		return fmt.Errorf("unsupported transport %q (use stdio or http)", transport)
	}
	return nil
}
