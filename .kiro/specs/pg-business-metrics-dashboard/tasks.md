# План реализации: Бизнес-метрики PostgreSQL для Grafana

## Обзор

Реализация `BusinessMetricsCollector` — Prometheus-коллектора для бизнес-метрик из PostgreSQL, его регистрация в сервисе и создание Grafana-дашборда. Коллектор следует паттерну существующего `DBCollector` из `internal/adapters/metrics/db_collector.go`.

## Задачи

- [x] 1. Реализовать BusinessMetricsCollector
  - [x] 1.1 Создать файл `internal/adapters/metrics/business_collector.go` со структурой и конструктором
    - Определить структуру `BusinessMetricsCollector` с полями `db *sql.DB`, `log *slog.Logger` и 7 дескрипторами `*prometheus.Desc`
    - Реализовать конструктор `NewBusinessMetricsCollector(db *sql.DB, namespace string, log *slog.Logger)`, создающий дескрипторы через `prometheus.NewDesc` с subsystem `"business"`
    - Определить константу `defaultQueryTimeout = 5 * time.Second`
    - Добавить compile-time проверку интерфейса: `var _ prometheus.Collector = (*BusinessMetricsCollector)(nil)`
    - Следовать паттерну `DBCollector` из `internal/adapters/metrics/db_collector.go`
    - _Требования: 8.2, 8.3_

  - [x] 1.2 Реализовать метод `Describe`
    - Отправить все 7 дескрипторов метрик в канал `ch chan<- *prometheus.Desc`
    - _Требования: 8.2_

  - [x] 1.3 Реализовать метод `Collect` для скалярных метрик
    - Реализовать сбор `easyp_business_plugins_total` (SQL: `SELECT count(*) FROM plugins`)
    - Реализовать сбор `easyp_business_audit_log_total` (SQL: `SELECT count(*) FROM audit_log`)
    - Реализовать сбор `easyp_business_audit_log_last_24h` (SQL: `SELECT count(*) FROM audit_log WHERE created_at > now() - interval '24 hours'`)
    - Каждый запрос выполнять с `context.WithTimeout(ctx, defaultQueryTimeout)`
    - При ошибке — логировать через `slog.Logger` с указанием имени метрики и текста ошибки, не отправлять метрику в канал, продолжить сбор остальных
    - _Требования: 1.1, 1.2, 3.1, 3.2, 7.1, 10.1, 10.2, 10.3_

  - [x] 1.4 Реализовать метод `Collect` для метрик с группировкой
    - Реализовать сбор `easyp_business_plugins_by_group` с label `group` (SQL: `SELECT group_name, count(*) FROM plugins GROUP BY group_name`)
    - Реализовать сбор `easyp_business_audit_log_by_operation` с label `operation` (SQL: `SELECT operation_type, count(*) FROM audit_log GROUP BY operation_type`)
    - Реализовать сбор `easyp_business_audit_log_by_status` с label `status` (SQL: `SELECT status, count(*) FROM audit_log GROUP BY status`)
    - Реализовать сбор `easyp_business_plugin_versions_count` с labels `group`, `name` (SQL: `SELECT group_name, name, count(DISTINCT version) FROM plugins GROUP BY group_name, name`)
    - Каждый запрос с `context.WithTimeout`, обработка ошибок аналогично скалярным метрикам
    - _Требования: 2.1, 2.2, 4.1, 4.2, 5.1, 5.2, 6.1, 10.1, 10.2, 10.3_

  - [ ]* 1.5 Написать property-тест: скалярные метрики отражают реальное количество записей
    - **Свойство 1: Скалярные метрики отражают реальное количество записей**
    - **Проверяет: Требования 1.1, 3.1, 7.1**
    - Использовать `pgregory.net/rapid` для генерации произвольного количества записей в mock-таблицах
    - Вызвать `Collect()`, проверить что скалярные gauge-метрики совпадают с ожидаемыми count

  - [ ]* 1.6 Написать property-тест: метрики с группировкой корректно отражают распределение
    - **Свойство 2: Метрики с группировкой корректно отражают распределение**
    - **Проверяет: Требования 2.1, 2.2, 4.1, 4.2, 5.1, 5.2**
    - Использовать `pgregory.net/rapid` для генерации записей с произвольными значениями группирующих полей
    - Проверить что количество экспортированных метрик = количество уникальных групп, и значения совпадают

  - [ ]* 1.7 Написать property-тест: метрика версий плагинов корректно отражает count distinct
    - **Свойство 3: Метрика версий плагинов корректно отражает count distinct**
    - **Проверяет: Требования 6.1**
    - Генерировать плагины с произвольными group/name/version, проверить count distinct versions для каждой пары

  - [ ]* 1.8 Написать property-тест: изоляция ошибок SQL-запросов
    - **Свойство 4: Изоляция ошибок SQL-запросов**
    - **Проверяет: Требования 10.1**
    - Генерировать произвольное подмножество запросов, возвращающих ошибку, проверить что остальные метрики экспортируются

  - [ ]* 1.9 Написать unit-тесты для BusinessMetricsCollector
    - Тест: `Describe` отправляет ровно 7 дескрипторов
    - Тест: `Collect` с пустыми таблицами — скалярные метрики возвращают 0
    - Тест: `Collect` при ошибке БД — метрика не экспортируется, ошибка логируется
    - Тест: `MustRegister` не вызывает panic
    - Использовать `sqlmock` для мокирования SQL-запросов
    - _Требования: 1.1, 1.2, 3.1, 3.2, 8.2, 10.1, 10.2_

- [x] 2. Контрольная точка — проверка коллектора
  - Убедиться что все тесты проходят, задать вопросы пользователю при необходимости.

- [x] 3. Зарегистрировать коллектор в main.go и создать Grafana-дашборд
  - [x] 3.1 Зарегистрировать BusinessMetricsCollector в `cmd/main.go`
    - Добавить создание `businessCollector` после строки с `dbCollector` в функции `run()`
    - Вызвать `reg.MustRegister(businessCollector)`
    - Использовать `r.DB().UnderlyingDB()` для получения `*sql.DB`, `namespace` и `log` из существующих переменных
    - _Требования: 8.1_

  - [x] 3.2 Создать Grafana-дашборд `configs/grafana/provisioning/dashboards/metrics/business.json`
    - Создать JSON-дашборд с 7 панелями:
      - Stat: "Всего плагинов" (`easyp_business_plugins_total`)
      - Stat: "Всего записей аудит-лога" (`easyp_business_audit_log_total`)
      - Stat: "Активность за 24ч" (`easyp_business_audit_log_last_24h`)
      - Pie Chart: "Плагины по группам" (`easyp_business_plugins_by_group`)
      - Pie Chart: "Операции по типам" (`easyp_business_audit_log_by_operation`)
      - Pie Chart: "Аудит по статусам" (`easyp_business_audit_log_by_status`)
      - Table: "Версии плагинов" (`easyp_business_plugin_versions_count`)
    - Использовать datasource `Prometheus`, дашборд автоматически подхватится провайдером "Metrics" из `dashboards.yaml`
    - Следовать структуре существующего `configs/grafana/provisioning/dashboards/metrics/service.json`
    - _Требования: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7, 9.8_

  - [ ]* 3.3 Написать unit-тест для валидации дашборда
    - **Свойство 5: Дашборд содержит все требуемые панели с корректными метриками**
    - **Проверяет: Требования 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7**
    - Парсинг JSON-файла дашборда, проверка наличия 7 панелей с правильными типами (stat, piechart, table) и PromQL-запросами

- [x] 4. Финальная контрольная точка
  - Убедиться что все тесты проходят, задать вопросы пользователю при необходимости.

## Примечания

- Задачи с `*` являются опциональными и могут быть пропущены для ускорения MVP
- Каждая задача ссылается на конкретные требования для трассируемости
- Property-тесты используют библиотеку `pgregory.net/rapid`
- Контрольные точки обеспечивают инкрементальную валидацию
