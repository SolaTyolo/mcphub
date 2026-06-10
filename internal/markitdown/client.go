package markitdown

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SolaTyolo/mcphub/internal/config"
)

type Client struct {
	url string
	mu  sync.Mutex
	ses *mcp.ClientSession
}

func NewClient(cfg config.Config) *Client {
	url := strings.TrimSpace(cfg.MarkItDownMCPURL)
	if url == "" {
		return nil
	}
	return &Client{url: url}
}

func (c *Client) Enabled() bool {
	return c != nil
}

func (c *Client) Convert(ctx context.Context, data []byte, filename string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("document parsing not configured (set MARKITDOWN_MCP_URL)")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty document")
	}

	session, err := c.session(ctx)
	if err != nil {
		return "", fmt.Errorf("connect markitdown: %w", err)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	uri := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "convert_to_markdown",
		Arguments: map[string]any{"uri": uri},
	})
	if err != nil {
		return "", err
	}
	if res.IsError {
		return "", fmt.Errorf("markitdown: %s", textContent(res.Content))
	}
	md := strings.TrimSpace(textContent(res.Content))
	if md == "" {
		return "", fmt.Errorf("no extractable text in document")
	}
	return md, nil
}

func (c *Client) session(ctx context.Context) (*mcp.ClientSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ses != nil {
		return c.ses, nil
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcphub", Version: "0.1.0"}, nil)
	transport := mcp.NewStreamableClientTransport(c.url, nil)
	ses, err := client.Connect(ctx, transport)
	if err != nil {
		return nil, err
	}
	c.ses = ses
	return ses, nil
}

func textContent(content []mcp.Content) string {
	var parts []string
	for _, item := range content {
		if tc, ok := item.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}
