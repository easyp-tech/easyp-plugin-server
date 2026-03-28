package api

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/easyp-tech/service/internal/license"
)

// LicenseInterceptor проверяет лицензию на уровне gRPC-запроса.
type LicenseInterceptor struct {
	gate   *license.FeatureGate
	logger *slog.Logger
	// methodFeatures маппит gRPC full method name → Feature.
	// Методы, отсутствующие в маппинге, считаются Community и пропускаются.
	methodFeatures map[string]license.Feature
}

// NewLicenseInterceptor создаёт интерсептор с маппингом method → Feature.
// Текущие методы (GenerateCode, Plugins) — Community, маппинг пуст.
// Маппинг будет расширяться по мере добавления Enterprise-методов.
func NewLicenseInterceptor(gate *license.FeatureGate, logger *slog.Logger) *LicenseInterceptor {
	return &LicenseInterceptor{
		gate:           gate,
		logger:         logger,
		methodFeatures: make(map[string]license.Feature),
	}
}

// checkFeature проверяет, разрешён ли вызов метода текущей лицензией.
// Возвращает nil, если метод не требует проверки (Community) или функция разрешена.
func (li *LicenseInterceptor) checkFeature(fullMethod string) error {
	feature, ok := li.methodFeatures[fullMethod]
	if !ok {
		// Community method — no license check needed.
		return nil
	}

	if li.gate.Enabled(feature) {
		return nil
	}

	li.logger.Warn("enterprise feature denied",
		"method", fullMethod,
		"feature", feature.String(),
	)

	return status.Errorf(codes.PermissionDenied, "feature %s requires enterprise license", feature)
}

// UnaryServerInterceptor возвращает grpc.UnaryServerInterceptor.
func (li *LicenseInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := li.checkFeature(info.FullMethod); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// wrappedStream wraps grpc.ServerStream to preserve context.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context.
func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// StreamServerInterceptor возвращает grpc.StreamServerInterceptor.
func (li *LicenseInterceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := li.checkFeature(info.FullMethod); err != nil {
			return err
		}

		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ss.Context()})
	}
}

// RegisterMethodFeature registers a gRPC method as requiring a specific license feature.
// This is used to extend the interceptor when new Enterprise methods are added.
func (li *LicenseInterceptor) RegisterMethodFeature(fullMethod string, feature license.Feature) {
	li.methodFeatures[fullMethod] = feature
	li.logger.Info("registered license check",
		"method", fullMethod,
		"feature", fmt.Sprintf("%s", feature),
	)
}
