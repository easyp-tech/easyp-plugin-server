// Package registry provides a registry for EasyP plugin server.
package registry

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/easyp-tech/service/internal/core"
	"github.com/easyp-tech/service/internal/database"
)

var _ core.Registry = &Registry{}
var _ core.Plugin = &plugin{}

type (
	// DockerConfig represents Docker execution configuration
	DockerConfig struct {
		Network    string            `json:"network,omitempty"`
		Memory     string            `json:"memory,omitempty"`
		CPUs       string            `json:"cpus,omitempty"`
		User       string            `json:"user,omitempty"`
		Env        map[string]string `json:"env,omitempty"`
		WorkingDir string            `json:"working_dir,omitempty"`
		ReadOnly   bool              `json:"read_only,omitempty"`
		TmpFS      map[string]string `json:"tmpfs,omitempty"`
	}

	// PluginConfig represents the complete plugin configuration
	PluginConfig struct {
		Docker *DockerConfig `json:"docker,omitempty"`
	}

	// Registry is a registry for EasyP plugin server.
	Registry struct {
		db     *database.SQL
		domain *url.URL
	}

	// plugin is a plugin in the registry.
	plugin struct {
		ID        uuid.UUID       `db:"id"`
		GroupName string          `db:"group_name"`
		Name      string          `db:"name"`
		Version   string          `db:"version"`
		Config    json.RawMessage `db:"config"`
		Tags      pq.StringArray  `db:"tags"`
		CreatedAt time.Time       `db:"created_at"`

		domain       *url.URL     `db:"-"`
		pluginConfig PluginConfig `db:"-"`
	}
)

// New build and returns a new Registry.
func New(ctx context.Context, db *database.SQL, domain string) (*Registry, error) {
	u, err := url.Parse(domain)
	if err != nil {
		return nil, fmt.Errorf("url.Parse: %w", err)
	}

	return &Registry{
		db:     db,
		domain: u,
	}, nil
}

// Get implements core.Registry.
func (r *Registry) Get(ctx context.Context, pluginGroup, pluginName, pluginVersion string) (p core.Plugin, err error) {
	dbFormat := plugin{}

	query := "select id, group_name, name, version, config, tags, created_at from plugins where group_name = $1 and name = $2 and version = $3"
	args := []any{pluginGroup, pluginName, pluginVersion}

	if pluginVersion == "latest" {
		query = "select id, group_name, name, version, config, tags, created_at from plugins where group_name = $1 and name = $2 order by version desc limit 1"
		args = []any{pluginGroup, pluginName}
	}

	if err := r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		return conn.GetContext(ctx, &dbFormat, query, args...)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s/%s:%s", core.ErrNotFound, pluginGroup, pluginName, pluginVersion)
		}
		return nil, fmt.Errorf("r.db.NoTxContext(Get): %w", err)
	}

	// Parse plugin configuration
	if len(dbFormat.Config) > 0 {
		if err := json.Unmarshal(dbFormat.Config, &dbFormat.pluginConfig); err != nil {
			return nil, fmt.Errorf("json.Unmarshal config: %w", err)
		}
	}

	dbFormat.domain = r.domain
	p = &dbFormat
	return p, nil
}

// List implements core.Registry.
func (r *Registry) List(ctx context.Context, filter core.PluginFilter) ([]core.PluginInfo, error) {
	var plugins []plugin
	query := "select id, group_name, name, version, tags, created_at from plugins where 1=1"
	var args []any
	argID := 1

	if filter.Group != "" {
		query += fmt.Sprintf(" and group_name = $%d", argID)
		args = append(args, filter.Group)
		argID++
	}
	if filter.Name != "" {
		query += fmt.Sprintf(" and name = $%d", argID)
		args = append(args, filter.Name)
		argID++
	}
	if filter.Version != "" {
		query += fmt.Sprintf(" and version = $%d", argID)
		args = append(args, filter.Version)
		argID++
	}

	if len(filter.Tags) > 0 {
		var nonEmptyTags []string
		for _, t := range filter.Tags {
			if t != "" {
				nonEmptyTags = append(nonEmptyTags, t)
			}
		}
		if len(nonEmptyTags) > 0 {
			query += fmt.Sprintf(" and tags @> $%d", argID)
			args = append(args, pq.Array(nonEmptyTags))
			argID++
		}
	}

	if err := r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		return conn.SelectContext(ctx, &plugins, query, args...)
	}); err != nil {
		return nil, fmt.Errorf("r.db.NoTxContext(List): %w", err)
	}

	result := make([]core.PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, *p.Info(ctx))
	}

	return result, nil
}

// Close database connection.
func (r *Registry) Close() error {
	return r.db.Close()
}

// Health checks the health of the registry.
func (r *Registry) Health(ctx context.Context) error {
	return r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		return conn.PingContext(ctx)
	})
}

// DB returns the underlying *database.SQL connection.
func (r *Registry) DB() *database.SQL { return r.db }

// Generate implements core.Plugin.
func (p *plugin) Generate(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	requestData, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("proto.Marshal: %w", err)
	}

	imageName := p.domain.String() + "/" + p.GroupName + "/" + p.Name + ":" + p.Version

	// Build Docker command with configuration from database
	args := []string{"run", "--rm", "-i"}

	// Get Docker configuration
	dockerConfig := p.pluginConfig.Docker

	// Apply Docker configuration from database
	if dockerConfig.Network != "" {
		args = append(args, "--network="+dockerConfig.Network)
	} else {
		// Default security: no network access
		args = append(args, "--network=none")
	}

	if dockerConfig.Memory != "" {
		args = append(args, "--memory="+dockerConfig.Memory)
	} else {
		// Default memory limit
		args = append(args, "--memory=128m")
	}

	if dockerConfig.CPUs != "" {
		args = append(args, "--cpus="+dockerConfig.CPUs)
	} else {
		// Default CPU limit
		args = append(args, "--cpus=1.0")
	}

	if dockerConfig.User != "" {
		args = append(args, "--user="+dockerConfig.User)
	}

	if dockerConfig.WorkingDir != "" {
		args = append(args, "--workdir="+dockerConfig.WorkingDir)
	}

	if dockerConfig.ReadOnly {
		args = append(args, "--read-only")
	}

	// Add environment variables
	for key, value := range dockerConfig.Env {
		args = append(args, "--env", key+"="+value)
	}

	// Add tmpfs mounts
	for path, opts := range dockerConfig.TmpFS {
		if opts != "" {
			args = append(args, "--tmpfs", path+":"+opts)
		} else {
			args = append(args, "--tmpfs", path)
		}
	}

	args = append(args, imageName)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = bytes.NewReader(requestData)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("plugin execution failed: %s, stderr: %s", err, string(exitErr.Stderr))
		}

		return nil, fmt.Errorf("cmd.Output: %w", err)
	}

	var response pluginpb.CodeGeneratorResponse
	if err := proto.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("proto.Unmarshal: %w", err)
	}

	return &response, nil
}

// Info implements core.Plugin.
func (p *plugin) Info(_ context.Context) *core.PluginInfo {
	return &core.PluginInfo{
		ID:        p.ID,
		Group:     p.GroupName,
		Name:      p.Name,
		Version:   p.Version,
		Tags:      []string(p.Tags),
		CreatedAt: p.CreatedAt,
	}
}
