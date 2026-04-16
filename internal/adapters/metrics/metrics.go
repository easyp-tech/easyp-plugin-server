// Package metrics provides a metrics adapter for the EasyP plugin server.
package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/easyp-tech/service/internal/core"
)

var _ core.Metrics = Metrics{}

// Metrics is the metrics adapter for the EasyP plugin server.
type Metrics struct {
	generated          *prometheus.CounterVec
	generationDuration *prometheus.HistogramVec
	generationErrors   *prometheus.CounterVec
	generationRetries  *prometheus.CounterVec
}

// New creates and returns a new Metrics adapter.
func New(reg *prometheus.Registry, namespace string) *Metrics {
	metrics := &Metrics{
		generated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "generated_plugin_code_total",
				Help:      "Total number of generated code requests by plugin.",
			},
			[]string{"plugin"},
		),
		generationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "generation_duration_seconds",
				Help:      "Duration of code generation in seconds.",
			},
			[]string{"plugin"},
		),
		generationErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "generation_errors_total",
				Help:      "Total number of generation errors by plugin and error type.",
			},
			[]string{"plugin", "error_type"},
		),
		generationRetries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "generation_retries_total",
				Help:      "Total number of generation retries by plugin.",
			},
			[]string{"plugin"},
		),
	}

	reg.MustRegister(metrics.generated)
	reg.MustRegister(metrics.generationDuration)
	reg.MustRegister(metrics.generationErrors)
	reg.MustRegister(metrics.generationRetries)

	return metrics
}

// GenerateCode implements the core.Metrics interface.
func (m Metrics) GenerateCode(_ context.Context, info core.PluginInfo) error {
	plugin := info.Group + "/" + info.Name + ":" + info.Version
	m.generated.WithLabelValues(plugin).Inc()

	return nil
}

// ObserveGenerationDuration records the duration of a code generation attempt.
func (m Metrics) ObserveGenerationDuration(_ context.Context, pluginName string, duration time.Duration) {
	m.generationDuration.WithLabelValues(pluginName).Observe(duration.Seconds())
}

// IncGenerationErrors increments the generation error counter.
func (m Metrics) IncGenerationErrors(_ context.Context, pluginName string, errorType string) {
	m.generationErrors.WithLabelValues(pluginName, errorType).Inc()
}

// IncGenerationRetries increments the generation retry counter.
func (m Metrics) IncGenerationRetries(_ context.Context, pluginName string) {
	m.generationRetries.WithLabelValues(pluginName).Inc()
}
