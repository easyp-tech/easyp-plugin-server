# Документ требований: Бизнес-метрики PostgreSQL для Grafana

## Введение

Функциональность добавляет сбор бизнес-метрик из PostgreSQL и их экспорт в Prometheus для отображения на дашборде Grafana. Метрики включают количество плагинов, записей аудит-лога, статистику по группам плагинов, распределение операций и ошибок. Это позволяет команде отслеживать состояние и рост сервиса в реальном времени.

## Глоссарий

- **Business_Metrics_Collector**: Prometheus-коллектор, который выполняет SQL-запросы к PostgreSQL и экспортирует бизнес-метрики
- **Dashboard**: JSON-файл дашборда Grafana, отображающий бизнес-метрики из Prometheus
- **Plugins_Table**: Таблица `plugins` в PostgreSQL, содержащая зарегистрированные плагины
- **Audit_Log_Table**: Таблица `audit_log` в PostgreSQL, содержащая записи аудит-журнала
- **Prometheus_Registry**: Реестр Prometheus, в который регистрируются коллекторы метрик

## Требования

### Требование 1: Сбор метрики общего количества плагинов

**User Story:** Как оператор сервиса, я хочу видеть общее количество зарегистрированных плагинов, чтобы отслеживать рост каталога плагинов.

#### Критерии приёмки

1. WHEN Prometheus выполняет scrape, THE Business_Metrics_Collector SHALL выполнить SQL-запрос `SELECT count(*) FROM plugins` и экспортировать результат как gauge-метрику `easyp_business_plugins_total`
2. IF запрос к Plugins_Table завершается ошибкой, THEN THE Business_Metrics_Collector SHALL записать ошибку в лог и не экспортировать метрику для данного scrape-цикла

### Требование 2: Сбор метрики количества плагинов по группам

**User Story:** Как оператор сервиса, я хочу видеть количество плагинов в разрезе групп (group_name), чтобы понимать распределение плагинов по экосистемам.

#### Критерии приёмки

1. WHEN Prometheus выполняет scrape, THE Business_Metrics_Collector SHALL выполнить SQL-запрос `SELECT group_name, count(*) FROM plugins GROUP BY group_name` и экспортировать результат как gauge-метрику `easyp_business_plugins_by_group` с label `group`
2. THE Business_Metrics_Collector SHALL экспортировать отдельное значение метрики для каждой уникальной группы плагинов

### Требование 3: Сбор метрики общего количества записей аудит-лога

**User Story:** Как оператор сервиса, я хочу видеть общее количество записей в аудит-логе, чтобы отслеживать объём активности сервиса.

#### Критерии приёмки

1. WHEN Prometheus выполняет scrape, THE Business_Metrics_Collector SHALL выполнить SQL-запрос `SELECT count(*) FROM audit_log` и экспортировать результат как gauge-метрику `easyp_business_audit_log_total`
2. IF запрос к Audit_Log_Table завершается ошибкой, THEN THE Business_Metrics_Collector SHALL записать ошибку в лог и не экспортировать метрику для данного scrape-цикла

### Требование 4: Сбор метрики записей аудит-лога по типу операции

**User Story:** Как оператор сервиса, я хочу видеть количество записей аудит-лога в разрезе типов операций, чтобы понимать какие операции используются чаще.

#### Критерии приёмки

1. WHEN Prometheus выполняет scrape, THE Business_Metrics_Collector SHALL выполнить SQL-запрос `SELECT operation_type, count(*) FROM audit_log GROUP BY operation_type` и экспортировать результат как gauge-метрику `easyp_business_audit_log_by_operation` с label `operation`
2. THE Business_Metrics_Collector SHALL экспортировать отдельное значение метрики для каждого уникального типа операции

### Требование 5: Сбор метрики записей аудит-лога по статусу

**User Story:** Как оператор сервиса, я хочу видеть количество записей аудит-лога в разрезе статусов (success/error), чтобы отслеживать уровень ошибок.

#### Критерии приёмки

1. WHEN Prometheus выполняет scrape, THE Business_Metrics_Collector SHALL выполнить SQL-запрос `SELECT status, count(*) FROM audit_log GROUP BY status` и экспортировать результат как gauge-метрику `easyp_business_audit_log_by_status` с label `status`
2. THE Business_Metrics_Collector SHALL экспортировать отдельное значение метрики для каждого уникального статуса

### Требование 6: Сбор метрики количества уникальных версий плагинов

**User Story:** Как оператор сервиса, я хочу видеть количество уникальных версий для каждого плагина, чтобы понимать глубину версионирования.

#### Критерии приёмки

1. WHEN Prometheus выполняет scrape, THE Business_Metrics_Collector SHALL выполнить SQL-запрос `SELECT group_name, name, count(DISTINCT version) FROM plugins GROUP BY group_name, name` и экспортировать результат как gauge-метрику `easyp_business_plugin_versions_count` с labels `group` и `name`

### Требование 7: Сбор метрики записей аудит-лога за последние 24 часа

**User Story:** Как оператор сервиса, я хочу видеть количество записей аудит-лога за последние 24 часа, чтобы отслеживать текущую активность сервиса.

#### Критерии приёмки

1. WHEN Prometheus выполняет scrape, THE Business_Metrics_Collector SHALL выполнить SQL-запрос `SELECT count(*) FROM audit_log WHERE created_at > now() - interval '24 hours'` и экспортировать результат как gauge-метрику `easyp_business_audit_log_last_24h`

### Требование 8: Регистрация коллектора в Prometheus Registry

**User Story:** Как разработчик, я хочу чтобы Business_Metrics_Collector автоматически регистрировался в Prometheus_Registry при старте сервиса, чтобы метрики были доступны без дополнительной настройки.

#### Критерии приёмки

1. WHEN сервис запускается, THE Business_Metrics_Collector SHALL зарегистрироваться в Prometheus_Registry через метод `MustRegister`
2. THE Business_Metrics_Collector SHALL реализовать интерфейс `prometheus.Collector` (методы `Describe` и `Collect`)
3. THE Business_Metrics_Collector SHALL принимать `*sql.DB` и namespace в качестве параметров конструктора, аналогично существующему DBCollector

### Требование 9: Grafana-дашборд для бизнес-метрик

**User Story:** Как оператор сервиса, я хочу иметь готовый Grafana-дашборд с визуализацией бизнес-метрик, чтобы видеть все ключевые показатели на одном экране.

#### Критерии приёмки

1. THE Dashboard SHALL содержать панель "Всего плагинов" типа stat, отображающую метрику `easyp_business_plugins_total`
2. THE Dashboard SHALL содержать панель "Всего записей аудит-лога" типа stat, отображающую метрику `easyp_business_audit_log_total`
3. THE Dashboard SHALL содержать панель "Активность за 24ч" типа stat, отображающую метрику `easyp_business_audit_log_last_24h`
4. THE Dashboard SHALL содержать панель "Плагины по группам" типа pie chart, отображающую метрику `easyp_business_plugins_by_group`
5. THE Dashboard SHALL содержать панель "Операции по типам" типа pie chart, отображающую метрику `easyp_business_audit_log_by_operation`
6. THE Dashboard SHALL содержать панель "Аудит по статусам" типа pie chart, отображающую метрику `easyp_business_audit_log_by_status`
7. THE Dashboard SHALL содержать панель "Версии плагинов" типа table, отображающую метрику `easyp_business_plugin_versions_count`
8. THE Dashboard SHALL быть размещён в директории `configs/grafana/provisioning/dashboards/metrics/` в формате JSON

### Требование 10: Устойчивость к ошибкам базы данных

**User Story:** Как оператор сервиса, я хочу чтобы сбой запроса к PostgreSQL не приводил к падению всего сервиса, чтобы обеспечить стабильную работу.

#### Критерии приёмки

1. IF один из SQL-запросов Business_Metrics_Collector завершается ошибкой, THEN THE Business_Metrics_Collector SHALL продолжить выполнение остальных запросов в рамках текущего scrape-цикла
2. IF SQL-запрос завершается ошибкой, THEN THE Business_Metrics_Collector SHALL записать ошибку в лог с указанием имени метрики и текста ошибки
3. THE Business_Metrics_Collector SHALL использовать контекст с таймаутом для каждого SQL-запроса, чтобы предотвратить зависание при проблемах с базой данных
