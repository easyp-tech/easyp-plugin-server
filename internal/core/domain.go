// Package core contains the domain types and interfaces for the plugin server.
package core

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/pluginpb"
)

// Errors.
var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidPluginName = errors.New("invalid plugin name")
	ErrGenerationFailed  = errors.New("code generation failed")
)

type (
	// Metrics defines the interface for collecting metrics about core operations.
	Metrics interface {
		// GenerateCode records metrics for a code generation request.
		// The pluginName parameter identifies which plugin was used (e.g., "go:v1.36.9").
		GenerateCode(ctx context.Context, pluginName string) error
	}

	// Registry provides access to available plugins.
	Registry interface {
		// Get retrieves a plugin by its identifier.
		// The pluginName parameter specifies the plugin to retrieve (e.g., "go:v1.36.9").
		// Returns an error if the plugin is not found or cannot be loaded.
		Get(ctx context.Context, pluginName string) (Plugin, error)
	}

	// Plugin represents a code generator plugin that processes protobuf definitions.
	Plugin interface {
		// Generate processes a code generation request and produces generated code.
		// Takes a protobuf CodeGeneratorRequest and returns a CodeGeneratorResponse
		// containing the generated files or an error if generation fails.
		Generate(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error)
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
)
