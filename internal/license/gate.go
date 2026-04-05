package license

import "github.com/easyp-tech/service/internal/core"

// FeatureGate предоставляет проверку доступности функций на основе текущей лицензии.
type FeatureGate struct {
	manager *LicenseManager
	metrics *LicenseMetrics
}

// NewFeatureGate создаёт FeatureGate, привязанный к LicenseManager.
func NewFeatureGate(manager *LicenseManager) *FeatureGate {
	return &FeatureGate{
		manager: manager,
		metrics: manager.Metrics(),
	}
}

// Enabled возвращает true, если функция разрешена текущей лицензией.
// Для невалидных значений Feature возвращает false.
//
// Алгоритм:
// 1. Конвертировать core.Feature в приватный feature; если невалиден → false
// 2. Получить claims из LicenseManager (потокобезопасно)
// 3. Если tier == Enterprise → true
// 4. Если feature.IsEnterprise() → инкремент метрики denied, false
// 5. Проверить наличие feature в claims.Features → результат
func (fg *FeatureGate) Enabled(f core.Feature) bool {
	// Step 1: Convert and validate.
	lf := feature(f)
	if !lf.Valid() {
		return false
	}

	// Step 2: Get current claims (thread-safe).
	claims := fg.manager.Claims()

	// Step 3: Enterprise tier → all features enabled.
	if claims.Tier == TierEnterprise {
		return true
	}

	// Step 4: Enterprise-only feature in non-Enterprise mode → deny + metric.
	if lf.IsEnterprise() {
		fg.metrics.featureDenied.WithLabelValues(f.String()).Inc()
		return false
	}

	// Step 5: Check if feature is in claims.Features.
	for _, cf := range claims.Features {
		if cf == lf {
			return true
		}
	}

	return false
}

// MaxWorkers возвращает лимит воркеров из текущей лицензии.
func (fg *FeatureGate) MaxWorkers() int {
	return fg.manager.Claims().MaxWorkers
}

// MaxPlugins возвращает лимит плагинов из текущей лицензии.
// -1 означает отсутствие ограничения.
func (fg *FeatureGate) MaxPlugins() int {
	return fg.manager.Claims().MaxPlugins
}
