package license

// feature определяет функцию сервиса как приватную типизированную константу.
// Публичные константы Feature определены в пакете core.
type feature int

const (
	featureCodeGeneration feature = iota // Базовая генерация кода
	featurePluginListing                 // Листинг плагинов
	featureMCPServerTools                // MCP server tools
	featureRateLimiting                  // Rate limiting
	featurePluginCRUD                    // CRUD операции с плагинами
	// Enterprise-only features
	featureMultiTenancy    // Мультитенантность
	featureResponseCaching // Кэширование ответов
	featureAudit           // Аудит

	featureCount // sentinel для валидации
)

// featureNames содержит строковые представления feature для метрик и логирования.
var featureNames = [featureCount]string{
	featureCodeGeneration:  "code_generation",
	featurePluginListing:   "plugin_listing",
	featureMCPServerTools:  "mcp_server_tools",
	featureRateLimiting:    "rate_limiting",
	featurePluginCRUD:      "plugin_crud",
	featureMultiTenancy:    "multi_tenancy",
	featureResponseCaching: "response_caching",
	featureAudit:           "audit",
}

// String возвращает строковое представление feature для метрик и логирования.
func (f feature) String() string {
	if !f.Valid() {
		return "unknown"
	}
	return featureNames[f]
}

// IsEnterprise возвращает true, если функция доступна только в Enterprise.
func (f feature) IsEnterprise() bool {
	return f == featureMultiTenancy || f == featureResponseCaching || f == featureAudit
}

// Valid возвращает true, если значение feature определено.
func (f feature) Valid() bool {
	return f >= 0 && f < featureCount
}
