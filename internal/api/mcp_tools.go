package api

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/easyp-tech/service/api/generator/v1"
	"github.com/easyp-tech/service/internal/core"
)

const pluginsListToolName = "plugins_list"

type pluginToolHandler struct {
	ps core.CoreService
}

func newPluginToolHandler(ps core.CoreService) *pluginToolHandler {
	return &pluginToolHandler{ps: ps}
}

func (h *pluginToolHandler) Plugins(ctx context.Context, req *generator.PluginsRequest) (*generator.PluginsResponse, error) {
	response := &generator.PluginsResponse{
		Plugins: make([]*generator.PluginInfo, 0),
	}
	if req == nil {
		req = &generator.PluginsRequest{}
	}
	if h == nil || h.ps == nil {
		return response, nil
	}

	filter := core.PluginFilter{
		Group:   strings.TrimSpace(req.GetGroup()),
		Name:    strings.TrimSpace(req.GetName()),
		Version: strings.TrimSpace(req.GetVersion()),
		Tags:    compactStrings(req.GetTags()),
	}

	plugins, err := h.ps.ListPlugins(ctx, filter)
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
