package grpchelper

import (
	"context"
	"fmt"
	"log/slog"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Dial creates a gRPC client connection to the given target.
func Dial(_ context.Context,
	addr string,
	log *slog.Logger,
	metrics *grpc_prometheus.ClientMetrics,
	extraUnary []grpc.UnaryClientInterceptor,
	extraStream []grpc.StreamClientInterceptor,
	extraDialOption []grpc.DialOption,
) (*grpc.ClientConn, error) {
	loggingOpts := []logging.Option{
		logging.WithLogOnEvents(
			logging.StartCall,
			logging.FinishCall,
			logging.PayloadReceived,
			logging.PayloadSent,
		),
	}

	dialOptions := append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepaliveTime,
			Timeout:             keepaliveTimeout,
			PermitWithoutStream: true,
		}),
	}, extraDialOption...)

	unaryInterceptor := append([]grpc.UnaryClientInterceptor{
		metrics.UnaryClientInterceptor(),
		logging.UnaryClientInterceptor(interceptorLogger(log)),
		grpc_validator.UnaryClientInterceptor(),
	}, extraUnary...)

	streamInterceptor := append([]grpc.StreamClientInterceptor{
		metrics.StreamClientInterceptor(),
		logging.StreamClientInterceptor(interceptorLogger(log), loggingOpts...),
	}, extraStream...)

	// Add OTEL tracing interceptors with custom span naming
	otelClientOpts := []otelgrpc.Option{
		otelgrpc.WithSpanAttributes(attribute.String("span.layer", "grpc-client")),
	}

	dialOptions = append(dialOptions,
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(otelClientOpts...)),
		grpc.WithChainUnaryInterceptor(
			unaryInterceptor...,
		),
		grpc.WithChainStreamInterceptor(
			streamInterceptor...,
		),
	)

	conn, err := grpc.NewClient(addr, dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("grpc.NewClient: %w", err)
	}

	return conn, nil
}
