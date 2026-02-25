# Дизайн-документ: Интеграция grpchelper

## Обзор

Интеграция библиотеки `internal/grpchelper` в сервис `easyp-tech/service` для замены ручной настройки gRPC-сервера. Основные задачи:

1. Адаптация импортов grpchelper — замена `sipki-tech/back-template` на локальные пакеты проекта
2. Рефакторинг `internal/api/api.go` — делегирование создания gRPC-сервера в `cmd/main.go` через `grpchelper.NewServer()`
3. Реализация `GRPCCodesConverter` на основе существующей функции `apiError`
4. Обеспечение совместимости метрик с Grafana Dashboard

### Принципы проектирования

- Минимальные изменения в grpchelper — только замена импортов и определение локального интерфейса метрик
- Инверсия зависимостей — grpchelper определяет свой интерфейс `Metrics`, а не импортирует внешний
- Разделение ответственности — создание gRPC-сервера в `cmd/main.go`, регистрация обработчиков в `api.New()`

## Архитектура

### Текущая архитектура

```mermaid
graph TD
    Main["cmd/main.go<br/>run()"] --> ApiNew["api.New()"]
    ApiNew --> GrpcNew["grpc.NewServer()"]
    ApiNew --> Health["health.NewServer()"]
    ApiNew --> Reflection["reflection.Register()"]
    ApiNew --> ErrorInt["errorInterceptor()"]
    ApiNew --> AuditInt["AuditInterceptor"]
    ApiNew --> OtelGrpc["otelgrpc.NewServerHandler()"]
```

### Целевая архитектура

```mermaid
graph TD
    Main["cmd/main.go<br/>run()"] --> ServerMetrics["grpchelper.NewServerMetrics(reg, 'easyp', 'api')"]
    Main --> PanicsCounter["prometheus.NewCounter<br/>easyp_panics_total"]
    Main --> NewServer["grpchelper.NewServer(metrics, log, serverMetrics, converter, extraUnary, extraStream)"]
    NewServer --> GrpcSrv["*grpc.Server"]
    NewServer --> HealthSrv["*health.Server"]
    Main --> ApiNew["api.New(grpcSrv, healthSrv, tracedCore, auditCh, log)"]
    ApiNew --> RegisterAPI["generator.RegisterServiceAPIServer()"]
    ApiNew --> SetStatus["healthSrv.SetServingStatus()"]
```

### Поток данных при создании сервера

```mermaid
sequenceDiagram
    participant Main as cmd/main.go
    participant GH as grpchelper
    participant API as api.New()

    Main->>GH: NewServerMetrics(reg, "easyp", "api")
    GH-->>Main: *grpc_prometheus.ServerMetrics
    Main->>Main: создать panicsCounter (prometheus.Counter)
    Main->>Main: создать converter (GRPCCodesConverterHandler)
    Main->>GH: NewServer(metrics, log, serverMetrics, converter, extraUnary, extraStream)
    GH-->>Main: (*grpc.Server, *health.Server)
    Main->>Main: serverMetrics.InitializeMetrics(grpcServer)
    Main->>API: New(grpcServer, healthServer, tracedCore, auditCh, log)
    API->>API: RegisterServiceAPIServer(grpcServer, api)
    API->>API: healthServer.SetServingStatus("ServiceAPI", SERVING)
```

