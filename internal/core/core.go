// Package core contains the core implementation of the business logic.
package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Core defines the interface for interacting with the plugin server.
type Core struct {
	metrics     Metrics
	registry    Registry
	featureGate FeatureGate
}

// New creates a new Core instance.
// If featureGate is nil, all features are considered available (backward compatibility).
func New(metrics Metrics, registry Registry, featureGate FeatureGate) *Core {
	return &Core{
		metrics:     metrics,
		registry:    registry,
		featureGate: featureGate,
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

var (
	nameRegexp    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	versionRegexp = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
)

func validateGroupName(group, name string) error {
	if !nameRegexp.MatchString(group) {
		return fmt.Errorf("%w: invalid group %q", ErrInvalidPluginName, group)
	}
	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("%w: invalid name %q", ErrInvalidPluginName, name)
	}
	return nil
}

func validateVersion(version string) error {
	if version != "latest" && !versionRegexp.MatchString(version) {
		return fmt.Errorf("%w: invalid version %q", ErrInvalidPluginName, version)
	}
	return nil
}

// checkFeature returns ErrFeatureDenied if the given feature is not enabled.
// If featureGate is nil, all features are considered available.
func (c *Core) checkFeature(feature Feature) error {
	if c.featureGate == nil {
		return nil
	}
	if c.featureGate.Enabled(feature) {
		return nil
	}
	return fmt.Errorf("%w: feature %s", ErrFeatureDenied, feature)
}

// CreatePlugin registers a new plugin in the registry.
func (c *Core) CreatePlugin(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error) {
	if err := c.checkFeature(FeaturePluginCRUD); err != nil {
		return nil, err
	}
	if err := validateGroupName(req.Group, req.Name); err != nil {
		return nil, err
	}
	if err := validateVersion(req.Version); err != nil {
		return nil, err
	}

	// Check MaxPlugins limit.
	if limit := c.featureGate.MaxPlugins(); limit >= 0 {
		plugins, err := c.registry.List(ctx, PluginFilter{})
		if err != nil {
			return nil, fmt.Errorf("c.registry.List: %w", err)
		}
		if len(plugins) >= limit {
			return nil, fmt.Errorf("%w: current %d, limit %d", ErrMaxPluginsExceeded, len(plugins), limit)
		}
	}

	info, err := c.registry.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("c.registry.Create: %w", err)
	}

	return info, nil
}

// UpdatePlugin modifies config and tags of an existing plugin.
// Group, Name, and Version are immutable identifiers used as lookup keys.
func (c *Core) UpdatePlugin(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error) {
	if err := c.checkFeature(FeaturePluginCRUD); err != nil {
		return nil, err
	}
	if err := validateGroupName(req.Group, req.Name); err != nil {
		return nil, err
	}
	if err := validateVersion(req.Version); err != nil {
		return nil, err
	}

	info, err := c.registry.Update(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("c.registry.Update: %w", err)
	}

	return info, nil
}

// DeletePlugin removes a plugin from the registry.
// In-flight GenerateCode executions that already obtained a Plugin reference
// will complete fully; only subsequent requests for this plugin will fail.
func (c *Core) DeletePlugin(ctx context.Context, group, name, version string) error {
	if err := c.checkFeature(FeaturePluginCRUD); err != nil {
		return err
	}
	if err := c.registry.Delete(ctx, group, name, version); err != nil {
		return fmt.Errorf("c.registry.Delete: %w", err)
	}

	return nil
}
