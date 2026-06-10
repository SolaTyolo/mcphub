package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SolaTyolo/mcphub/internal/logx"
	"github.com/SolaTyolo/mcphub/internal/models"
)

const mcpTag = "mcp"

type ToolDef struct {
	Name         string
	Description  string
	ServerName   string
	OriginalName string
	InputSchema  map[string]any
}

type Pool struct {
	idleTTL  time.Duration
	mu       sync.Mutex
	sessions map[string]*managedSession
}

type managedSession struct {
	server   *models.MCPServer
	session  *mcp.ClientSession
	lastUsed time.Time
}

func NewPool(idleTTL time.Duration) *Pool {
	p := &Pool{
		idleTTL:  idleTTL,
		sessions: make(map[string]*managedSession),
	}
	go p.reapLoop()
	return p
}

func (p *Pool) Invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.sessions)
	for _, ms := range p.sessions {
		_ = ms.session.Close()
	}
	p.sessions = make(map[string]*managedSession)
	if n > 0 {
		logx.Info(mcpTag, "invalidated sessions=%d", n)
	}
}

func (p *Pool) ConnectAll(ctx context.Context, servers []*models.MCPServer) error {
	for _, srv := range servers {
		if err := srv.Validate(); err != nil {
			return err
		}
		p.mu.Lock()
		if _, exists := p.sessions[srv.ID]; exists {
			p.mu.Unlock()
			logx.Info(mcpTag, "reuse session server=%s", srv.Name)
			continue
		}
		p.mu.Unlock()

		logx.Info(mcpTag, "connect server=%s transport=%s target=%s",
			srv.Name, srv.Transport, connectTarget(srv))
		session, err := connect(ctx, srv)
		if err != nil {
			logx.Error(mcpTag, "connect failed server=%s: %v", srv.Name, err)
			return fmt.Errorf("connect %s: %w", srv.Name, err)
		}
		p.mu.Lock()
		p.sessions[srv.ID] = &managedSession{
			server:   srv,
			session:  session,
			lastUsed: time.Now(),
		}
		p.mu.Unlock()
		logx.Info(mcpTag, "connected server=%s", srv.Name)
	}
	return nil
}

func (p *Pool) ListTools(ctx context.Context) ([]ToolDef, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.sessions) == 0 {
		return nil, fmt.Errorf("mcp pool not initialized")
	}

	var tools []ToolDef
	for _, ms := range p.sessions {
		ms.lastUsed = time.Now()
		result, err := ms.session.ListTools(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("list tools %s: %w", ms.server.Name, err)
		}
		for _, t := range result.Tools {
			tools = append(tools, ToolDef{
				Name:         prefixTool(ms.server.Name, t.Name),
				Description:  t.Description,
				ServerName:   ms.server.Name,
				OriginalName: t.Name,
				InputSchema:  schemaToMap(t.InputSchema),
			})
		}
	}
	logx.Info(mcpTag, "listed tools count=%d", len(tools))
	return tools, nil
}

func (p *Pool) CallTool(ctx context.Context, prefixedName string, args map[string]any) (string, error) {
	serverName, toolName, err := splitToolName(prefixedName)
	if err != nil {
		return "", err
	}

	p.mu.Lock()
	var ms *managedSession
	for _, s := range p.sessions {
		if s.server.Name == serverName {
			ms = s
			break
		}
	}
	if ms == nil {
		p.mu.Unlock()
		return "", fmt.Errorf("server not found: %s", serverName)
	}
	ms.lastUsed = time.Now()
	p.mu.Unlock()

	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}
	res, err := ms.session.CallTool(ctx, params)
	if err != nil {
		return "", err
	}
	if res.IsError {
		return "", fmt.Errorf("tool error: %s", contentToString(res.Content))
	}
	return contentToString(res.Content), nil
}

func contentToString(content []mcp.Content) string {
	var parts []string
	for _, c := range content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		} else {
			b, _ := json.Marshal(c)
			parts = append(parts, string(b))
		}
	}
	return strings.Join(parts, "\n")
}

func schemaToMap(schema any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return out
}

func prefixTool(serverName, toolName string) string {
	return serverName + "__" + toolName
}

func splitToolName(prefixed string) (serverName, toolName string, err error) {
	idx := strings.Index(prefixed, "__")
	if idx <= 0 {
		return "", "", fmt.Errorf("invalid tool name: %s", prefixed)
	}
	return prefixed[:idx], prefixed[idx+2:], nil
}

func (p *Pool) reapLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		p.mu.Lock()
		now := time.Now()
		var closed int
		for sid, ms := range p.sessions {
			if now.Sub(ms.lastUsed) > p.idleTTL {
				_ = ms.session.Close()
				delete(p.sessions, sid)
				closed++
				logx.Info(mcpTag, "idle timeout server=%s idle=%s",
					ms.server.Name, now.Sub(ms.lastUsed).Round(time.Second))
			}
		}
		p.mu.Unlock()
		if closed > 0 {
			logx.Info(mcpTag, "reaped idle sessions count=%d", closed)
		}
	}
}
