package mcpserver

import (
	"context"
	"log/slog"
	"net/http"

	easypmcp "github.com/easyp-tech/easyp/mcp/easypconfig"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/easyp-tech/service/internal/core"
)

// PluginService provides plugin discovery for MCP tools.
type PluginService interface {
	ListPlugins(ctx context.Context, filter core.PluginFilter) ([]core.PluginInfo, error)
}

// Server wraps MCP HTTP handler.
type Server struct {
	handler http.Handler
}

// New creates an MCP server and returns its HTTP handler wrapper.
func New(pluginService PluginService, logger *slog.Logger) *Server {
	opts := &mcp.ServerOptions{
		Instructions: "EasyP MCP server with plugin discovery and easyp.yaml schema helpers.",
	}
	if logger != nil {
		opts.Logger = logger
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "easyp-service-mcp",
		Version: "v1.0.0",
	}, opts)

	registerPluginTools(srv, pluginService)
	easypmcp.RegisterTool(srv)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		Logger: logger,
	})

	return &Server{handler: handler}
}

// Handler returns MCP streamable HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.handler
}
