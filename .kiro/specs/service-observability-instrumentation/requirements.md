# Документ требований: Комплексная инструментация наблюдаемости EasyP API Service

## Введение

Сервис EasyP API Service выполняет генерацию protobuf-кода через Docker-контейнеры. На данный момент наблюдаемость ограничена одним Prometheus-счётчиком (`generated_plugin_code_total`), частичным трейсингом (TracingCore, TracingRegistry, TracingPlugin) и базовой инициализацией Pyroscope. Цель — расширить инструментацию до 13 Prometheus-метрик, покрыть трейсингом все ключевые операции и обеспечить корректную отправку профилировочных данных в Pyroscope с привязкой лейблов.

## Глоссарий

- **Metrics_Adapter** — компонент `internal/adapters/metrics`, отвечающий за регистрацию и обновление Prometheus-метрик
- **WorkerPool** — компонент `internal/core/pool.go`, управляющий пулом горутин для ограничения параллелизма Docker-выполнения
- **Registry** — компонент `internal/adapters/registry`, обеспечивающий доступ к плагинам через PostgreSQL и Docker
- **AuditWorker** — компонент `internal/adapters/audit/worker.go`, асинхронно записывающий аудит-события из канала в хранилище
- **TracingCore** — декоратор CoreService, добавляющий span-ы трассировки на уровне бизнес-логики
- **TracingRegistry** — декоратор Registry, добавляющий span-ы трассировки на уровне реестра плагинов
- **TracingPlugin** — декоратор Plugin, добавляющий span и метрику длительности генерации
- **Prometheus_Registry** — экземпляр `prometheus.Registry`, в котором регистрируются все метрики
- **Pyroscope_Profiler** — компонент профилирования, отправляющий CPU/memory/goroutine-профили в Pyroscope
- **DB_Collector** — компонент, периодически собирающий статистику из `sql.DBStats` и экспортирующий её как Prometheus-метрики
- **Transient_Error** — временная ошибка Docker-выполнения (exit code 125/126/127, connection refused, daemon error)
- **Permanent_Error** — ошибка, не являющаяся временной (ошибка маршалинга, ErrNotFound, context.DeadlineExceeded)

## Требования

### Требование 1: Гистограмма длительности генерации

**User Story:** Как SRE-инженер, я хочу видеть распределение времени генерации кода по плагинам, чтобы выявлять деградацию производительности Docker-выполнения.

#### Критерии приёмки

1. WHEN метод `Generate` вызывается на уровне poolPlugin, THE Metrics_Adapter SHALL записать наблюдение в гистограмму `generation_duration_seconds` с лейблом `plugin`, содержащим имя плагина в формате `group/name:version`
2. THE Metrics_Adapter SHALL зарегистрировать метрику `generation_duration_seconds` в Prometheus_Registry при инициализации
3. WHEN генерация завершается ошибкой, THE Metrics_Adapter SHALL записать длительность в гистограмму `generation_duration_seconds` независимо от результата

### Требование 2: Счётчик ошибок генерации

**User Story:** Как SRE-инженер, я хочу различать временные и постоянные ошибки генерации по плагинам, чтобы корректно настраивать алерты.

#### Критерии приёмки

1. WHEN генерация завершается ошибкой, THE Metrics_Adapter SHALL инкрементировать счётчик `generation_errors_total` с лейблами `plugin` и `error_type`
2. THE Metrics_Adapter SHALL устанавливать лейбл `error_type` в значение `transient` для временных ошибок и `permanent` для остальных ошибок
3. THE Metrics_Adapter SHALL зарегистрировать метрику `generation_errors_total` в Prometheus_Registry при инициализации

### Требование 3: Счётчик повторных попыток генерации

**User Story:** Как SRE-инженер, я хочу отслеживать количество повторных попыток генерации по плагинам, чтобы оценивать стабильность Docker-демона.

#### Критерии приёмки

1. WHEN poolPlugin выполняет повторную попытку генерации, THE Metrics_Adapter SHALL инкрементировать счётчик `generation_retries_total` с лейблом `plugin`
2. THE Metrics_Adapter SHALL зарегистрировать метрику `generation_retries_total` в Prometheus_Registry при инициализации

### Требование 4: Gauge глубины очереди WorkerPool

**User Story:** Как SRE-инженер, я хочу видеть текущую глубину очереди заданий, чтобы оценивать нагрузку и приближение к backpressure.

#### Критерии приёмки

1. WHEN запрашивается значение метрики `pool_queue_depth`, THE WorkerPool SHALL возвращать текущее значение `len(jobs)` как gauge-метрику
2. THE WorkerPool SHALL зарегистрировать метрику `pool_queue_depth` в Prometheus_Registry при инициализации

### Требование 5: Gauge активных воркеров

**User Story:** Как SRE-инженер, я хочу видеть количество занятых воркеров в реальном времени, чтобы оценивать утилизацию пула.

#### Критерии приёмки

1. WHILE воркер обрабатывает задание, THE WorkerPool SHALL инкрементировать gauge `pool_active_workers`
2. WHEN воркер завершает обработку задания, THE WorkerPool SHALL декрементировать gauge `pool_active_workers`
3. THE WorkerPool SHALL зарегистрировать метрику `pool_active_workers` в Prometheus_Registry при инициализации

### Требование 6: Счётчик отклонённых запросов

**User Story:** Как SRE-инженер, я хочу знать, сколько запросов отклонено из-за переполнения очереди, чтобы масштабировать сервис.

#### Критерии приёмки

1. WHEN очередь WorkerPool заполнена и новое задание отклоняется, THE WorkerPool SHALL инкрементировать счётчик `pool_rejected_total`
2. THE WorkerPool SHALL зарегистрировать метрику `pool_rejected_total` в Prometheus_Registry при инициализации

### Требование 7: Счётчик принятых заданий

**User Story:** Как SRE-инженер, я хочу видеть общее количество принятых заданий, чтобы рассчитывать процент отклонений.

#### Критерии приёмки

1. WHEN задание успешно помещается в очередь WorkerPool, THE WorkerPool SHALL инкрементировать счётчик `pool_jobs_total`
2. THE WorkerPool SHALL зарегистрировать метрику `pool_jobs_total` в Prometheus_Registry при инициализации


### Требование 8: Gauge открытых соединений БД

**User Story:** Как SRE-инженер, я хочу видеть количество открытых соединений к PostgreSQL, чтобы выявлять утечки соединений.

#### Критерии приёмки

1. THE DB_Collector SHALL экспортировать значение `sql.DBStats.OpenConnections` как gauge-метрику `db_open_connections`
2. THE DB_Collector SHALL зарегистрировать метрику `db_open_connections` в Prometheus_Registry при инициализации

### Требование 9: Gauge простаивающих соединений БД

**User Story:** Как SRE-инженер, я хочу видеть количество простаивающих соединений к PostgreSQL, чтобы оптимизировать пул соединений.

#### Критерии приёмки

1. THE DB_Collector SHALL экспортировать значение `sql.DBStats.Idle` как gauge-метрику `db_idle_connections`
2. THE DB_Collector SHALL зарегистрировать метрику `db_idle_connections` в Prometheus_Registry при инициализации

### Требование 10: Счётчик ожиданий соединений БД

**User Story:** Как SRE-инженер, я хочу видеть количество запросов, ожидавших свободного соединения, чтобы оценивать давление на пул.

#### Критерии приёмки

1. THE DB_Collector SHALL экспортировать значение `sql.DBStats.WaitCount` как counter-метрику `db_wait_count_total`
2. THE DB_Collector SHALL зарегистрировать метрику `db_wait_count_total` в Prometheus_Registry при инициализации

### Требование 11: Счётчик суммарного времени ожидания соединений БД

**User Story:** Как SRE-инженер, я хочу видеть суммарное время ожидания соединений, чтобы оценивать влияние пула на латентность.

#### Критерии приёмки

1. THE DB_Collector SHALL экспортировать значение `sql.DBStats.WaitDuration` в секундах как counter-метрику `db_wait_duration_seconds_total`
2. THE DB_Collector SHALL зарегистрировать метрику `db_wait_duration_seconds_total` в Prometheus_Registry при инициализации

### Требование 12: Gauge глубины очереди аудита

**User Story:** Как SRE-инженер, я хочу видеть текущий уровень заполнения канала аудит-событий, чтобы предотвращать потерю событий.

#### Критерии приёмки

1. WHEN запрашивается значение метрики `audit_queue_depth`, THE AuditWorker SHALL возвращать текущее значение `len(entries)` как gauge-метрику
2. THE AuditWorker SHALL зарегистрировать метрику `audit_queue_depth` в Prometheus_Registry при инициализации

### Требование 13: Счётчик потерянных аудит-событий

**User Story:** Как SRE-инженер, я хочу знать количество потерянных аудит-событий, чтобы оценивать надёжность аудит-подсистемы.

#### Критерии приёмки

1. WHEN аудит-канал переполнен и событие не может быть отправлено, THE AuditWorker SHALL инкрементировать счётчик `audit_events_lost_total`
2. WHEN AuditWorker завершает работу по таймауту и в канале остаются необработанные события, THE AuditWorker SHALL увеличить счётчик `audit_events_lost_total` на количество потерянных событий
3. THE AuditWorker SHALL зарегистрировать метрику `audit_events_lost_total` в Prometheus_Registry при инициализации

### Требование 14: Расширение трейсинга — WorkerPool

**User Story:** Как разработчик, я хочу видеть span-ы операций WorkerPool в трейсах, чтобы диагностировать задержки в очереди и обработке заданий.

#### Критерии приёмки

1. WHEN метод `Get` вызывается на WorkerPool, THE TracingRegistry SHALL создать span `pool.Get` с атрибутами `plugin.group`, `plugin.name`, `plugin.version`
2. WHEN задание ожидает в очереди WorkerPool, THE TracingRegistry SHALL записать время ожидания в атрибут span `pool.queue_wait_ms`
3. IF задание отклоняется из-за переполнения очереди, THEN THE TracingRegistry SHALL записать событие `pool.rejected` в текущий span

### Требование 15: Расширение трейсинга — DB-запросы

**User Story:** Как разработчик, я хочу видеть span-ы SQL-запросов в трейсах, чтобы диагностировать медленные запросы к PostgreSQL.

#### Критерии приёмки

1. WHEN выполняется SQL-запрос в Registry, THE TracingRegistry SHALL создать дочерний span с атрибутом `db.system` равным `postgresql` и атрибутом `db.statement`, содержащим текст запроса
2. WHEN выполняется SQL-запрос в AuditWorker, THE AuditWorker SHALL создать дочерний span с атрибутом `db.system` равным `postgresql`

### Требование 16: Расширение трейсинга — Docker-выполнение

**User Story:** Как разработчик, я хочу видеть span-ы Docker-выполнения в трейсах, чтобы различать время получения плагина и время генерации кода.

#### Критерии приёмки

1. WHEN выполняется Docker-команда в методе `Generate` плагина, THE TracingPlugin SHALL создать дочерний span `docker.exec` с атрибутами `docker.image` и `docker.command`
2. IF Docker-команда завершается с ошибкой, THEN THE TracingPlugin SHALL записать exit code в атрибут span `docker.exit_code`

### Требование 17: Расширение трейсинга — Аудит

**User Story:** Как разработчик, я хочу видеть span-ы аудит-операций в трейсах, чтобы диагностировать задержки записи аудит-событий.

#### Критерии приёмки

1. WHEN AuditWorker записывает аудит-событие в хранилище, THE AuditWorker SHALL создать span `audit.save` с атрибутами `audit.operation_type` и `audit.entry_id`
2. IF запись аудит-события завершается ошибкой, THEN THE AuditWorker SHALL записать ошибку в span через RecordError

### Требование 18: Pyroscope-профилирование с лейблами

**User Story:** Как SRE-инженер, я хочу коррелировать профили CPU/memory с конкретными операциями, чтобы находить горячие точки в генерации кода.

#### Критерии приёмки

1. WHILE выполняется генерация кода, THE Pyroscope_Profiler SHALL прикреплять лейбл `plugin` с именем плагина к профилировочным данным через `pyroscope.TagWrapper`
2. WHILE выполняется обработка задания в WorkerPool, THE Pyroscope_Profiler SHALL прикреплять лейбл `operation` со значением `worker.process_job` к профилировочным данным
3. THE Pyroscope_Profiler SHALL сохранять все типы профилей: CPU, AllocObjects, AllocSpace, InuseObjects, InuseSpace, Goroutines

### Требование 19: Сбор метрик БД через sql.DBStats

**User Story:** Как SRE-инженер, я хочу получать метрики пула соединений PostgreSQL без дополнительных запросов к БД, чтобы минимизировать накладные расходы.

#### Критерии приёмки

1. THE DB_Collector SHALL собирать статистику из `sql.DBStats` через вызов `db.Stats()` без выполнения дополнительных SQL-запросов
2. THE DB_Collector SHALL реализовать интерфейс `prometheus.Collector` для интеграции с Prometheus_Registry
3. WHEN Prometheus выполняет scrape, THE DB_Collector SHALL возвращать актуальные значения из `sql.DBStats` на момент запроса
