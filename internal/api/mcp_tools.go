package api

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/easyp-tech/service/api/generator/v1"
	"github.com/easyp-tech/service/internal/core"
)

type pluginToolHandler struct {
	ps core.Service
}

func newPluginToolHandler(ps core.Service) *pluginToolHandler {
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

	response.Total = int32(len(plugins)) //nolint:gosec // len() result fits int32 in practice
	response.Plugins = make([]*generator.PluginInfo, 0, len(plugins))
	for _, plugInfo := range plugins {
		response.Plugins = append(response.Plugins, &generator.PluginInfo{
			Id:        plugInfo.ID.String(),
			Group:     plugInfo.Group,
			Name:      plugInfo.Name,
			Version:   plugInfo.Version,
			CreatedAt: timestamppb.New(plugInfo.CreatedAt),
			Tags:      append([]string(nil), plugInfo.Tags...),
		})
	}

	return response, nil
}
