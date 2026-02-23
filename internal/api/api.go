// Package api implements the API server for the application.
package api

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	generator "github.com/easyp-tech/service/api/generator/v1"
	"github.com/easyp-tech/service/internal/core"
)

var _ generator.ServiceAPIServer = (*API)(nil)

// API provides the API server implementation.
type API struct {
	app core.CoreService
}

// New registers the API handler on the given gRPC server and sets the health
// serving status. The gRPC server and health server are expected to be created
// externally (e.g. via grpchelper.NewServer).
func New(grpcSrv *grpc.Server, healthSrv *health.Server, applications core.CoreService) {
	healthSrv.SetServingStatus(generator.ServiceAPI_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)

	api := &API{
		app: applications,
	}
	generator.RegisterServiceAPIServer(grpcSrv, api)
}

// GenerateCode implements generator.PluginGeneratorServiceServer.
func (api *API) GenerateCode(ctx context.Context, request *generator.GenerateCodeRequest) (*generator.GenerateCodeResponse, error) {
	resp, err := api.app.Generate(ctx, core.GenerateCodeRequest{
		PluginName: request.PluginName,
		Payload:    request.CodeGeneratorRequest,
	})
	if err != nil {
		return nil, fmt.Errorf("api.app.Generate: %w", err)
	}

	return &generator.GenerateCodeResponse{
		CodeGeneratorResponse: resp.Payload,
	}, nil
}

// Plugins implements generator.ServiceAPIServer.
func (api *API) Plugins(ctx context.Context, _ *generator.PluginsRequest) (*generator.PluginsResponse, error) {
	plugins, err := api.app.ListPlugins(ctx, core.PluginFilter{})
	if err != nil {
		return nil, fmt.Errorf("api.app.ListPlugins: %w", err)
	}

	response := &generator.PluginsResponse{
		Plugins: make([]*generator.PluginInfo, 0, len(plugins)),
	}

	for _, p := range plugins {
		response.Plugins = append(response.Plugins, &generator.PluginInfo{
			Id:        p.ID.String(),
			Group:     p.Group,
			Name:      p.Name,
			Version:   p.Version,
			Tags:      p.Tags,
			CreatedAt: timestamppb.New(p.CreatedAt),
		})
	}

	return response, nil
}

// ErrorToStatus converts an application error to a gRPC status.
// Compatible with grpchelper.GRPCCodesConverterHandler.
func ErrorToStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}

	code := codes.Internal
	switch {
	case errors.Is(err, core.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, core.ErrInvalidPluginName):
		code = codes.InvalidArgument
	case errors.Is(err, core.ErrGenerationFailed):
		code = codes.Internal
	case errors.Is(err, core.ErrServerOverloaded):
		code = codes.ResourceExhausted
	case errors.Is(err, core.ErrShuttingDown):
		code = codes.Unavailable
	case errors.Is(err, context.DeadlineExceeded):
		code = codes.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		code = codes.Canceled
	}

	return status.New(code, err.Error())
}
