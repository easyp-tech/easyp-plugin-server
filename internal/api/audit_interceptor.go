package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofrs/uuid/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	generator "github.com/easyp-tech/service/api/generator/v1"
	"github.com/easyp-tech/service/internal/core"
)

// AuditInterceptor creates a gRPC UnaryServerInterceptor for audit logging.
type AuditInterceptor struct {
	entries chan<- core.AuditEntry
	logger  *slog.Logger
}

// NewAuditInterceptor creates a new AuditInterceptor with the given audit
// channel and logger.
func NewAuditInterceptor(entries chan<- core.AuditEntry, logger *slog.Logger) *AuditInterceptor {
	return &AuditInterceptor{entries: entries, logger: logger}
}

// methodToOperationType maps a gRPC full method name to an audit operation type.
func methodToOperationType(fullMethod string) string {
	switch fullMethod {
	case generator.ServiceAPI_GenerateCode_FullMethodName:
		return core.OperationGenerateCode
	case generator.ServiceAPI_Plugins_FullMethodName:
		return core.OperationListPlugins
	case generator.ServiceAPI_CreatePlugin_FullMethodName:
		return core.OperationCreatePlugin
	case generator.ServiceAPI_UpdatePlugin_FullMethodName:
		return core.OperationUpdatePlugin
	case generator.ServiceAPI_DeletePlugin_FullMethodName:
		return core.OperationDeletePlugin
	default:
		return ""
	}
}

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor that records audit entries.
func (a *AuditInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Determine operation type; skip audit for unknown methods.
		opType := methodToOperationType(info.FullMethod)
		if opType == "" {
			return handler(ctx, req)
		}

		// Extract peer address.
		callerAddr := "unknown"
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			callerAddr = p.Addr.String()
		}

		// Call the actual handler and measure duration.
		start := time.Now()
		resp, err := handler(ctx, req)
		durationMs := time.Since(start).Milliseconds()

		// Determine status and error details.
		entryStatus := core.AuditStatusSuccess
		var errorCode, errorMessage string
		if err != nil {
			entryStatus = core.AuditStatusError
			if st, ok := status.FromError(err); ok {
				errorCode = st.Code().String()
				errorMessage = st.Message()
			}
		}

		// Extract metadata from the response.
		metadata := extractMetadata(opType, resp)

		// Extract plugin name from request.
		var pluginName string
		switch r := req.(type) {
		case *generator.GenerateCodeRequest:
			pluginName = r.GetPluginName()
		case *generator.CreatePluginRequest:
			pluginName = r.GetGroup() + "/" + r.GetName() + ":" + r.GetVersion()
		case *generator.UpdatePluginRequest:
			pluginName = r.GetGroup() + "/" + r.GetName() + ":" + r.GetVersion()
		case *generator.DeletePluginRequest:
			pluginName = r.GetGroup() + "/" + r.GetName() + ":" + r.GetVersion()
		}

		// Build the audit entry.
		id, _ := uuid.NewV4()
		entry := core.AuditEntry{
			ID:            id,
			OperationType: opType,
			PluginName:    pluginName,
			CallerAddress: callerAddr,
			Status:        entryStatus,
			ErrorCode:     errorCode,
			ErrorMessage:  errorMessage,
			DurationMs:    durationMs,
			Metadata:      metadata,
			CreatedAt:     time.Now(),
		}

		// Non-blocking send to the audit channel.
		select {
		case a.entries <- entry:
		default:
			a.logger.Warn("audit buffer full, event dropped", "method", info.FullMethod)
		}

		return resp, err
	}
}

// extractMetadata extracts response-specific metadata for the audit entry.
func extractMetadata(opType string, resp any) map[string]any {
	metadata := make(map[string]any)

	switch opType {
	case core.OperationGenerateCode:
		if genResp, ok := resp.(*generator.GenerateCodeResponse); ok && genResp != nil {
			files := genResp.GetCodeGeneratorResponse().GetFile()
			metadata["file_count"] = len(files)
		}
	case core.OperationListPlugins:
		if plugResp, ok := resp.(*generator.PluginsResponse); ok && plugResp != nil {
			metadata["plugin_count"] = len(plugResp.GetPlugins())
		}
	}

	return metadata
}
