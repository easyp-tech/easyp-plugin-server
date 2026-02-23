package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/easyp-tech/service/internal/core"
)

// Compile-time interface check.
var _ core.CoreService = (*TracingCore)(nil)

// TracingCore — декоратор core.CoreService, добавляющий span-ы трассировки.
// Проксирует все вызовы в реальный Core.
type TracingCore struct {
	inner  core.CoreService
	tracer trace.Tracer
}

// NewTracingCore creates a new TracingCore decorator wrapping the given CoreService.
func NewTracingCore(inner core.CoreService) *TracingCore {
	return &TracingCore{
		inner:  inner,
		tracer: otel.Tracer("core"),
	}
}

// Generate creates a span "core.Generate" with attribute "plugin.name",
// proxies the call to the inner service, and on error sets span status to Error with RecordError.
func (c *TracingCore) Generate(ctx context.Context, req core.GenerateCodeRequest) (*core.GenerateCodeResponse, error) {
	ctx, span := c.tracer.Start(ctx, "core.Generate",
		trace.WithAttributes(attribute.String("plugin.name", req.PluginName)))
	defer span.End()

	resp, err := c.inner.Generate(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return resp, nil
}

// ListPlugins creates a span "core.ListPlugins",
// proxies the call to the inner service, and on error sets span status to Error with RecordError.
func (c *TracingCore) ListPlugins(ctx context.Context, filter core.PluginFilter) ([]core.PluginInfo, error) {
	ctx, span := c.tracer.Start(ctx, "core.ListPlugins")
	defer span.End()

	result, err := c.inner.ListPlugins(ctx, filter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return result, nil
}
