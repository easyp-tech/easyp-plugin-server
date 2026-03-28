package license

// Feature определяет функцию сервиса как типизированную константу.
type Feature int

const (
	FeatureCodeGeneration Feature = iota // Базовая генерация кода
	FeaturePluginListing                 // Листинг плагинов
	FeatureMCPServerTools                // MCP server tools
	FeatureRateLimiting                  // Rate limiting
	FeaturePluginCRUD                    // CRUD операции с плагинами
	// Enterprise-only features
	FeatureMultiTenancy    // Мультитенантность
	FeatureResponseCaching // Кэширование ответов
	FeatureAudit           // Аудит

	featureCount // sentinel для валидации
)

// featureNames содержит строковые представления Feature для метрик и логирования.
var featureNames = [featureCount]string{
	FeatureCodeGeneration:  "code_generation",
	FeaturePluginListing:   "plugin_listing",
	FeatureMCPServerTools:  "mcp_server_tools",
	FeatureRateLimiting:    "rate_limiting",
	FeaturePluginCRUD:      "plugin_crud",
	FeatureMultiTenancy:    "multi_tenancy",
	FeatureResponseCaching: "response_caching",
	FeatureAudit:           "audit",
}

// String возвращает строковое представление Feature для метрик и логирования.
func (f Feature) String() string {
	if !f.Valid() {
		return "unknown"
	}
	return featureNames[f]
}

// IsEnterprise возвращает true, если функция доступна только в Enterprise.
func (f Feature) IsEnterprise() bool {
	return f == FeatureMultiTenancy || f == FeatureResponseCaching || f == FeatureAudit
}

// Valid возвращает true, если значение Feature определено.
func (f Feature) Valid() bool {
	return f >= 0 && f < featureCount
}
