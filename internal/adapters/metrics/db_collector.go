package metrics

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

// DBCollector collects database connection pool metrics from sql.DBStats.
// It implements the prometheus.Collector interface.
type DBCollector struct {
	db *sql.DB

	openConnections *prometheus.Desc
	idleConnections *prometheus.Desc
	waitCount       *prometheus.Desc
	waitDuration    *prometheus.Desc
}

// NewDBCollector creates a new DBCollector.
func NewDBCollector(db *sql.DB, namespace string) *DBCollector {
	return &DBCollector{
		db: db,
		openConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "db_open_connections"),
			"Number of open connections to the database.",
			nil, nil,
		),
		idleConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "db_idle_connections"),
			"Number of idle connections in the pool.",
			nil, nil,
		),
		waitCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "db_wait_count_total"),
			"Total number of connections waited for.",
			nil, nil,
		),
		waitDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "db_wait_duration_seconds_total"),
			"Total time blocked waiting for a new connection.",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *DBCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.openConnections
	ch <- c.idleConnections
	ch <- c.waitCount
	ch <- c.waitDuration
}

// Collect implements prometheus.Collector.
func (c *DBCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.db.Stats()

	ch <- prometheus.MustNewConstMetric(c.openConnections, prometheus.GaugeValue, float64(stats.OpenConnections))
	ch <- prometheus.MustNewConstMetric(c.idleConnections, prometheus.GaugeValue, float64(stats.Idle))
	ch <- prometheus.MustNewConstMetric(c.waitCount, prometheus.CounterValue, float64(stats.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, stats.WaitDuration.Seconds())
}
