# План реализации: Улучшения SDK

## Обзор

Пошаговая реализация улучшений Go SDK для EasyP API Service. Каждый шаг строится на предыдущем, начиная с исправления бага компиляции и заканчивая интеграцией всех компонентов. Все изменения затрагивают пакет `sdk/`.

## Задачи

- [x] 1. Исправить баг компиляции в `sdk/client.go`
  - Удалить неиспользуемую переменную `ctx` (и связанный `context.WithTimeout`) из функции `NewClient`
  - Убедиться, что `go build ./sdk/...` проходит без ошибок
  - _Требования: 1.1, 1.2, 1.3_

- [x] 2. Расширить конфигурацию и Option-функции в `sdk/config.go`
  - [x] 2.1 Добавить новые поля в структуру `config`
    - Поля retry: `maxRetries`, `retryBaseDelay`, `retryMaxDelay`
    - Поля таймаутов: `generateCodeTimeout`, `listPluginsTimeout`
    - Поля interceptors: `unaryInterceptors []grpc.UnaryClientInterceptor`
    - Поля health check: `enableHealthCheck`, `healthCheckInterval`
    - Поля keepalive: `keepaliveParams *keepalive.ClientParameters`
    - Обновить `defaultConfig()` со значениями по умолчанию из дизайн-документа (retry: 3/100ms/5s, таймауты: 30s/10s, health: 30s)
    - Добавить необходимые импорты (`time`, `log/slog`, `google.golang.org/grpc/keepalive`)
    - _Требования: 2.1, 2.4, 3.1, 3.2, 6.1, 6.2_

  - [x] 2.2 Реализовать новые Option-функции
    - `WithMaxRetries(n int) Option`
    - `WithRetryBaseDelay(d time.Duration) Option`
    - `WithGenerateCodeTimeout(d time.Duration) Option`
    - `WithListPluginsTimeout(d time.Duration) Option`
    - `WithUnaryInterceptor(i grpc.UnaryClientInterceptor) Option`
    - `WithLoggingInterceptor(logger *slog.Logger) Option` — добавляет logging interceptor в цепочку
    - `WithMetricsInterceptor(collector MetricsCollector) Option` — добавляет metrics interceptor в цепочку
    - `WithHealthCheck(interval time.Duration) Option`
    - `WithKeepaliveParams(params keepalive.ClientParameters) Option`
    - _Требования: 2.1, 3.3, 5.1, 6.1, 6.5_

- [x] 3. Создать `sdk/retry.go` — retry interceptor
  - Реализовать `retryUnaryInterceptor(maxRetries int, baseDelay, maxDelay time.Duration) grpc.UnaryClientInterceptor`
  - Проверка транзиентных кодов: `UNAVAILABLE`, `DEADLINE_EXCEEDED`, `RESOURCE_EXHAUSTED`
  - Экспоненциальный backoff: `min(baseDelay * 2^attempt + jitter, maxDelay)`, jitter до 25%
  - При отмене контекста — немедленный возврат ошибки контекста
  - Возврат последней ошибки после исчерпания попыток
  - _Требования: 2.2, 2.3, 2.4, 2.5, 2.6_

- [x] 4. Создать `sdk/filter.go` — фильтрация плагинов
  - Определить тип `PluginFilter` с полями `Group`, `Name`, `Version`
  - Реализовать функцию `applyFilter(plugins []*generator.PluginInfo, filter PluginFilter) []*generator.PluginInfo`
  - Фильтрация по непустым полям: пустое поле игнорируется, непустое — требует точного совпадения
  - Пустой фильтр (все поля пустые) — возвращает исходный список без изменений
  - _Требования: 4.1, 4.2, 4.3, 4.4, 4.5_

- [x] 5. Создать `sdk/interceptors.go` — logging и metrics interceptors
  - [x] 5.1 Реализовать `loggingUnaryInterceptor(logger *slog.Logger) grpc.UnaryClientInterceptor`
    - Записывать: метод RPC, длительность вызова, код статуса ответа
    - _Требования: 5.3_

  - [x] 5.2 Определить интерфейс `MetricsCollector` и реализовать `metricsUnaryInterceptor`
    - Интерфейс `MetricsCollector` с методом `RecordCall(method string, duration time.Duration, code codes.Code)`
    - Реализовать `metricsUnaryInterceptor(collector MetricsCollector) grpc.UnaryClientInterceptor`
    - _Требования: 5.4_

- [x] 6. Создать `sdk/health.go` — health monitor
  - Определить структуру `healthMonitor` с полями `conn`, `interval`, `stopCh`
  - Реализовать метод `start()` — горутина с тикером, проверяющая `conn.GetState()`
  - При `TRANSIENT_FAILURE` вызывать `conn.Connect()`
  - Реализовать метод `stop()` — закрытие `stopCh` для остановки горутины
  - _Требования: 6.1, 6.2, 6.3, 6.4_

- [x] 7. Интегрировать все компоненты в `sdk/client.go`
  - [x] 7.1 Обновить структуру `Client`
    - Добавить поля `cfg *config` и `health *healthMonitor`
    - _Требования: 6.1_

  - [x] 7.2 Обновить `NewClient` — сборка цепочки interceptors и подключение keepalive
    - Собрать interceptors: retry первый, затем пользовательские
    - Подключить через `grpc.WithChainUnaryInterceptor`
    - Добавить `grpc.WithKeepaliveParams` если настроен
    - Запустить health monitor если включён
    - _Требования: 2.1, 5.2, 5.5, 6.4, 6.5_

  - [x] 7.3 Обновить `GenerateCode` — добавить обёртку таймаута
    - Реализовать вспомогательный метод `withTimeout(ctx, defaultTimeout)` — выбор min(userDeadline, defaultTimeout)
    - Обернуть вызов `GenerateCode` через `withTimeout` с `cfg.generateCodeTimeout`
    - _Требования: 3.1, 3.4, 3.5_

  - [x] 7.4 Обновить `ListPlugins` — добавить фильтрацию и таймаут
    - Изменить сигнатуру на `ListPlugins(ctx context.Context, filter ...PluginFilter)`
    - Обернуть вызов через `withTimeout` с `cfg.listPluginsTimeout`
    - Применить `applyFilter` если фильтр передан и непустой
    - _Требования: 3.2, 3.4, 4.1, 4.5, 4.6_

  - [x] 7.5 Обновить `Close` — остановка health monitor
    - Вызвать `health.stop()` перед закрытием соединения (если health monitor запущен)
    - _Требования: 6.1_

- [x] 8. Контрольная точка — компиляция
  - Убедиться, что `go build ./sdk/...` проходит без ошибок. Все компоненты интегрированы и код компилируется. Задать вопросы пользователю, если что-то неясно.

- [x] 9. Unit-тесты
  - [x] 9.1 Написать unit-тесты для retry interceptor (`sdk/retry_test.go`)
    - Тест значений по умолчанию (3 попытки, 100ms)
    - Тест отмены контекста во время retry
    - Тест нетранзиентных ошибок — без retry
    - _Требования: 2.4, 2.6_

  - [x] 9.2 Написать unit-тесты для фильтрации (`sdk/filter_test.go`)
    - Тест вызова без фильтра — полный список
    - Тест фильтрации по каждому полю отдельно
    - Тест пустого фильтра — полный список
    - _Требования: 4.1, 4.5, 4.6_

  - [x] 9.3 Написать unit-тесты для таймаутов (`sdk/client_test.go`)
    - Тест дефолтных значений 30с/10с
    - Тест `withTimeout` с пользовательским дедлайном
    - _Требования: 3.1, 3.2, 3.4_

  - [x] 9.4 Написать unit-тесты для interceptors (`sdk/interceptors_test.go`)
    - Тест logging interceptor — проверка записи метода, длительности, кода
    - Тест metrics interceptor — проверка вызова `RecordCall`
    - Тест добавления interceptors через Option
    - _Требования: 5.1, 5.3, 5.4_

  - [x] 9.5 Написать unit-тесты для health monitor (`sdk/health_test.go`)
    - Тест включения через Option
    - Тест остановки при `Close()`
    - _Требования: 6.1, 6.2_

- [ ] 10. Property-based тесты (rapid)
  - [ ]* 10.1 Property-тест: применение Option изменяет конфигурацию (`sdk/config_prop_test.go`)
    - **Property 1: Применение Option изменяет конфигурацию**
    - **Validates: Requirements 2.1, 3.3, 6.5**

  - [ ]* 10.2 Property-тест: retry при транзиентных ошибках (`sdk/retry_prop_test.go`)
    - **Property 2: Retry при транзиентных ошибках**
    - **Validates: Requirements 2.2**

  - [ ]* 10.3 Property-тест: экспоненциальный backoff с jitter (`sdk/retry_prop_test.go`)
    - **Property 3: Экспоненциальный backoff с jitter**
    - **Validates: Requirements 2.3**

  - [ ]* 10.4 Property-тест: возврат последней ошибки после исчерпания попыток (`sdk/retry_prop_test.go`)
    - **Property 4: Возврат последней ошибки после исчерпания попыток**
    - **Validates: Requirements 2.5**

  - [ ]* 10.5 Property-тест: выбор минимального дедлайна (`sdk/client_prop_test.go`)
    - **Property 5: Выбор минимального дедлайна**
    - **Validates: Requirements 3.4**

  - [ ]* 10.6 Property-тест: фильтрация возвращает только совпадающие плагины (`sdk/filter_prop_test.go`)
    - **Property 6: Фильтрация возвращает только совпадающие плагины**
    - **Validates: Requirements 4.2, 4.3, 4.4**

  - [ ]* 10.7 Property-тест: пустой фильтр — тождественная операция (`sdk/filter_prop_test.go`)
    - **Property 7: Пустой фильтр — тождественная операция**
    - **Validates: Requirements 4.5**

  - [ ]* 10.8 Property-тест: порядок выполнения interceptors (`sdk/interceptors_prop_test.go`)
    - **Property 8: Порядок выполнения interceptors**
    - **Validates: Requirements 5.2, 5.5**

- [x] 11. Финальная контрольная точка
  - Убедиться, что все тесты проходят (`go test ./sdk/...`). Задать вопросы пользователю, если что-то неясно.

## Примечания

- Задачи с `*` — опциональные, можно пропустить для ускорения MVP
- Каждая задача ссылается на конкретные требования для трассируемости
- Property-тесты используют библиотеку `pgregory.net/rapid`
- Контрольные точки обеспечивают инкрементальную валидацию
