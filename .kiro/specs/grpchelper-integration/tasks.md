# План реализации: Интеграция grpchelper

## Обзор

Интеграция библиотеки `internal/grpchelper` в сервис: адаптация импортов (замена `sipki-tech/back-template` на локальные пакеты), рефакторинг создания gRPC-сервера (перенос из `api.New()` в `cmd/main.go` через `grpchelper.NewServer()`), реализация `GRPCCodesConverter` и обеспечение совместимости метрик с Grafana Dashboard.

## Задачи

- [x] 1. Добавить зависимости grpc-middleware в go.mod
  - Выполнить `go get github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus`
  - Выполнить `go get github.com/grpc-ecosystem/go-grpc-middleware/v2`
  - Убедиться, что `go mod tidy` проходит без ошибок
  - _Требования: 1.4_

- [x] 2. Адаптировать импорты grpchelper
  - [x] 2.1 Адаптировать `internal/grpchelper/trace_logging.go`
    - Заменить импорт `github.com/sipki-tech/back-template/internal/logger` на `github.com/easyp-tech/service/internal/monitor`
    - Заменить `logger.FromContext(ctx)` на `monitor.FromContext(ctx)`
    - Заменить `logger.NewContext(ctx, log)` на `monitor.WithContext(ctx, log)` (имя функции отличается!)
    - Проверить, что `slog.Default()` используется как fallback корректно (в monitor пакете fallback — это внутренний `sLogger`, не `slog.Default()`)
    - _Требования: 1.1, 1.4_

  - [x] 2.2 Адаптировать `internal/grpchelper/grpc_logs.go`
    - Заменить импорт `github.com/sipki-tech/back-template/internal/logger` на `github.com/easyp-tech/service/internal/monitor`
    - Заменить импорт `github.com/sipki-tech/back-template/internal/metrics` — удалить, так как интерфейс метрик будет определён локально
    - Заменить `logger.FromContext(ctx)` на `monitor.FromContext(ctx)` в функции `recoveryFunc`
    - Определить константу `const panicReason = "panic_reason"` локально в файле и использовать вместо `logger.PanicReason.String()`
    - Заменить `m.PanicsTotal.Inc()` на `m.PanicsTotal().Inc()` (метод вместо поля, согласно локальному интерфейсу)
    - Обновить `interceptorLogger`: заменить `logger.FromContext(ctx)` на `monitor.FromContext(ctx)`, исправить fallback-проверку (monitor возвращает внутренний логгер, а не `slog.Default()`)
    - _Требования: 1.1, 1.2, 1.3, 6.2, 6.3_

  - [x] 2.3 Определить локальный интерфейс `Metrics` в `internal/grpchelper/server.go`
    - Удалить импорт `github.com/sipki-tech/back-template/internal/metrics`
    - Определить интерфейс `Metrics` с методом `PanicsTotal() prometheus.Counter` в пакете grpchelper
    - Обновить сигнатуру `NewServer` — параметр `m` теперь типа `Metrics` (локальный интерфейс)
    - _Требования: 1.3, 1.4, 6.1_

- [x] 3. Контрольная точка — компиляция grpchelper
  - Убедиться, что пакет `internal/grpchelper` компилируется без ошибок
  - Выполнить `go build ./internal/grpchelper/...`
  - При возникновении вопросов — уточнить у пользователя
  - _Требования: 1.4_

- [x] 4. Реализовать GRPCCodesConverter и обновить api.go
  - [x] 4.1 Экспортировать функцию-конвертер из пакета `api`
    - Переименовать `apiError` в `ErrorToStatus` (или аналогичное экспортируемое имя) — функция типа `func(error) *status.Status`, совместимая с `grpchelper.GRPCCodesConverterHandler`
    - Убедиться, что маппинг ошибок сохранён: `core.ErrNotFound` → `codes.NotFound`, `core.ErrInvalidPluginName` → `codes.InvalidArgument`, `core.ErrGenerationFailed` → `codes.Internal`, `core.ErrServerOverloaded` → `codes.ResourceExhausted`, `core.ErrShuttingDown` → `codes.Unavailable`, `context.DeadlineExceeded` → `codes.DeadlineExceeded`, `context.Canceled` → `codes.Canceled`
    - Для `nil` ошибки возвращать `status.New(codes.OK, "")` (а не `nil`), чтобы соответствовать контракту `GRPCCodesConverterHandler`
    - _Требования: 3.1, 3.2_

  - [x] 4.2 Рефакторинг `api.New()` — изменить сигнатуру
    - Новая сигнатура: `New(grpcSrv *grpc.Server, healthSrv *health.Server, applications core.CoreService, auditCh chan<- core.AuditEntry, log *slog.Logger)`
    - Удалить создание `grpc.NewServer()`, `health.NewServer()`, `reflection.Register()`, `errorInterceptor()`
    - Удалить импорт `otelgrpc`
    - Сохранить `generator.RegisterServiceAPIServer(grpcSrv, api)` — регистрация обработчика на переданном сервере
    - Сохранить `healthServer.SetServingStatus(generator.ServiceAPI_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)` — на переданном health-сервере
    - Функция больше не возвращает `*grpc.Server` (возвращает `void` или структуру `*API`)
    - _Требования: 4.1, 4.2, 4.3, 4.4, 4.5, 5.1, 5.2, 5.3_

  - [ ]* 4.3 Написать unit-тесты для `ErrorToStatus`
    - Проверить маппинг каждого типа ошибки на соответствующий gRPC-код
    - Проверить, что неизвестная ошибка возвращает `codes.Internal`
    - Проверить, что `nil` возвращает `codes.OK`
    - _Требования: 3.1, 3.2_

- [x] 5. Обновить `cmd/main.go` — интеграция grpchelper
  - [x] 5.1 Создать ServerMetrics и panics-счётчик
    - Вызвать `grpchelper.NewServerMetrics(reg, "easyp", "api")` для создания gRPC-метрик
    - Создать `prometheus.Counter` для паник (например, `prometheus.NewCounter(prometheus.CounterOpts{Namespace: "easyp", Name: "panics_total"})`) и зарегистрировать в `reg`
    - Реализовать адаптер, удовлетворяющий интерфейсу `grpchelper.Metrics` (метод `PanicsTotal() prometheus.Counter`)
    - _Требования: 2.1, 2.2, 6.1_

  - [x] 5.2 Создать gRPC-сервер через grpchelper.NewServer
    - Создать `converter` из `api.ErrorToStatus` (тип `grpchelper.GRPCCodesConverterHandler`)
    - Создать `AuditInterceptor` и передать `auditInterceptor.UnaryServerInterceptor()` в `extraUnary`
    - Вызвать `grpchelper.NewServer(metricsAdapter, log, serverMetrics, converter, extraUnary, extraStream)`
    - Вызвать `serverMetrics.InitializeMetrics(grpcServer)` после создания сервера
    - _Требования: 2.3, 2.4, 3.3, 4.1, 4.2_

  - [x] 5.3 Передать gRPC-сервер и health-сервер в `api.New()`
    - Обновить вызов `api.New(grpcSrv, healthSrv, tracedCore, auditCh, log)` в соответствии с новой сигнатурой
    - Удалить старый вызов `api.New(ctx, tracedCore, auditCh)`
    - Убедиться, что `grpcSrv` используется далее для `Serve()` и `GracefulStop()`
    - _Требования: 5.4_

- [x] 6. Контрольная точка — полная компиляция и тесты
  - Убедиться, что `go build ./...` проходит без ошибок
  - Убедиться, что все существующие тесты проходят (`go test ./...`)
  - При возникновении вопросов — уточнить у пользователя

- [x] 7. Совместимость метрик с Grafana Dashboard
  - [x] 7.1 Проверить имена экспортируемых метрик
    - Убедиться, что `NewServerMetrics(reg, "easyp", "api")` генерирует метрики с префиксом `easyp_api_grpc_server_*`
    - Проверить наличие метрик: `easyp_api_grpc_server_started_total`, `easyp_api_grpc_server_handled_total`, `easyp_api_grpc_server_handling_seconds_bucket`, `easyp_api_grpc_server_msg_received_total`, `easyp_api_grpc_server_msg_sent_total`
    - Сверить с запросами в `configs/grafana/provisioning/dashboards/service/service.json`
    - _Требования: 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 8. Финальная контрольная точка
  - Убедиться, что все тесты проходят, при возникновении вопросов — уточнить у пользователя

## Примечания

- Задачи, отмеченные `*`, являются опциональными и могут быть пропущены
- Каждая задача ссылается на конкретные требования для трассируемости
- Контрольные точки обеспечивают инкрементальную валидацию
- Ключевое отличие API monitor от logger: `monitor.WithContext` вместо `logger.NewContext`, `monitor.FromContext` вместо `logger.FromContext`
- `monitor.FromContext` возвращает внутренний `sLogger` (не `slog.Default()`) при отсутствии логгера в контексте — fallback-проверки в grpchelper нужно адаптировать
