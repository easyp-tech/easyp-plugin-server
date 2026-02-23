// Package core contains the domain types and interfaces for the plugin server.
package core

import (
	"context"
	"errors"
	"time"

	"github.com/gofrs/uuid/v5"
	"google.golang.org/protobuf/types/pluginpb"
)

// Errors.
var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidPluginName = errors.New("invalid plugin name")
	ErrGenerationFailed  = errors.New("code generation failed")
	ErrServerOverloaded  = errors.New("server overloaded")
	ErrShuttingDown      = errors.New("server shutting down")
)

// Audit operation types.
const (
	OperationGenerateCode = "GENERATE_CODE"
	OperationListPlugins  = "LIST_PLUGINS"
)

// Audit statuses.
const (
	AuditStatusSuccess = "success"
	AuditStatusError   = "error"
)

type (
	// Metrics defines the interface for collecting metrics about core operations.
	Metrics interface {
		// GenerateCode records metrics for a code generation request.
		// The pluginName parameter identifies which plugin was used (e.g., "grpc/go:v1.36.9").
		GenerateCode(ctx context.Context, info PluginInfo) error
		// ObserveGenerationDuration records the duration of a code generation attempt.
		ObserveGenerationDuration(ctx context.Context, pluginName string, duration time.Duration)
		// IncGenerationErrors increments the generation error counter with the given error type.
		IncGenerationErrors(ctx context.Context, pluginName string, errorType string)
		// IncGenerationRetries increments the generation retry counter.
		IncGenerationRetries(ctx context.Context, pluginName string)
	}

	// Registry provides access to available plugins.
	Registry interface {
		// Get retrieves a plugin by its identifier.
		// The pluginName parameter specifies the plugin to retrieve (e.g., "protobuf/go:v1.36.9").
		// Returns an error if the plugin is not found or cannot be loaded.
		Get(ctx context.Context, pluginGroup, pluginName, pluginVersion string) (Plugin, error)
		// List retrieves a list of plugins matching the filter.
		List(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
	}

	// Plugin represents a code generator plugin that processes protobuf definitions.
	Plugin interface {
		// Generate processes a code generation request and produces generated code.
		// Takes a protobuf CodeGeneratorRequest and returns a CodeGeneratorResponse
		// containing the generated files or an error if generation fails.
		Generate(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error)
		// Info retrieves information about a plugin by its identifier.
		Info(ctx context.Context) *PluginInfo
	}

	// GenerateCodeRequest represents an incoming request to generate code using a specific plugin.
	GenerateCodeRequest struct {
		// PluginName identifies the plugin to use for code generation.
		// Format: "<language>:v<version>" (e.g., "go:v1.36.9", "python:v3.20.0").
		PluginName string
		// Payload contains the protobuf code generation request with source files and parameters.
		Payload *pluginpb.CodeGeneratorRequest
	}

	// GenerateCodeResponse wraps the response from a code generation operation.
	GenerateCodeResponse struct {
		// Payload contains the protobuf code generation response with generated files.
		Payload *pluginpb.CodeGeneratorResponse
	}

	// PluginInfo represents information about a plugin.
	PluginInfo struct {
		ID        uuid.UUID
		Group     string
		Name      string
		Version   string
		Tags      []string
		CreatedAt time.Time
	}

	// PluginFilter represents a filter for listing plugins.
	PluginFilter struct {
		Group   string
		Name    string
		Version string
		Tags    []string
	}

	// AuditEntry представляет одну запись аудит-журнала.
	AuditEntry struct {
		ID            uuid.UUID
		OperationType string
		PluginName    string
		CallerAddress string
		Status        string
		ErrorCode     string
		ErrorMessage  string
		DurationMs    int64
		Metadata      map[string]any
		CreatedAt     time.Time
	}

	// AuditLog определяет интерфейс для записи аудит-событий.
	AuditLog interface {
		// Save сохраняет аудит-запись в хранилище.
		Save(ctx context.Context, entry AuditEntry) error
	}

	// CoreService defines the business logic interface used by the API layer.
	CoreService interface {
		Generate(ctx context.Context, req GenerateCodeRequest) (*GenerateCodeResponse, error)
		ListPlugins(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
	}
)
