# План реализации: Аудит-логирование

## Обзор

Реализация аудит-логирования как сквозной функциональности через gRPC-интерсептор с асинхронной записью в PostgreSQL. Каждый шаг инкрементально добавляет компоненты: доменный слой → миграция → адаптер хранилища → фоновый воркер → интерсептор → интеграция.

## Задачи

- [x] 1. Доменный слой: тип AuditEntry и интерфейс AuditLog
  - [x] 1.1 Добавить тип `AuditEntry` и интерфейс `AuditLog` в `internal/core/domain.go`
    - Добавить импорты `context`, `time`, `github.com/gofrs/uuid/v5` (uuid уже импортирован)
    - Определить структуру `AuditEntry` с полями: `ID uuid.UUID`, `OperationType string`, `PluginName string`, `CallerAddress string`, `Status string`, `ErrorCode string`, `ErrorMessage string`, `DurationMs int64`, `Metadata map[string]any`, `CreatedAt time.Time`
    - Определить интерфейс `AuditLog` с методом `Save(ctx context.Context, entry AuditEntry) error`
    - _Требования: 6.1, 6.3_

  - [ ]* 1.2 Написать property-тест для маппинга gRPC-метода в Operation Type
    - **Property 4: Маппинг gRPC-метода в Operation Type**
    - Создать файл `internal/api/audit_interceptor_test.go`
    - Определить функцию маппинга `methodToOperationType` и протестировать с помощью `rapid`: для известных методов возвращается корректный тип, для произвольных строк — пустая строка
    - **Validates: Требования 5.4**

- [x] 2. SQL-миграция для таблицы audit_log
  - [x] 2.1 Создать файл миграции `migrate/3.audit_log.sql`
    - Создать таблицу `audit_log` с полями: `id UUID NOT NULL DEFAULT gen_random_uuid()`, `operation_type TEXT NOT NULL`, `plugin_name TEXT`, `caller_address TEXT NOT NULL`, `status TEXT NOT NULL`, `error_code TEXT`, `error_message TEXT`, `duration_ms BIGINT NOT NULL`, `metadata JSONB NOT NULL DEFAULT '{}'`, `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `PRIMARY KEY (id)`
    - Создать индекс `idx_audit_log_created_at` по полю `created_at`
    - Создать индекс `idx_audit_log_operation_type` по полю `operation_type`
    - Использовать `CREATE TABLE IF NOT EXISTS` и `CREATE INDEX IF NOT EXISTS` по аналогии с существующими миграциями
    - _Требования: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 3. Аудит-адаптер (Store) для PostgreSQL
  - [x] 3.1 Создать файл `internal/adapters/audit/audit.go` с реализацией `Store`
    - Определить структуру `Store` с полями `db *sqlx.DB` и `logger *slog.Logger`
    - Реализовать конструктор `New(db *sqlx.DB, logger *slog.Logger) *Store`
    - Реализовать метод `Save(ctx context.Context, entry core.AuditEntry) error` — INSERT в таблицу `audit_log` с сериализацией `Metadata` в JSONB
    - Убедиться, что `Store` реализует интерфейс `core.AuditLog` (compile-time check: `var _ core.AuditLog = &Store{}`)
    - _Требования: 3.1, 6.2_

  - [ ]* 3.2 Написать property-тест для round-trip сохранения
    - **Property 3: Round-trip сохранения аудит-записи**
    - Создать файл `internal/adapters/audit/audit_test.go`
    - С помощью `rapid` генерировать произвольные `AuditEntry`, сохранять через `Save()`, читать из БД по `id` и проверять совпадение всех полей
    - Требует тестовую БД PostgreSQL (можно пометить `//go:build integration`)
    - **Validates: Требования 3.1**

- [x] 4. Checkpoint — проверка доменного слоя и адаптера
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Background Worker для асинхронной записи
  - [x] 5.1 Создать файл `internal/adapters/audit/worker.go` с реализацией `Worker`
    - Определить структуру `Worker` с полями: `store core.AuditLog`, `entries <-chan core.AuditEntry`, `logger *slog.Logger`, `done chan struct{}`
    - Реализовать `NewWorker(store core.AuditLog, bufferSize int, logger *slog.Logger) (*Worker, chan<- core.AuditEntry)` — создаёт буферизированный канал и возвращает write-end
    - Реализовать `Run(ctx context.Context)` — читает из канала, вызывает `store.Save()`, при ошибке логирует через slog и продолжает
    - Реализовать `Shutdown(timeout time.Duration) int` — закрывает канал, ожидает завершения записи с таймаутом, возвращает количество потерянных событий
    - _Требования: 4.1, 4.2, 7.1, 7.2_

  - [ ]* 5.2 Написать property-тест для graceful degradation при ошибке записи
    - **Property 5: Graceful degradation при ошибке записи**
    - Создать файл `internal/adapters/audit/worker_test.go`
    - С помощью `rapid` генерировать последовательности `AuditEntry` и мок `AuditLog`, который случайно возвращает ошибки; проверить, что воркер продолжает обработку всех записей
    - **Validates: Требования 4.1**

  - [ ]* 5.3 Написать property-тест для полного сброса буфера при завершении
    - **Property 7: Полный сброс буфера при завершении**
    - В файле `internal/adapters/audit/worker_test.go`
    - С помощью `rapid` генерировать набор `AuditEntry`, отправить в канал, вызвать `Shutdown` и проверить, что все события сохранены (при условии, что `Save` не превышает таймаут)
    - **Validates: Требования 7.1, 7.2**

- [x] 6. Аудит-интерсептор для gRPC
  - [x] 6.1 Создать файл `internal/api/audit_interceptor.go` с реализацией `AuditInterceptor`
    - Определить структуру `AuditInterceptor` с полями `entries chan<- core.AuditEntry` и `logger *slog.Logger`
    - Реализовать функцию маппинга `methodToOperationType(fullMethod string) string`
    - Реализовать метод `UnaryServerInterceptor() grpc.UnaryServerInterceptor`:
      - Извлечь peer-адрес через `peer.FromContext(ctx)`, при отсутствии — `"unknown"`
      - Определить `operation_type` через маппинг; для неизвестных методов — пропустить аудит, вызвать handler напрямую
      - Замерить длительность через `time.Since(start)`
      - Определить статус (`"success"` / `"error"`) и извлечь error_code/error_message из gRPC status
      - Извлечь метаданные из ответа: `file_count` из `GenerateCodeResponse`, `plugin_count` из `PluginsResponse`
      - Отправить `AuditEntry` в канал через неблокирующий `select` с `default` (при полном канале — логировать предупреждение)
    - _Требования: 1.1, 1.2, 1.3, 2.1, 2.2, 4.2, 4.3, 5.1, 5.2, 5.3, 5.4_

  - [ ]* 6.2 Написать property-тест для корректности формирования аудит-записи
    - **Property 1: Корректность формирования аудит-записи интерсептором**
    - В файле `internal/api/audit_interceptor_test.go`
    - С помощью `rapid` генерировать произвольные peer-адреса, методы и результаты handler; проверить, что `AuditEntry` содержит корректные `operation_type`, `caller_address`, `duration_ms >= 0`, `status`
    - **Validates: Требования 1.1, 2.1, 5.2, 5.3**

  - [ ]* 6.3 Написать property-тест для корректности извлечения метаданных
    - **Property 2: Корректность извлечения метаданных из ответа**
    - В файле `internal/api/audit_interceptor_test.go`
    - С помощью `rapid` генерировать ответы `GenerateCodeResponse` и `PluginsResponse` с произвольным количеством файлов/плагинов; проверить корректность `metadata`
    - **Validates: Требования 1.2, 1.3, 2.2**

  - [ ]* 6.4 Написать property-тест для неблокирующей отправки в канал
    - **Property 6: Неблокирующая отправка в канал**
    - В файле `internal/api/audit_interceptor_test.go`
    - С помощью `rapid` генерировать `AuditEntry` и отправлять в заполненный канал (размер 1); проверить, что отправка завершается мгновенно (не блокирует) и событие отбрасывается
    - **Validates: Требования 4.2, 4.3**

- [x] 7. Checkpoint — проверка воркера и интерсептора
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Интеграция в api.New и cmd/main.go
  - [x] 8.1 Изменить `api.New` в `internal/api/api.go`
    - Изменить сигнатуру: `func New(ctx context.Context, applications *core.Core, auditCh chan<- core.AuditEntry) *grpc.Server`
    - Извлечь текущий inline error interceptor в отдельную функцию `errorInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor`
    - Создать `AuditInterceptor` и использовать `grpc.ChainUnaryInterceptor` вместо `grpc.UnaryInterceptor`
    - Аудит-интерсептор первый в цепочке (измеряет полную длительность)
    - _Требования: 5.1_

  - [x] 8.2 Добавить метод `DB()` в `Registry` в `internal/adapters/registry/registry.go`
    - Добавить метод `func (r *Registry) DB() *sqlx.DB { return r.sql }` для доступа к `*sqlx.DB` из `cmd/main.go`
    - _Требования: 3.1_

  - [x] 8.3 Интегрировать аудит-логирование в `cmd/main.go`
    - Добавить импорт пакета `audit` (`internal/adapters/audit`)
    - Создать `auditStore := audit.New(r.DB(), log)`
    - Создать `auditWorker, auditCh := audit.NewWorker(auditStore, 1000, log)`
    - Запустить воркер: `go auditWorker.Run(ctx)`
    - Передать `auditCh` в `api.New(ctx, module, auditCh)`
    - Добавить shutdown-логику перед закрытием БД: `lost := auditWorker.Shutdown(5 * time.Second)` с логированием потерянных событий
    - _Требования: 4.2, 7.1, 7.2_

- [x] 9. Финальный checkpoint — полная проверка
  - Ensure all tests pass, ask the user if questions arise.

## Примечания

- Задачи с `*` — опциональные (property-тесты и unit-тесты), можно пропустить для быстрого MVP
- Каждая задача ссылается на конкретные требования для трассируемости
- Property-тесты используют библиотеку [rapid](https://github.com/flyingmutant/rapid)
- Checkpoints обеспечивают инкрементальную валидацию
