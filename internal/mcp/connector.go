package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SolaTyolo/mcphub/internal/models"
)

func connect(ctx context.Context, srv *models.MCPServer) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "mcphub", Version: "0.1.0"}, nil)
	transport, err := newTransport(ctx, srv)
	if err != nil {
		return nil, err
	}
	return client.Connect(ctx, transport)
}

func newTransport(ctx context.Context, srv *models.MCPServer) (mcp.Transport, error) {
	switch srv.Transport {
	case models.TransportStdio:
		cmd := exec.CommandContext(ctx, srv.Command, srv.Args...)
		cmd.Env = os.Environ()
		for k, v := range srv.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		return mcp.NewCommandTransport(cmd), nil
	case models.TransportHTTP:
		return mcp.NewStreamableClientTransport(srv.URL, &mcp.StreamableClientTransportOptions{
			HTTPClient: httpClientFromEnv(srv.Env),
		}), nil
	default:
		return nil, fmt.Errorf("unsupported transport %s", srv.Transport)
	}
}

// ConnectTarget describes the MCP server endpoint for logging.
func ConnectTarget(srv *models.MCPServer) string {
	return connectTarget(srv)
}

func connectTarget(srv *models.MCPServer) string {
	if srv.Transport == models.TransportHTTP {
		return srv.URL
	}
	return srv.Command + " " + joinArgs(srv.Args)
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	out := args[0]
	for _, a := range args[1:] {
		out += " " + a
	}
	return out
}
