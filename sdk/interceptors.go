package sdk

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetricsCollector is the interface for collecting SDK call metrics.
type MetricsCollector interface {
	RecordCall(method string, duration time.Duration, code codes.Code)
}

// loggingUnaryInterceptor returns a gRPC unary client interceptor that logs
// the RPC method, call duration, and response status code.
func loggingUnaryInterceptor(logger *slog.Logger) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)
		code := status.Code(err)

		logger.InfoContext(ctx, "gRPC call",
			slog.String("method", method),
			slog.Duration("duration", duration),
			slog.String("code", code.String()),
		)

		return err
	}
}

// metricsUnaryInterceptor returns a gRPC unary client interceptor that records
// call metrics via the provided MetricsCollector.
func metricsUnaryInterceptor(collector MetricsCollector) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)
		code := status.Code(err)

		collector.RecordCall(method, duration, code)

		return err
	}
}
