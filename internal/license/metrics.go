package license

import "github.com/prometheus/client_golang/prometheus"

// Metrics содержит Prometheus-метрики лицензирования.
type Metrics struct {
	valid         prometheus.Gauge       // {namespace}_license_valid: 1=valid, 0=invalid/absent
	expiryTS      prometheus.Gauge       // {namespace}_license_expiry_timestamp_seconds
	featureDenied *prometheus.CounterVec // {namespace}_license_feature_denied_total{feature}
}

// NewMetrics создаёт и регистрирует метрики лицензирования в реестре.
func NewMetrics(reg *prometheus.Registry, namespace string) *Metrics {
	metricObj := &Metrics{
		valid: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "license_valid",
			Help:      "Whether the license is valid (1) or invalid/absent (0).",
		}),
		expiryTS: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "license_expiry_timestamp_seconds",
			Help:      "Unix timestamp of the license expiration.",
		}),
		featureDenied: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "license_feature_denied_total",
			Help:      "Total number of feature access denials.",
		}, []string{"feature"}),
	}

	if reg != nil {
		reg.MustRegister(metricObj.valid, metricObj.expiryTS, metricObj.featureDenied)
	}

	return metricObj
}
