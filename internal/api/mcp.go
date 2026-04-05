package api

import (
	"fmt"
	"log/slog"
	"net/http"

	easypmcp "github.com/easyp-tech/easyp/mcp/easypconfig"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/easyp-tech/service/api/generator/v1"
	"github.com/easyp-tech/service/internal/core"
)

func newMCPHandler(ps core.CoreService, logger *slog.Logger) http.Handler {
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

	if err := generator.RegisterServiceAPITools(srv, newPluginToolHandler(ps)); err != nil {
		panic(fmt.Errorf("register generator MCP tools: %w", err))
	}
	easypmcp.RegisterTool(srv)

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		Logger: logger,
	})
}

// MCPHandler returns the MCP streamable HTTP handler.
func (api *API) MCPHandler() http.Handler {
	return api.mcpHandler
}
