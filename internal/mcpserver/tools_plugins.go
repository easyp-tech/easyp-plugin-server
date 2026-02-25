package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/easyp-tech/service/internal/core"
)

type pluginsListInput struct {
	Group   string   `json:"group,omitempty"`
	Name    string   `json:"name,omitempty"`
	Version string   `json:"version,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type pluginsListItem struct {
	ID        string   `json:"id"`
	Group     string   `json:"group"`
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Tags      []string `json:"tags,omitempty"`
	CreatedAt string   `json:"created_at"`
}

type pluginsListOutput struct {
	Total   int               `json:"total"`
	Plugins []pluginsListItem `json:"plugins"`
}

const (
	pluginsListToolName = "plugins_list"
)

func registerPluginTools(server *mcp.Server, pluginService PluginService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:         pluginsListToolName,
		Description:  "List available plugins with optional filters: group, name, version, tags.",
		InputSchema:  pluginsListInputSchema(),
		OutputSchema: pluginsListOutputSchema(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input pluginsListInput) (*mcp.CallToolResult, pluginsListOutput, error) {
		filter := core.PluginFilter{
			Group:   strings.TrimSpace(input.Group),
			Name:    strings.TrimSpace(input.Name),
			Version: strings.TrimSpace(input.Version),
			Tags:    compactStrings(input.Tags),
		}

		plugins, err := pluginService.ListPlugins(ctx, filter)
		if err != nil {
			return nil, pluginsListOutput{}, fmt.Errorf("list plugins: %w", err)
		}

		items := make([]pluginsListItem, 0, len(plugins))
		for _, p := range plugins {
			items = append(items, pluginsListItem{
				ID:        p.ID.String(),
				Group:     p.Group,
				Name:      p.Name,
				Version:   p.Version,
				Tags:      append([]string(nil), p.Tags...),
				CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
			})
		}

		return nil, pluginsListOutput{
			Total:   len(items),
			Plugins: items,
		}, nil
	})
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}
