package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/grafana/pyroscope-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init initialises TracerProvider, MeterProvider and returns a composite
// shutdown function together with a configured *slog.Logger.
//
// If OTLP exporter creation fails the error is logged as a warning and the
// service continues to operate without telemetry export.
func Init(ctx context.Context, cfg Config, baseHandler slog.Handler) (shutdownFn func(context.Context) error, logger *slog.Logger, err error) {
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion("dev"),
	)

	// --- Trace exporter ---------------------------------------------------
	traceExp, traceErr := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if traceErr != nil {
		slog.New(baseHandler).Warn("failed to create OTLP trace exporter, continuing without trace export", "error", traceErr)
	}

	// --- Metric exporter --------------------------------------------------
	metricExp, metricErr := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if metricErr != nil {
		slog.New(baseHandler).Warn("failed to create OTLP metric exporter, continuing without metric export", "error", metricErr)
	}

	// --- TracerProvider ---------------------------------------------------
	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}
	if traceExp != nil {
		tpOpts = append(tpOpts, sdktrace.WithBatcher(traceExp))
	}
	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)

	// --- MeterProvider ----------------------------------------------------
	mpOpts := []metric.Option{
		metric.WithResource(res),
	}
	if metricExp != nil {
		mpOpts = append(mpOpts, metric.WithReader(
			metric.NewPeriodicReader(metricExp, metric.WithInterval(15*time.Second)),
		))
	}
	mp := metric.NewMeterProvider(mpOpts...)
	otel.SetMeterProvider(mp)

	// --- W3C TraceContext propagation -------------------------------------
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// --- Logger -----------------------------------------------------------
	logger = slog.New(NewTraceHandler(baseHandler))

	// --- Pyroscope profiling -----------------------------------------------
	profiler, pyroErr := pyroscope.Start(pyroscope.Config{
		ApplicationName: cfg.ServiceName,
		ServerAddress:   cfg.PyroscopeEndpoint,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
		},
	})
	if pyroErr != nil {
		slog.New(baseHandler).Warn("failed to start Pyroscope profiler, continuing without profiling", "error", pyroErr)
	}

	// --- Composite shutdown -----------------------------------------------
	shutdownFn = func(ctx context.Context) error {
		var pyroStopErr error
		if profiler != nil {
			pyroStopErr = profiler.Stop()
		}
		return errors.Join(
			tp.Shutdown(ctx),
			mp.Shutdown(ctx),
			pyroStopErr,
		)
	}

	return shutdownFn, logger, nil
}
