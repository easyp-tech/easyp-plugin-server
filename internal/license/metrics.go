package license

import "github.com/prometheus/client_golang/prometheus"

// LicenseMetrics содержит Prometheus-метрики лицензирования.
type LicenseMetrics struct {
	valid         prometheus.Gauge       // {namespace}_license_valid: 1=valid, 0=invalid/absent
	expiryTS      prometheus.Gauge       // {namespace}_license_expiry_timestamp_seconds
	featureDenied *prometheus.CounterVec // {namespace}_license_feature_denied_total{feature}
}

// NewLicenseMetrics создаёт и регистрирует метрики лицензирования в реестре.
func NewLicenseMetrics(reg *prometheus.Registry, namespace string) *LicenseMetrics {
	m := &LicenseMetrics{
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
		reg.MustRegister(m.valid, m.expiryTS, m.featureDenied)
	}

	return m
}
