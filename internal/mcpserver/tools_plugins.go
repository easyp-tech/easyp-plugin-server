package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	generator "github.com/easyp-tech/service/api/generator/v1"
	"github.com/easyp-tech/service/internal/core"
)

const (
	pluginsListToolName = "plugins_list"
)

type pluginToolHandler struct {
	pluginService PluginService
}

func newPluginToolHandler(pluginService PluginService) *pluginToolHandler {
	return &pluginToolHandler{pluginService: pluginService}
}

func (h *pluginToolHandler) Plugins(ctx context.Context, req *generator.PluginsRequest) (*generator.PluginsResponse, error) {
	response := &generator.PluginsResponse{
		Plugins: make([]*generator.PluginInfo, 0),
	}
	if req == nil {
		req = &generator.PluginsRequest{}
	}
	if h == nil || h.pluginService == nil {
		return response, nil
	}

	filter := core.PluginFilter{
		Group:   strings.TrimSpace(req.GetGroup()),
		Name:    strings.TrimSpace(req.GetName()),
		Version: strings.TrimSpace(req.GetVersion()),
		Tags:    compactStrings(req.GetTags()),
	}

	plugins, err := h.pluginService.ListPlugins(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}

	response.Total = int32(len(plugins))
	response.Plugins = make([]*generator.PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		response.Plugins = append(response.Plugins, &generator.PluginInfo{
			Id:        p.ID.String(),
			Group:     p.Group,
			Name:      p.Name,
			Version:   p.Version,
			CreatedAt: timestamppb.New(p.CreatedAt),
			Tags:      append([]string(nil), p.Tags...),
		})
	}

	return response, nil
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
