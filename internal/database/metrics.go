package database

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/easyp-tech/service/internal/database/internal"
)

// MetricCollector is a helper for easy collecting metrics for every handler.
type MetricCollector interface {
	// Collecting collects Metrics information for handlers.
	Collecting(method string, f func() error) func() error
}

const (
	labelFunc = "func" // Value: caller's func/method name.
)

var _ MetricCollector = Metrics{}

// Metrics contains general metrics for DAL methods.
type Metrics struct {
	callErrTotal *prometheus.CounterVec
	callDuration *prometheus.HistogramVec
}

// NewMetrics registers and returns common DAL metrics used by all
// services (namespace).
func NewMetrics(reg *prometheus.Registry, namespace, subsystem string, methodsFrom any) Metrics {
	var metric Metrics
	metric.callErrTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "errors_total",
			Help:      "Amount of DAL errors.",
		},
		[]string{labelFunc},
	)
	reg.MustRegister(metric.callErrTotal)
	metric.callDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "call_duration_seconds",
			Help:      "DAL call latency.",
		},
		[]string{labelFunc},
	)
	reg.MustRegister(metric.callDuration)

	for _, methodName := range internal.MethodsOf(methodsFrom) {
		l := prometheus.Labels{
			labelFunc: methodName,
		}
		metric.callErrTotal.With(l)
		metric.callDuration.With(l)
	}

	return metric
}

// Collecting implements MetricCollector.
func (m Metrics) Collecting(method string, fn func() error) func() error {
	return func() error {
		start := time.Now()
		labels := prometheus.Labels{labelFunc: method}
		var result error
		defer func() {
			m.callDuration.With(labels).Observe(time.Since(start).Seconds())
			if result != nil {
				m.callErrTotal.With(labels).Inc()
			} else if r := recover(); r != nil {
				m.callErrTotal.With(labels).Inc()
				panic(r)
			}
		}()

		result = fn()

		return result
	}
}

var _ MetricCollector = NoMetric{}

// NoMetric if you want to turn off metrics.
type NoMetric struct{}

// Collecting implements MetricCollector.
func (n NoMetric) Collecting(_ string, f func() error) func() error {
	return func() error { return f() }
}
