# Документ требований

## Введение

Интеграция библиотеки `internal/grpchelper` в сервис для замены текущей ручной настройки gRPC-сервера в `internal/api/api.go` и `cmd/main.go`. Библиотека grpchelper предоставляет полный middleware-стек: grpc-prometheus метрики, OTel-трейсинг, структурированное логирование, recovery, валидацию, извлечение реального IP и keepalive. Интеграция требует адаптации импортов grpchelper (замена `sipki-tech/back-template` на пакеты проекта) и рефакторинга создания gRPC-сервера.

## Глоссарий

- **GRPCHelper**: Библиотека `internal/grpchelper`, предоставляющая фабрику gRPC-сервера с полным middleware-стеком
- **API**: Пакет `internal/api`, реализующий gRPC-обработчики сервиса
- **Monitor**: Пакет `internal/monitor`, предоставляющий функции `FromContext`/`WithContext`/`NewContext` для работы с `*slog.Logger` в контексте
- **ServerMetrics**: Объект `grpc_prometheus.ServerMetrics`, собирающий Prometheus-метрики gRPC-сервера
- **GRPCCodesConverter**: Функция типа `GRPCCodesConverterHandler`, преобразующая ошибки приложения в gRPC status codes
- **AuditInterceptor**: Существующий перехватчик в `internal/api/audit_interceptor.go`, записывающий аудит-события в канал
- **Grafana_Dashboard**: Дашборд в `configs/grafana/provisioning/dashboards/service/service.json`, ожидающий метрики с префиксом `easyp_api_grpc_server_*`
- **Main**: Точка входа `cmd/main.go`, функция `run`, инициализирующая и запускающая gRPC-сервер

## Требования

### Требование 1: Адаптация импортов grpchelper

**User Story:** Как разработчик, я хочу, чтобы grpchelper использовал пакеты текущего проекта, чтобы библиотека компилировалась без внешних зависимостей на `sipki-tech/back-template`.

#### Критерии приёмки

1. THE GRPCHelper SHALL использовать пакет `github.com/easyp-tech/service/internal/monitor` вместо `github.com/sipki-tech/back-template/internal/logger` для функций `FromContext` и `NewContext` работы с логгером в контексте
2. THE GRPCHelper SHALL определять константу `PanicReason` локально или использовать строковый литерал вместо импорта `logger.PanicReason` из внешнего пакета
3. THE GRPCHelper SHALL принимать интерфейс с методом `PanicsTotal` (счётчик паник) вместо импорта `github.com/sipki-tech/back-template/internal/metrics`
4. WHEN все импорты адаптированы, THE GRPCHelper SHALL компилироваться без ошибок в контексте модуля `github.com/easyp-tech/service`

### Требование 2: Создание ServerMetrics в main.go

**User Story:** Как оператор сервиса, я хочу, чтобы gRPC-метрики собирались с правильным namespace и subsystem, чтобы Grafana_Dashboard отображал данные корректно.

#### Критерии приёмки

1. THE Main SHALL создавать ServerMetrics через вызов `grpchelper.NewServerMetrics(reg, "easyp", "api")` с namespace `"easyp"` и subsystem `"api"`
2. THE Main SHALL передавать созданный объект ServerMetrics в функцию создания gRPC-сервера
3. WHEN ServerMetrics создан и зарегистрирован, THE Main SHALL экспортировать метрики с префиксом `easyp_api_grpc_server_*`, совместимым с Grafana_Dashboard
4. THE Main SHALL вызывать `serverMetrics.InitializeMetrics(server)` после создания gRPC-сервера для инициализации метрик

### Требование 3: Реализация GRPCCodesConverter

**User Story:** Как разработчик, я хочу, чтобы ошибки приложения автоматически преобразовывались в gRPC status codes через grpchelper, чтобы заменить текущий ручной `errorInterceptor`.

#### Критерии приёмки

1. THE API SHALL предоставлять функцию типа `GRPCCodesConverterHandler`, реализующую маппинг ошибок `core.ErrNotFound` → `codes.NotFound`, `core.ErrInvalidPluginName` → `codes.InvalidArgument`, `core.ErrGenerationFailed` → `codes.Internal`, `core.ErrServerOverloaded` → `codes.ResourceExhausted`, `core.ErrShuttingDown` → `codes.Unavailable`, `context.DeadlineExceeded` → `codes.DeadlineExceeded`, `context.Canceled` → `codes.Canceled`
2. IF ошибка не соответствует ни одному известному типу, THEN THE GRPCCodesConverter SHALL возвращать `codes.Internal`
3. THE GRPCCodesConverter SHALL передаваться в `grpchelper.NewServer` в качестве параметра `converter`

### Требование 4: Рефакторинг создания gRPC-сервера

**User Story:** Как разработчик, я хочу заменить ручную настройку gRPC-сервера в `api.New()` на вызов `grpchelper.NewServer()`, чтобы получить полный middleware-стек.

#### Критерии приёмки

1. THE API SHALL использовать `grpchelper.NewServer()` для создания gRPC-сервера вместо прямого вызова `grpc.NewServer()`
2. THE API SHALL передавать `AuditInterceptor.UnaryServerInterceptor()` в параметре `extraUnary` при вызове `grpchelper.NewServer()`
3. THE API SHALL удалить ручную регистрацию health check и reflection, так как `grpchelper.NewServer()` выполняет их автоматически
4. THE API SHALL удалить функцию `errorInterceptor`, так как её функциональность заменяется `grpchelper.UnaryConvertCodesServerInterceptor` с GRPCCodesConverter
5. THE API SHALL удалить прямой импорт `otelgrpc`, так как OTel-трейсинг настраивается внутри `grpchelper.NewServer()`
6. WHEN `grpchelper.NewServer()` возвращает `(*grpc.Server, *health.Server)`, THE API SHALL использовать возвращённый `*health.Server` для установки статуса сервиса через `SetServingStatus`

### Требование 5: Обновление сигнатуры api.New

**User Story:** Как разработчик, я хочу, чтобы функция `api.New()` принимала уже созданный gRPC-сервер и health-сервер, чтобы их создание контролировалось из `cmd/main.go`.

#### Критерии приёмки

1. THE API SHALL изменить сигнатуру функции `New` так, чтобы она принимала `*grpc.Server` и `*health.Server` вместо создания их внутри
2. THE API SHALL регистрировать обработчик `generator.RegisterServiceAPIServer` на переданном gRPC-сервере
3. THE API SHALL устанавливать статус `SERVING` для `ServiceAPI` на переданном health-сервере
4. THE Main SHALL создавать gRPC-сервер через `grpchelper.NewServer()` и передавать результат в `api.New()`

### Требование 6: Интеграция recovery и panic-метрик

**User Story:** Как оператор сервиса, я хочу, чтобы паники в gRPC-обработчиках перехватывались и учитывались в метриках, чтобы сервис оставался стабильным.

#### Критерии приёмки

1. THE GRPCHelper SHALL принимать интерфейс с методом инкремента счётчика паник для использования в recovery-перехватчике
2. WHEN паника происходит в gRPC-обработчике, THE GRPCHelper SHALL инкрементировать счётчик паник и возвращать ошибку `codes.Internal` клиенту
3. WHEN паника происходит в gRPC-обработчике, THE GRPCHelper SHALL логировать причину паники и стектрейс через логгер из контекста

### Требование 7: Совместимость с Grafana Dashboard

**User Story:** Как оператор сервиса, я хочу, чтобы после интеграции grpchelper все панели Grafana_Dashboard продолжали работать без изменений.

#### Критерии приёмки

1. THE ServerMetrics SHALL экспортировать метрику `easyp_api_grpc_server_started_total` для подсчёта начатых gRPC-вызовов
2. THE ServerMetrics SHALL экспортировать метрику `easyp_api_grpc_server_handled_total` с лейблом `grpc_code` для подсчёта завершённых gRPC-вызовов
3. THE ServerMetrics SHALL экспортировать метрику `easyp_api_grpc_server_handling_seconds_bucket` для гистограммы латентности
4. THE ServerMetrics SHALL экспортировать метрики `easyp_api_grpc_server_msg_received_total` и `easyp_api_grpc_server_msg_sent_total` для подсчёта сообщений
5. WHEN Grafana_Dashboard запрашивает метрики по шаблону `easyp_api_grpc_server_*`, THE ServerMetrics SHALL предоставлять данные с лейблами `instance`, `job`, `grpc_method`
