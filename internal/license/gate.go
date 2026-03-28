package license

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
// 1. Если feature невалиден → false
// 2. Получить claims из LicenseManager (потокобезопасно)
// 3. Если tier == Enterprise → true
// 4. Если feature.IsEnterprise() → инкремент метрики denied, false
// 5. Проверить наличие feature в claims.Features → результат
func (fg *FeatureGate) Enabled(feature Feature) bool {
	// Step 1: Invalid feature → false.
	if !feature.Valid() {
		return false
	}

	// Step 2: Get current claims (thread-safe).
	claims := fg.manager.Claims()

	// Step 3: Enterprise tier → all features enabled.
	if claims.Tier == TierEnterprise {
		return true
	}

	// Step 4: Enterprise-only feature in non-Enterprise mode → deny + metric.
	if feature.IsEnterprise() {
		fg.metrics.featureDenied.WithLabelValues(feature.String()).Inc()
		return false
	}

	// Step 5: Check if feature is in claims.Features.
	for _, f := range claims.Features {
		if f == feature {
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
