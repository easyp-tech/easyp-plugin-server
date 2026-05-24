// Package registry provides a registry for EasyP plugin server.
package registry

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/easyp-tech/service/internal/core"
	"github.com/easyp-tech/service/internal/database"
)

const (
	numPipesReader = 2
	stderrLimit    = 1 * 1024 * 1024
)

// Registry package errors.
var (
	ErrEmptyConfig     = errors.New("empty config")
	ErrOldFormat       = errors.New("old format config is no longer supported")
	ErrEmptyCommand    = errors.New("empty command array")
	ErrInvalidConfig   = errors.New("invalid plugin configuration")
	ErrEmptyPluginsDir = errors.New("pluginsDir cannot be empty")
)

var (
	_ core.Registry = &Registry{}
	_ core.Plugin   = &plugin{}
)

type (
	// PluginConfig represents the complete plugin configuration.
	PluginConfig struct {
		Command []string          `json:"command"`
		Env     map[string]string `json:"env,omitempty"`
		Timeout string            `json:"timeout,omitempty"`
	}

	// Registry is a registry for EasyP plugin server.
	Registry struct {
		db            *database.SQL
		pluginsDir    string
		maxOutputSize int64
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

		maxOutputSize int64        `db:"-"`
		pluginConfig  PluginConfig `db:"-"`
	}
)

// ValidateConfig checks if the given config is valid.
func ValidateConfig(config json.RawMessage, pluginsDir string) error {
	if len(config) == 0 {
		return ErrEmptyConfig
	}

	// Check if it's in the old docker format
	var raw map[string]json.RawMessage
	unmarshalErr := json.Unmarshal(config, &raw)
	if unmarshalErr == nil {
		if _, exists := raw["docker"]; exists {
			return ErrOldFormat
		}
	}

	var pConfig PluginConfig
	unmarshalErr = json.Unmarshal(config, &pConfig)
	if unmarshalErr != nil {
		return fmt.Errorf("%w: json.Unmarshal config: %w", ErrInvalidConfig, unmarshalErr)
	}

	if len(pConfig.Command) == 0 {
		return ErrEmptyCommand
	}

	cleanedPluginsDir := filepath.Clean(pluginsDir)

	hasElementInPluginsDir := false
	for _, arg := range pConfig.Command {
		cleanedArg := filepath.Clean(arg)
		if cleanedArg == cleanedPluginsDir || strings.HasPrefix(cleanedArg, cleanedPluginsDir+string(filepath.Separator)) {
			hasElementInPluginsDir = true

			break
		}
	}

	if !hasElementInPluginsDir {
		return fmt.Errorf(
			"%w: must contain at least one path inside plugins directory (no traversal outside plugins directory allowed): %s",
			ErrInvalidConfig,
			pluginsDir,
		)
	}

	return nil
}

// New build and returns a new Registry.
func New(_ context.Context, db *database.SQL, pluginsDir string, maxOutputSize int64) (*Registry, error) {
	if pluginsDir == "" {
		return nil, ErrEmptyPluginsDir
	}

	return &Registry{
		db:            db,
		pluginsDir:    pluginsDir,
		maxOutputSize: maxOutputSize,
	}, nil
}

// Get implements core.Registry.
func (r *Registry) Get(ctx context.Context, pluginGroup, pluginName, pluginVersion string) (core.Plugin, error) {
	dbFormat := plugin{}

	query := "select id, group_name, name, version, config, tags, created_at from plugins where group_name = $1 and name = $2 and version = $3"
	args := []any{pluginGroup, pluginName, pluginVersion}

	if pluginVersion == "latest" {
		query = "select id, group_name, name, version, config, tags, created_at" +
			" from plugins where group_name = $1 and name = $2 order by version desc limit 1"
		args = []any{pluginGroup, pluginName}
	}

	err := r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		return conn.GetContext(ctx, &dbFormat, query, args...)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s/%s:%s", core.ErrNotFound, pluginGroup, pluginName, pluginVersion)
		}

		return nil, fmt.Errorf("r.db.NoTxContext(Get): %w", err)
	}

	// Parse plugin configuration
	if len(dbFormat.Config) > 0 {
		err := json.Unmarshal(dbFormat.Config, &dbFormat.pluginConfig)
		if err != nil {
			return nil, fmt.Errorf("json.Unmarshal config: %w", err)
		}
	}

	dbFormat.maxOutputSize = r.maxOutputSize

	return &dbFormat, nil
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
		}
	}

	err := r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		return conn.SelectContext(ctx, &plugins, query, args...)
	})
	if err != nil {
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
	err := r.db.Close()
	if err != nil {
		return fmt.Errorf("db.Close: %w", err)
	}

	return nil
}

// Health checks the health of the registry.
func (r *Registry) Health(ctx context.Context) error {
	err := r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		return conn.PingContext(ctx)
	})
	if err != nil {
		return fmt.Errorf("db.NoTxContext(Health): %w", err)
	}

	return nil
}

// DB returns the underlying *database.SQL connection.
func (r *Registry) DB() *database.SQL { return r.db }

// readPipes reads stdout and stderr in parallel up to limits and returns data.
func readPipes(stdout io.Reader, stderr io.Reader, maxStdout int64) ([]byte, []byte, error) {
	var stdoutData, stderrData []byte
	var stdoutErr, stderrErr error

	var wg sync.WaitGroup
	wg.Add(numPipesReader)

	go func() {
		defer wg.Done()

		lr := io.LimitReader(stdout, maxStdout+1)
		stdoutData, stdoutErr = io.ReadAll(lr)
	}()

	go func() {
		defer wg.Done()

		lr := io.LimitReader(stderr, stderrLimit)
		stderrData, stderrErr = io.ReadAll(lr)
	}()

	wg.Wait()

	if stdoutErr != nil {
		return nil, nil, fmt.Errorf("failed to read plugin stdout: %w", stdoutErr)
	}

	if stderrErr != nil {
		return nil, nil, fmt.Errorf("failed to read plugin stderr: %w", stderrErr)
	}

	return stdoutData, stderrData, nil
}

// Generate implements core.Plugin.
func (p *plugin) Generate(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	requestData, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: proto.Marshal: %w", core.ErrGenerationFailed, err)
	}

	if len(p.pluginConfig.Command) == 0 {
		return nil, fmt.Errorf("%w: plugin configuration has no command specified", core.ErrInvalidPluginName)
	}

	// 1. Handle per-plugin custom timeout
	if p.pluginConfig.Timeout != "" {
		timeoutDuration, err := time.ParseDuration(p.pluginConfig.Timeout)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid plugin timeout %q: %w", core.ErrGenerationFailed, p.pluginConfig.Timeout, err)
		}

		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeoutDuration)

		defer cancel()
	}

	// 2. Prepare command execution
	//nolint:gosec // The command is validated by ValidateConfig and retrieved from the registry database.
	cmd := exec.CommandContext(ctx, p.pluginConfig.Command[0], p.pluginConfig.Command[1:]...)

	// Clean env, only propagate configured env variables
	cmd.Env = make([]string, 0, len(p.pluginConfig.Env))
	for k, v := range p.pluginConfig.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Stdin setup
	cmd.Stdin = bytes.NewReader(requestData)

	// Process group isolation (Unix-specific pgid setup)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Stdout and stderr pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: cmd.StdoutPipe: %w", core.ErrGenerationFailed, err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: cmd.StderrPipe: %w", core.ErrGenerationFailed, err)
	}

	// Start command
	startErr := cmd.Start()
	if startErr != nil {
		if errors.Is(startErr, exec.ErrNotFound) {
			return nil, fmt.Errorf("%w: plugin binary not found: %w", core.ErrNotFound, startErr)
		}

		if errors.Is(startErr, os.ErrPermission) {
			return nil, fmt.Errorf("%w: permission denied to execute plugin: %w", core.ErrGenerationFailed, startErr)
		}

		return nil, fmt.Errorf("%w: cmd.Start: %w", core.ErrGenerationFailed, startErr)
	}

	// Ensure process group is killed when we exit (e.g. on context timeout / cancellation)
	doneChan := make(chan struct{})
	defer close(doneChan)

	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				// Kill the entire process group (pgid is negative PID)
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-doneChan:
		}
	}()

	// Read outputs concurrently with size limits.
	stdoutData, stderrData, readErr := readPipes(stdoutPipe, stderrPipe, p.maxOutputSize)
	if readErr != nil {
		return nil, readErr
	}

	// Wait for process to exit
	waitErr := cmd.Wait()

	// Check if context was cancelled/timed out
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: context error: %w", core.ErrGenerationFailed, ctx.Err())
	}

	// Process output limits
	if int64(len(stdoutData)) > p.maxOutputSize {
		return nil, fmt.Errorf("%w: plugin execution failed: output limit exceeded (max %d bytes)", core.ErrGenerationFailed, p.maxOutputSize)
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return nil, fmt.Errorf("%w: plugin execution failed: %w, stderr: %s", core.ErrGenerationFailed, waitErr, string(stderrData))
		}

		if strings.Contains(waitErr.Error(), "not found") || strings.Contains(waitErr.Error(), "no such file") {
			return nil, fmt.Errorf("%w: plugin binary not found: %w", core.ErrNotFound, waitErr)
		}

		if strings.Contains(waitErr.Error(), "permission denied") {
			return nil, fmt.Errorf("%w: permission denied to execute plugin: %w", core.ErrGenerationFailed, waitErr)
		}

		return nil, fmt.Errorf("%w: plugin execution failed: %w, stderr: %s", core.ErrGenerationFailed, waitErr, string(stderrData))
	}

	var response pluginpb.CodeGeneratorResponse
	err = proto.Unmarshal(stdoutData, &response)
	if err != nil {
		return nil, fmt.Errorf("%w: proto.Unmarshal: %w", core.ErrGenerationFailed, err)
	}

	return &response, nil
}

// Create implements core.Registry.
func (r *Registry) Create(ctx context.Context, req core.CreatePluginRequest) (*core.PluginInfo, error) {
	validateErr := ValidateConfig(req.Config, r.pluginsDir)
	if validateErr != nil {
		return nil, fmt.Errorf("ValidateConfig: %w", validateErr)
	}

	var plug plugin

	err := r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		return conn.QueryRowxContext(ctx,
			`INSERT INTO plugins (group_name, name, version, config, tags)
                         VALUES ($1, $2, $3, $4, $5)
                         RETURNING id, group_name, name, version, tags, created_at`,
			req.Group, req.Name, req.Version, req.Config, pq.Array(req.Tags),
		).StructScan(&plug)
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, fmt.Errorf("%w: %s/%s:%s", core.ErrAlreadyExists, req.Group, req.Name, req.Version)
		}

		return nil, fmt.Errorf("r.db.NoTxContext(Create): %w", err)
	}

	return &core.PluginInfo{
		ID:        plug.ID,
		Group:     plug.GroupName,
		Name:      plug.Name,
		Version:   plug.Version,
		Tags:      []string(plug.Tags),
		CreatedAt: plug.CreatedAt,
	}, nil
}

// Update implements core.Registry.
func (r *Registry) Update(ctx context.Context, req core.UpdatePluginRequest) (*core.PluginInfo, error) {
	validateErr := ValidateConfig(req.Config, r.pluginsDir)
	if validateErr != nil {
		return nil, fmt.Errorf("ValidateConfig: %w", validateErr)
	}

	var plug plugin

	err := r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		return conn.QueryRowxContext(ctx,
			`UPDATE plugins SET config = $1, tags = $2
			 WHERE group_name = $3 AND name = $4 AND version = $5
			 RETURNING id, group_name, name, version, tags, created_at`,
			req.Config, pq.Array(req.Tags), req.Group, req.Name, req.Version,
		).StructScan(&plug)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s/%s:%s", core.ErrNotFound, req.Group, req.Name, req.Version)
		}

		return nil, fmt.Errorf("r.db.NoTxContext(Update): %w", err)
	}

	return &core.PluginInfo{
		ID:        plug.ID,
		Group:     plug.GroupName,
		Name:      plug.Name,
		Version:   plug.Version,
		Tags:      []string(plug.Tags),
		CreatedAt: plug.CreatedAt,
	}, nil
}

// Delete implements core.Registry.
func (r *Registry) Delete(ctx context.Context, group, name, version string) error {
	err := r.db.NoTxContext(ctx, func(conn *sqlx.DB) error {
		result, err := conn.ExecContext(ctx,
			`DELETE FROM plugins WHERE group_name = $1 AND name = $2 AND version = $3`,
			group, name, version,
		)
		if err != nil {
			return fmt.Errorf("r.db.NoTxContext(Delete): %w", err)
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("result.RowsAffected: %w", err)
		}

		if rows == 0 {
			return fmt.Errorf("%w: %s/%s:%s", core.ErrNotFound, group, name, version)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("db.NoTxContext(Delete): %w", err)
	}

	return nil
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
