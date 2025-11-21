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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sipki-tech/dev-platform/database"
	"github.com/sipki-tech/dev-platform/database/connectors"
	"github.com/sipki-tech/dev-platform/database/migrations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/easyp-tech/service/internal/core"
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
		// Future extensions can be added here:
		// Security SecurityConfig `json:"security,omitempty"`
		// Monitoring MonitoringConfig `json:"monitoring,omitempty"`
		// Cache CacheConfig `json:"cache,omitempty"`
	}

	// Config provide connection info for database.
	Config struct {
		Postgres   connectors.Raw
		MigrateDir string
		Driver     string
		Domain     string
	}

	// Registry is a registry for EasyP plugin server.
	Registry struct {
		sql    *database.SQL
		domain *url.URL
	}

	// plugin is a plugin in the registry.
	plugin struct {
		ID        uuid.UUID       `db:"id"`
		GroupName string          `db:"group_name"`
		Name      string          `db:"name"`
		Version   string          `db:"version"`
		Config    json.RawMessage `db:"config"`
		CreatedAt time.Time       `db:"created_at"`

		domain       *url.URL     `db:"-"`
		pluginConfig PluginConfig `db:"-"`
	}
)

// New build and returns a new Registry.
func New(ctx context.Context, reg *prometheus.Registry, namespace string, cfg Config) (*Registry, error) {
	const subsystem = "repo"
	m := database.NewMetrics(reg, namespace, subsystem, new(core.Registry))

	returnErrs := []error{ // List of core.Err… returned by Repo methods.
		core.ErrNotFound,
		core.ErrInvalidPluginName,
	}

	migrates, err := migrations.Parse(cfg.MigrateDir)
	if err != nil {
		return nil, fmt.Errorf("migrations.Parse: %w", err)
	}

	err = migrations.Run(ctx, cfg.Driver, &cfg.Postgres, migrations.Up, migrates)
	if err != nil {
		return nil, fmt.Errorf("migrations.Run: %w", err)
	}

	conn, err := database.NewSQL(ctx, cfg.Driver, database.SQLConfig{
		Metrics:    m,
		ReturnErrs: returnErrs,
	}, &cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("database.NewSQL: %w", err)
	}

	u, err := url.Parse(cfg.Domain)
	if err != nil {
		return nil, fmt.Errorf("url.Parse: %w", err)
	}

	return &Registry{
		sql:    conn,
		domain: u,
	}, nil
}

// Get implements core.Registry.
func (r *Registry) Get(ctx context.Context, pluginGroup, pluginName, pluginVersion string) (p core.Plugin, err error) {
	err = r.sql.NoTx(func(d *sqlx.DB) error {
		dbFormat := plugin{}

		query := "select id, group_name, name, version, config, created_at from plugins where group_name = $1 and name = $2 and version = $3"
		args := []any{pluginGroup, pluginName, pluginVersion}

		if pluginVersion == "latest" {
			query = "select id, group_name, name, version, config, created_at from plugins where group_name = $1 and name = $2 order by version desc limit 1"
			args = []any{pluginGroup, pluginName}
		}

		err := d.GetContext(ctx, &dbFormat, query, args...)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("d.GetContext: %w", core.ErrNotFound)
		case err != nil:
			return fmt.Errorf("d.GetContext: %w", err)
		}

		// Parse plugin configuration
		if len(dbFormat.Config) > 0 {
			if err := json.Unmarshal(dbFormat.Config, &dbFormat.pluginConfig); err != nil {
				return fmt.Errorf("json.Unmarshal config: %w", err)
			}
		}

		dbFormat.domain = r.domain
		p = &dbFormat
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sql.NoTx: %w", err)
	}

	return p, nil
}

// Close database connection.
func (r *Registry) Close() error {
	return r.sql.Close()
}

// Health checks the health of the registry.
func (r *Registry) Health(ctx context.Context) error {
	return r.sql.NoTx(func(db *sqlx.DB) error { return db.PingContext(ctx) })
}

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
		CreatedAt: p.CreatedAt,
	}
}
