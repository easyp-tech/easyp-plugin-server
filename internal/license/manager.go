package license

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/easyp-tech/service/internal/core"
)

const defaultCacheTTL = 5 * time.Minute

// Config holds license manager configuration.
type Config struct {
	// CacheTTL is the interval between license refresh calls.
	// Defaults to 5 minutes when zero or negative.
	CacheTTL time.Duration `env:"LICENSE_CACHE_TTL" yaml:"cache_ttl"`
}

// Manager fetches, caches, and periodically refreshes license claims.
type Manager struct {
	mu      sync.RWMutex
	claims  core.LicenseClaims
	client  core.LicenseClient
	cfg     Config
	logger  *slog.Logger
	metrics *Metrics
}

// NewManager creates a Manager backed by the given LicenseClient.
// It performs an initial license fetch; on failure it falls back to community mode.
func NewManager(
	ctx context.Context,
	client core.LicenseClient,
	cfg Config,
	logger *slog.Logger,
	reg *prometheus.Registry,
	namespace string,
) (*Manager, error) {
	if client == nil {
		return nil, ErrNoClient
	}

	metrics := NewMetrics(reg, namespace)

	lm := &Manager{
		claims:  core.CommunityLicenseClaims(),
		client:  client,
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
	}

	if lm.cfg.CacheTTL <= 0 {
		lm.cfg.CacheTTL = defaultCacheTTL
	}

	// Perform initial fetch so claims are populated before the first request.
	lm.refresh(ctx)

	return lm, nil
}

// Claims returns the cached LicenseClaims. Safe for concurrent use.
func (lm *Manager) Claims() core.LicenseClaims {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return lm.claims
}

// StartRefreshWatcher starts a background goroutine that refreshes license claims
// on every CacheTTL tick. It stops when ctx is cancelled.
func (lm *Manager) StartRefreshWatcher(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(lm.cfg.CacheTTL)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				lm.refresh(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Metrics returns the Prometheus metrics collector (used by FeatureGate).
func (lm *Manager) Metrics() *Metrics {
	return lm.metrics
}

// refresh calls ValidateLicense and updates the cached claims.
// On error it keeps the previous claims (graceful fallback).
func (lm *Manager) refresh(ctx context.Context) {
	claims, err := lm.client.ValidateLicense(ctx)
	if err != nil {
		lm.logger.Warn("failed to validate license, keeping previous claims", "error", err)
		lm.updateMetrics(false)

		return
	}

	lm.mu.Lock()
	lm.claims = claims
	lm.mu.Unlock()

	lm.logger.Info("license refreshed",
		"tier", claims.Tier,
		"max_workers", claims.MaxWorkers,
		"max_plugins", claims.MaxPlugins,
		"features_count", len(claims.Features),
	)

	lm.updateMetrics(true)
}

// updateMetrics sets the Prometheus license validity gauge.
func (lm *Manager) updateMetrics(valid bool) {
	if valid {
		lm.metrics.valid.Set(1)
	} else {
		lm.metrics.valid.Set(0)
	}
}
