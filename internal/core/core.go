// Package core contains the core implementation of the business logic.
package core

import (
	"context"
	"fmt"
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
	plugin, err := c.registry.Get(ctx, req.PluginName)
	if err != nil {
		return nil, fmt.Errorf("c.registry.Get: %w", err)
	}

	generatedCode, err := plugin.Generate(ctx, req.Payload)
	if err != nil {
		return nil, fmt.Errorf("plugin.Generate: %w", err)
	}

	err = c.metrics.GenerateCode(ctx, req.PluginName)
	if err != nil {
		return nil, fmt.Errorf("c.metrics.GenerateCode: %w", err)
	}

	return &GenerateCodeResponse{
		Payload: generatedCode,
	}, nil
}
