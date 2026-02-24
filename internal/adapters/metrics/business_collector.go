package metrics

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const defaultQueryTimeout = 5 * time.Second

// Compile-time check that BusinessMetricsCollector implements prometheus.Collector.
var _ prometheus.Collector = (*BusinessMetricsCollector)(nil)

// BusinessMetricsCollector collects business metrics from PostgreSQL.
// It implements the prometheus.Collector interface.
type BusinessMetricsCollector struct {
	db  *sql.DB
	log *slog.Logger

	pluginsTotal        *prometheus.Desc
	pluginsByGroup      *prometheus.Desc
	auditLogTotal       *prometheus.Desc
	auditLogByOperation *prometheus.Desc
	auditLogByStatus    *prometheus.Desc
	pluginVersionsCount *prometheus.Desc
	auditLogLast24h     *prometheus.Desc
}

// NewBusinessMetricsCollector creates a new BusinessMetricsCollector.
func NewBusinessMetricsCollector(db *sql.DB, namespace string, log *slog.Logger) *BusinessMetricsCollector {
	return &BusinessMetricsCollector{
		db:  db,
		log: log,
		pluginsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "business", "plugins_total"),
			"Total number of registered plugins.",
			nil, nil,
		),
		pluginsByGroup: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "business", "plugins_by_group"),
			"Number of plugins per group.",
			[]string{"group"}, nil,
		),
		auditLogTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "business", "audit_log_total"),
			"Total number of audit log entries.",
			nil, nil,
		),
		auditLogByOperation: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "business", "audit_log_by_operation"),
			"Number of audit log entries per operation type.",
			[]string{"operation"}, nil,
		),
		auditLogByStatus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "business", "audit_log_by_status"),
			"Number of audit log entries per status.",
			[]string{"status"}, nil,
		),
		pluginVersionsCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "business", "plugin_versions_count"),
			"Number of unique versions per plugin.",
			[]string{"group", "name"}, nil,
		),
		auditLogLast24h: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "business", "audit_log_last_24h"),
			"Number of audit log entries in the last 24 hours.",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *BusinessMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.pluginsTotal
	ch <- c.pluginsByGroup
	ch <- c.auditLogTotal
	ch <- c.auditLogByOperation
	ch <- c.auditLogByStatus
	ch <- c.pluginVersionsCount
	ch <- c.auditLogLast24h
}

// Collect implements prometheus.Collector.
func (c *BusinessMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	// Scalar metrics
	c.collectScalar(ch, c.pluginsTotal, "plugins_total",
		"SELECT count(*) FROM plugins")
	c.collectScalar(ch, c.auditLogTotal, "audit_log_total",
		"SELECT count(*) FROM audit_log")
	c.collectScalar(ch, c.auditLogLast24h, "audit_log_last_24h",
		"SELECT count(*) FROM audit_log WHERE created_at > now() - interval '24 hours'")

	// Grouped metrics
	c.collectGrouped(ch, c.pluginsByGroup, "plugins_by_group",
		"SELECT group_name, count(*) FROM plugins GROUP BY group_name")
	c.collectGrouped(ch, c.auditLogByOperation, "audit_log_by_operation",
		"SELECT operation_type, count(*) FROM audit_log GROUP BY operation_type")
	c.collectGrouped(ch, c.auditLogByStatus, "audit_log_by_status",
		"SELECT status, count(*) FROM audit_log GROUP BY status")
	c.collectGrouped2(ch, c.pluginVersionsCount, "plugin_versions_count",
		"SELECT group_name, name, count(DISTINCT version) FROM plugins GROUP BY group_name, name")
}

func (c *BusinessMetricsCollector) collectScalar(ch chan<- prometheus.Metric, desc *prometheus.Desc, name, query string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	var count int64
	if err := c.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		c.log.Error("failed to collect metric", "metric", name, "error", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(count))
}

func (c *BusinessMetricsCollector) collectGrouped(ch chan<- prometheus.Metric, desc *prometheus.Desc, name, query string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		c.log.Error("failed to collect metric", "metric", name, "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var labelValue string
		var count int64
		if err := rows.Scan(&labelValue, &count); err != nil {
			c.log.Error("failed to collect metric", "metric", name, "error", err)
			return
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(count), labelValue)
	}

	if err := rows.Err(); err != nil {
		c.log.Error("failed to collect metric", "metric", name, "error", err)
	}
}

func (c *BusinessMetricsCollector) collectGrouped2(ch chan<- prometheus.Metric, desc *prometheus.Desc, name, query string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		c.log.Error("failed to collect metric", "metric", name, "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var label1, label2 string
		var count int64
		if err := rows.Scan(&label1, &label2, &count); err != nil {
			c.log.Error("failed to collect metric", "metric", name, "error", err)
			return
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(count), label1, label2)
	}

	if err := rows.Err(); err != nil {
		c.log.Error("failed to collect metric", "metric", name, "error", err)
	}
}
