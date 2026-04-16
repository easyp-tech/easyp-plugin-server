package grpchelper

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"github.com/easyp-tech/service/internal/monitor"
)

// TraceLoggingUnaryServerInterceptor adds trace_id and span_id to the logger in context.
func TraceLoggingUnaryServerInterceptor(baseLog *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = enrichLoggerWithTrace(ctx, baseLog)

		return handler(ctx, req)
	}
}

// TraceLoggingStreamServerInterceptor adds trace_id and span_id to the logger in context.
func TraceLoggingStreamServerInterceptor(baseLog *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := enrichLoggerWithTrace(ss.Context(), baseLog)
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		return handler(srv, wrapped)
	}
}

func enrichLoggerWithTrace(ctx context.Context, baseLog *slog.Logger) context.Context {
	// Always start with baseLog since this interceptor runs first in the chain
	// and its job is to set the logger in context with trace info.
	log := baseLog

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		log = log.With(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	return monitor.WithContext(ctx, log)
}

type wrappedServerStream struct {
	grpc.ServerStream

	ctx context.Context //nolint:containedctx // context is required for gRPC stream wrapper
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
