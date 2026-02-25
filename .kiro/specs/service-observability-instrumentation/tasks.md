# План реализации: Инструментирование наблюдаемости сервиса

## Обзор

Расширение инструментирования Go gRPC-сервиса EasyP API Service: добавление 13 Prometheus-метрик (генерация, WorkerPool, БД, аудит), расширение трейсинга (WorkerPool, DB-запросы, Docker-выполнение, аудит), интеграция Pyroscope-лейблов. Базовая инфраструктура телеметрии (OTel SDK, декораторы TracingCore/TracingRegistry/TracingPlugin, TraceHandler, Pyroscope init, CoreService интерфейс, otelgrpc, wiring) уже реализована. Язык реализации — Go.

## Задачи

- [x] 1. Базовая инфраструктура телеметрии (уже реализовано)
  - [x] 1.1 Пакет `internal/telemetry` — Config, Init(), TraceHandler, TracingCore, TracingRegistry, TracingPlugin
    - _Требования: покрыты ранее_
  - [x] 1.2 Интерфейс CoreService в `internal/core/domain.go`, otelgrpc в `internal/api/api.go`
    - _Требования: покрыты ранее_
  - [x] 1.3 Wiring в `cmd/main.go` — декораторы, WorkerPool, Pyroscope init
    - _Требования: покрыты ранее_

- [x] 2. Prometheus-метрики генерации кода (Metrics Adapter)
  - [x] 2.1 Расширить интерфейс `Metrics` в `internal/core/domain.go` — добавить методы `ObserveGenerationDuration(ctx, pluginName, duration)`, `IncGenerationErrors(ctx, pluginName, errorType)`, `IncGenerationRetries(ctx, pluginName)`
    - Метод `ObserveGenerationDuration` записывает наблюдение в гистограмму с лейблом `plugin`
    - Метод `IncGenerationErrors` инкрементирует счётчик с лейблами `plugin` и `error_type` (`transient`/`permanent`)
    - Метод `IncGenerationRetries` инкрементирует счётчик с лейблом `plugin`
    - _Требования: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 3.1, 3.2_

  - [x] 2.2 Реализовать 3 метрики в `internal/adapters/metrics/metrics.go` — добавить поля `generationDuration` (Histogram), `generationErrors` (CounterVec), `generationRetries` (CounterVec)
    - `generation_duration_seconds` — гистограмма с лейблом `plugin`, регистрируется в Prometheus_Registry при инициализации
    - `generation_errors_total` — счётчик с лейблами `plugin`, `error_type`, регистрируется при инициализации
    - `generation_retries_total` — счётчик с лейблом `plugin`, регистрируется при инициализации
    - Реализовать методы `ObserveGenerationDuration`, `IncGenerationErrors`, `IncGenerationRetries`
    - _Требования: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 3.1, 3.2_

  - [x] 2.3 Вызвать метрики генерации из `poolPlugin.Generate` в `internal/core/pool.go`
    - `poolPlugin` должен получить ссылку на `Metrics` (передать через `WorkerPool` → `processJob` → `poolPlugin`)
    - В `poolPlugin.Generate`: замерить длительность и вызвать `ObserveGenerationDuration` (независимо от результата — Требование 1.3)
    - При ошибке: определить тип через `isTransient()` и вызвать `IncGenerationErrors` с `error_type=transient` или `permanent`
    - При retry: вызвать `IncGenerationRetries` перед повторной попыткой
    - _Требования: 1.1, 1.3, 2.1, 2.2, 3.1_

  - [ ]* 2.4 Написать unit-тесты для метрик генерации
    - Тест: `generation_duration_seconds` записывается при успешной и ошибочной генерации
    - Тест: `generation_errors_total` инкрементируется с корректным `error_type`
    - Тест: `generation_retries_total` инкрементируется при retry
    - _Требования: 1.1, 1.3, 2.1, 2.2, 3.1_

- [x] 3. Контрольная точка — Метрики генерации
  - Убедиться, что все тесты проходят, задать вопросы пользователю при необходимости.

- [x] 4. Prometheus-метрики WorkerPool
  - [x] 4.1 Добавить приём `*prometheus.Registry` в `NewWorkerPool` и зарегистрировать 4 метрики в `internal/core/pool.go`
    - `pool_queue_depth` — GaugeFunc, возвращающий `len(p.jobs)` на момент scrape
    - `pool_active_workers` — Gauge, inc в начале `processJob`, dec в defer
    - `pool_rejected_total` — Counter, inc в `Get` при `default` (очередь полна)
    - `pool_jobs_total` — Counter, inc в `Get` при успешной отправке в канал
    - Все метрики регистрируются в Prometheus_Registry при инициализации `NewWorkerPool`
    - _Требования: 4.1, 4.2, 5.1, 5.2, 5.3, 6.1, 6.2, 7.1, 7.2_

  - [x] 4.2 Обновить `cmd/main.go` — передать `*prometheus.Registry` в `NewWorkerPool`
    - Добавить параметр `reg` в вызов `core.NewWorkerPool(tracedRegistry, cfg, log, reg)`
    - _Требования: 4.2, 5.3, 6.2, 7.2_

  - [ ]* 4.3 Написать unit-тесты для метрик WorkerPool
    - Тест: `pool_queue_depth` отражает текущий `len(jobs)`
    - Тест: `pool_active_workers` инкрементируется/декрементируется при обработке
    - Тест: `pool_rejected_total` инкрементируется при переполнении очереди
    - Тест: `pool_jobs_total` инкрементируется при принятии задания
    - _Требования: 4.1, 5.1, 5.2, 6.1, 7.1_

- [x] 5. Контрольная точка — Метрики WorkerPool
  - Убедиться, что все тесты проходят, задать вопросы пользователю при необходимости.

- [x] 6. DB Collector — метрики пула соединений PostgreSQL
  - [x] 6.1 Создать `internal/adapters/metrics/db_collector.go` — структура `DBCollector`, реализующая `prometheus.Collector`
    - Принимает `*sql.DB` в конструкторе
    - Метод `Describe` описывает 4 метрики: `db_open_connections`, `db_idle_connections`, `db_wait_count_total`, `db_wait_duration_seconds_total`
    - Метод `Collect` вызывает `db.Stats()` (без SQL-запросов) и возвращает актуальные значения:
      - `db_open_connections` — gauge из `DBStats.OpenConnections`
      - `db_idle_connections` — gauge из `DBStats.Idle`
      - `db_wait_count_total` — counter из `DBStats.WaitCount`
      - `db_wait_duration_seconds_total` — counter из `DBStats.WaitDuration.Seconds()`
    - _Требования: 8.1, 8.2, 9.1, 9.2, 10.1, 10.2, 11.1, 11.2, 19.1, 19.2, 19.3_

  - [x] 6.2 Зарегистрировать `DBCollector` в `cmd/main.go`
    - Создать `metrics.NewDBCollector(r.DB().DB)` после инициализации Registry
    - Вызвать `reg.MustRegister(dbCollector)`
    - _Требования: 8.2, 9.2, 10.2, 11.2, 19.2_

  - [ ]* 6.3 Написать unit-тесты для DBCollector
    - Тест: `Collect` возвращает 4 метрики с корректными именами и типами
    - Тест: значения соответствуют `sql.DBStats`
    - Тест: не выполняются SQL-запросы (только `db.Stats()`)
    - _Требования: 8.1, 9.1, 10.1, 11.1, 19.1_

- [x] 7. Контрольная точка — DB Collector
  - Убедиться, что все тесты проходят, задать вопросы пользователю при необходимости.

- [x] 8. Метрики AuditWorker
  - [x] 8.1 Добавить Prometheus-метрики в `internal/adapters/audit/worker.go`
    - Добавить приём `*prometheus.Registry` в `NewWorker`
    - `audit_queue_depth` — GaugeFunc, возвращающий `len(entries)` на момент scrape
    - `audit_events_lost_total` — Counter, inc при переполнении канала (если добавить non-blocking send) и при shutdown timeout
    - Зарегистрировать метрики в Prometheus_Registry при инициализации
    - В `Shutdown`: при таймауте увеличить `audit_events_lost_total` на количество потерянных событий
    - _Требования: 12.1, 12.2, 13.1, 13.2, 13.3_

  - [x] 8.2 Обновить `cmd/main.go` — передать `*prometheus.Registry` в `NewWorker`
    - _Требования: 12.2, 13.3_

  - [ ]* 8.3 Написать unit-тесты для метрик AuditWorker
    - Тест: `audit_queue_depth` отражает текущий `len(entries)`
    - Тест: `audit_events_lost_total` инкрементируется при переполнении и shutdown timeout
    - _Требования: 12.1, 13.1, 13.2_

- [x] 9. Контрольная точка — Метрики аудита
  - Убедиться, что все тесты проходят, задать вопросы пользователю при необходимости.

- [x] 10. Расширение трейсинга — WorkerPool
  - [x] 10.1 Добавить span `pool.Get` в `WorkerPool.Get` в `internal/core/pool.go`
    - Создать span `pool.Get` с атрибутами `plugin.group`, `plugin.name`, `plugin.version`
    - Записать время ожидания в очереди в атрибут `pool.queue_wait_ms` (замерить от отправки job до получения результата)
    - При отклонении (очередь полна): записать событие `pool.rejected` в текущий span
    - Для трейсинга использовать `go.opentelemetry.io/otel` напрямую (WorkerPool — часть core, но span-ы pool.* допустимы)
    - _Требования: 14.1, 14.2, 14.3_

  - [ ]* 10.2 Написать unit-тесты для трейсинга WorkerPool
    - Тест: span `pool.Get` создаётся с корректными атрибутами
    - Тест: `pool.queue_wait_ms` записывается
    - Тест: событие `pool.rejected` записывается при переполнении
    - _Требования: 14.1, 14.2, 14.3_

- [x] 11. Расширение трейсинга — DB-запросы
  - [x] 11.1 Добавить атрибут `db.statement` в span-ы `registry.Get` и `registry.List` в `internal/telemetry/tracing_registry.go`
    - В `Get`: добавить атрибут `db.statement` с текстом SQL-запроса (select ... from plugins where ...)
    - В `List`: добавить атрибут `db.statement` с текстом SQL-запроса
    - Атрибут `db.system=postgresql` уже присутствует в `Get`; добавить его в `List`
    - _Требования: 15.1_

  - [x] 11.2 Добавить span с `db.system=postgresql` в AuditWorker при записи в БД
    - В `internal/adapters/audit/worker.go` метод `Run`: создать span с `db.system=postgresql` вокруг вызова `w.store.Save()`
    - _Требования: 15.2_

  - [ ]* 11.3 Написать unit-тесты для DB-трейсинга
    - Тест: span `registry.Get` содержит `db.statement`
    - Тест: span в AuditWorker содержит `db.system=postgresql`
    - _Требования: 15.1, 15.2_

- [x] 12. Расширение трейсинга — Docker-выполнение
  - [x] 12.1 Добавить дочерний span `docker.exec` в `TracingPlugin.Generate` в `internal/telemetry/tracing_plugin.go`
    - Создать дочерний span `docker.exec` с атрибутами `docker.image` и `docker.command`
    - Span `docker.exec` должен быть дочерним по отношению к `plugin.Generate`
    - При ошибке: извлечь exit code из `exec.ExitError` и записать в атрибут `docker.exit_code`
    - _Требования: 16.1, 16.2_

  - [ ]* 12.2 Написать unit-тесты для Docker-трейсинга
    - Тест: span `docker.exec` создаётся с `docker.image` и `docker.command`
    - Тест: `docker.exit_code` записывается при ошибке
    - _Требования: 16.1, 16.2_

- [x] 13. Расширение трейсинга — Аудит
  - [x] 13.1 Добавить span `audit.save` в `AuditWorker.Run` в `internal/adapters/audit/worker.go`
    - Создать span `audit.save` с атрибутами `audit.operation_type` и `audit.entry_id`
    - При ошибке записи: вызвать `span.RecordError(err)` и установить статус `Error`
    - _Требования: 17.1, 17.2_

  - [ ]* 13.2 Написать unit-тесты для аудит-трейсинга
    - Тест: span `audit.save` создаётся с корректными атрибутами
    - Тест: ошибка записывается через RecordError
    - _Требования: 17.1, 17.2_

- [x] 14. Контрольная точка — Расширенный трейсинг
  - Убедиться, что все тесты проходят, задать вопросы пользователю при необходимости.

- [x] 15. Pyroscope-лейблы
  - [x] 15.1 Добавить `pyroscope.TagWrapper` в `TracingPlugin.Generate` в `internal/telemetry/tracing_plugin.go`
    - Обернуть вызов `p.inner.Generate` в `pyroscope.TagWrapper(ctx, pyroscope.Labels("plugin", pluginName), func() { ... })`
    - Лейбл `plugin` содержит имя плагина в формате `group/name:version`
    - _Требования: 18.1_

  - [x] 15.2 Добавить `pyroscope.TagWrapper` в `WorkerPool.processJob` в `internal/core/pool.go`
    - Обернуть обработку задания в `pyroscope.TagWrapper(ctx, pyroscope.Labels("operation", "worker.process_job"), func() { ... })`
    - _Требования: 18.2_

  - [ ]* 15.3 Написать unit-тесты для Pyroscope-лейблов
    - Тест: `pyroscope.TagWrapper` вызывается с корректными лейблами (через mock или проверку вызова)
    - _Требования: 18.1, 18.2_

- [x] 16. Финальная контрольная точка — Полная проверка
  - Убедиться, что все тесты проходят, задать вопросы пользователю при необходимости.
  - Проверить, что проект компилируется без ошибок.
  - Убедиться, что существующая метрика `generated_plugin_code_total` продолжает работать через Prometheus-эндпоинт.
  - Убедиться, что все 13 новых Prometheus-метрик зарегистрированы и доступны.

## Примечания

- Задачи, помеченные `*`, являются опциональными и могут быть пропущены для ускорения MVP
- Каждая задача ссылается на конкретные требования для трассируемости
- Контрольные точки обеспечивают инкрементальную валидацию
- Задачи 1.x помечены как [x] — базовая инфраструктура телеметрии уже реализована
- Файлы `internal/core/core.go`, `internal/adapters/registry/registry.go` **не изменяются** — бизнес-логика и слой БД остаются чистыми от кода трассировки
- `internal/core/pool.go` и `internal/adapters/audit/worker.go` получают Prometheus-метрики и минимальный трейсинг напрямую (допустимое исключение для инфраструктурных компонентов)
