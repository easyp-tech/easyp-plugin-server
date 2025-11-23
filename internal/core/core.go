// Package core contains the core implementation of the business logic.
package core

import (
	"context"
	"fmt"
	"strings"
)

// Core defines the interface for interacting with the plugin server.
type Core struct {
	metrics  Metrics
	registry Registry
}

// New creates a new Core instance.
func New(metrics Metrics, registry Registry) *Core {
	return &Core{
		metrics:  metrics,
		registry: registry,
	}
}

// Generate generates code by plugin.
func (c *Core) Generate(ctx context.Context, req GenerateCodeRequest) (*GenerateCodeResponse, error) {
	group, err := getGroup(req.PluginName)
	if err != nil {
		return nil, fmt.Errorf("getGroup: %w", err)
	}

	name, version, err := getNameAndVersion(req.PluginName)
	if err != nil {
		return nil, fmt.Errorf("getNameAndVersion: %w", err)
	}

	plugin, err := c.registry.Get(ctx, group, name, version)
	if err != nil {
		return nil, fmt.Errorf("c.registry.Get: %w", err)
	}

	generatedCode, err := plugin.Generate(ctx, req.Payload)
	if err != nil {
		return nil, fmt.Errorf("plugin.Generate: %w", err)
	}

	err = c.metrics.GenerateCode(ctx, *plugin.Info(ctx))
	if err != nil {
		return nil, fmt.Errorf("c.metrics.GenerateCode: %w", err)
	}

	return &GenerateCodeResponse{
		Payload: generatedCode,
	}, nil
}

// ListPlugins retrieves a list of plugins matching the filter.
func (c *Core) ListPlugins(ctx context.Context, filter PluginFilter) ([]PluginInfo, error) {
	plugins, err := c.registry.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("c.registry.List: %w", err)
	}

	return plugins, nil
}

func getGroup(pluginName string) (string, error) {
	splitArray := strings.Split(pluginName, "/")
	if len(splitArray) != 2 {
		return "", fmt.Errorf("%w: %s", ErrInvalidPluginName, pluginName)
	}

	return splitArray[0], nil
}

func getNameAndVersion(pluginName string) (string, string, error) {
	splitArray := strings.Split(pluginName, "/")
	if len(splitArray) != 2 {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidPluginName, pluginName)
	}

	nameVersion := strings.Split(splitArray[1], ":")
	if len(nameVersion) != 2 {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidPluginName, pluginName)
	}

	return nameVersion[0], nameVersion[1], nil
}
