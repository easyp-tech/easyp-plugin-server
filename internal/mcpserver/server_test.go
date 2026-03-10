package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/easyp-tech/service/internal/core"
)

type fakePluginService struct {
	returnPlugins []core.PluginInfo
	lastFilter    core.PluginFilter
}

func (f *fakePluginService) Generate(context.Context, core.GenerateCodeRequest) (*core.GenerateCodeResponse, error) {
	return &core.GenerateCodeResponse{Payload: &pluginpb.CodeGeneratorResponse{}}, nil
}

func (f *fakePluginService) ListPlugins(_ context.Context, filter core.PluginFilter) ([]core.PluginInfo, error) {
	f.lastFilter = filter
	return f.returnPlugins, nil
}

func TestMCPServer_RegistersToolsAndListsPlugins(t *testing.T) {
	t.Parallel()

	pluginID := uuid.Must(uuid.NewV4())
	fake := &fakePluginService{
		returnPlugins: []core.PluginInfo{
			{
				ID:        pluginID,
				Group:     "grpc",
				Name:      "go",
				Version:   "v1.5.1",
				Tags:      []string{"stable", "go"},
				CreatedAt: time.Date(2026, time.February, 1, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	session, shutdown := newTestSession(t, fake)
	defer shutdown()

	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 2)

	toolNames := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	require.Contains(t, toolNames, pluginsListToolName)

	args := map[string]any{
		"group": "grpc",
		"name":  "go",
		"tags":  []string{"stable", "go"},
	}
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      pluginsListToolName,
		Arguments: args,
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var out struct {
		Total   int `json:"total"`
		Plugins []struct {
			ID        string   `json:"id"`
			Group     string   `json:"group"`
			Name      string   `json:"name"`
			Version   string   `json:"version"`
			Tags      []string `json:"tags"`
			CreatedAt string   `json:"created_at"`
		} `json:"plugins"`
	}
	decodeStructured(t, res, &out)

	require.Equal(t, 1, out.Total)
	require.Len(t, out.Plugins, 1)
	require.Equal(t, pluginID.String(), out.Plugins[0].ID)
	require.Equal(t, "grpc", out.Plugins[0].Group)
	require.Equal(t, "go", out.Plugins[0].Name)
	require.Equal(t, "v1.5.1", out.Plugins[0].Version)
	require.Equal(t, []string{"stable", "go"}, out.Plugins[0].Tags)
	require.Equal(t, "2026-02-01T10:00:00Z", out.Plugins[0].CreatedAt)

	require.Equal(t, core.PluginFilter{
		Group: "grpc",
		Name:  "go",
		Tags:  []string{"stable", "go"},
	}, fake.lastFilter)
}

func newTestSession(t *testing.T, pluginService PluginService) (*mcp.ClientSession, func()) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(pluginService, logger)

	mux := http.NewServeMux()
	mux.Handle("/mcp", s.Handler())

	httpSrv := httptest.NewServer(mux)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "v1.0.0",
	}, nil)

	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: httpSrv.URL + "/mcp",
	}, nil)
	require.NoError(t, err)

	return session, func() {
		session.Close()
		httpSrv.Close()
	}
}

func decodeStructured(t *testing.T, res *mcp.CallToolResult, dst any) {
	t.Helper()

	data, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, dst))
}
