# План реализации: Generation Worker Pool

## Обзор

Реализация in-memory worker pool для ограничения параллелизма Docker-контейнеров при генерации кода. WorkerPool реализует интерфейс `Registry`, встраиваясь в цепочку декораторов между Core и TracingRegistry. Реализация включает backpressure, таймауты, retry и graceful shutdown.

## Задачи

- [x] 1. Добавить доменные типы и ошибки
  - [x] 1.1 Добавить новые ошибки `ErrServerOverloaded` и `ErrShuttingDown` в `internal/core/domain.go`
    - Добавить `ErrServerOverloaded = errors.New("server overloaded")`
    - Добавить `ErrShuttingDown = errors.New("server shutting down")`
    - _Требования: 2.3, 6.1_

  - [x] 1.2 Добавить маппинг новых ошибок в `apiError` в `internal/api/api.go`
    - `ErrServerOverloaded` → `codes.ResourceExhausted`
    - `ErrShuttingDown` → `codes.Unavailable`
    - _Требования: 2.3, 6.1_

- [x] 2. Реализовать WorkerPool и poolPlugin
  - [x] 2.1 Создать файл `internal/core/pool.go` с типами `WorkerPoolConfig`, `job`, `jobResult`, `WorkerPool`, `poolPlugin`
    - Определить `WorkerPoolConfig` с полями: `Workers`, `QueueSize`, `GenerationTimeout`, `MaxRetries`, `ShutdownTimeout` и yaml/env тегами
    - Определить `job` и `jobResult` структуры
    - Определить `WorkerPool` структуру с полями: `inner Registry`, `jobs chan job`, `cfg`, `logger`, `wg`, `closed atomic.Bool`
    - Определить `poolPlugin` структуру с полями: `inner Plugin`, `cfg`, `logger`
    - _Требования: 1.1, 1.2, 7.1_

  - [x] 2.2 Реализовать конструктор `NewWorkerPool` с нормализацией конфигурации
    - Валидация и нормализация: `workers < 1` → 1, `queue_size < 0` → 0, `generation_timeout == 0` → 120s, `max_retries == 0` → 2, `shutdown_timeout == 0` → 30s
    - Создание буферизированного канала `jobs` с ёмкостью `QueueSize`
    - _Требования: 1.1, 1.2, 1.3, 1.4, 4.4, 5.4, 7.3_

  - [x] 2.3 Реализовать метод `Start(ctx context.Context)` — запуск N горутин-воркеров
    - Каждый воркер читает из канала `jobs` в цикле `for j := range p.jobs`
    - Воркер вызывает `inner.Get` для получения Plugin, оборачивает в `poolPlugin`
    - Отправляет результат в `j.result`
    - _Требования: 1.1, 3.1, 3.2, 3.3_

  - [x] 2.4 Реализовать метод `Get(ctx, pluginGroup, pluginName, pluginVersion)` с backpressure
    - Проверка `closed` флага → `ErrShuttingDown`
    - Non-blocking select для отправки job в канал → `ErrServerOverloaded` при заполненной очереди
    - Блокировка на канале ответа с учётом `ctx.Done()`
    - Логирование backpressure событий на уровне Warn
    - _Требования: 2.1, 2.2, 2.3, 6.1, 8.2_

  - [x] 2.5 Реализовать метод `List(ctx, filter)` — прямое проксирование в `inner.List`
    - _Требования: нет (List не проходит через очередь)_

  - [x] 2.6 Реализовать `poolPlugin.Generate` с таймаутом и retry
    - Создание контекста с `GenerationTimeout`
    - Цикл retry до `MaxRetries + 1` попыток
    - Проверка `ctx.Err()` перед каждой попыткой
    - Логирование retry попыток с номером и причиной
    - Реализовать `poolPlugin.Info` — проксирование в `inner.Info`
    - _Требования: 4.1, 4.2, 4.3, 5.1, 5.5, 8.1, 8.3_

  - [x] 2.7 Реализовать функцию `isTransient(err error)` для определения транзиентных ошибок
    - `exec.ExitError` с кодами 125, 126, 127
    - Подстроки: "connection refused", "daemon", "temporary failure"
    - `context.DeadlineExceeded` НЕ транзиентная
    - _Требования: 5.1_

  - [x] 2.8 Реализовать метод `Shutdown(timeout time.Duration) int`
    - Установка `closed = true`, закрытие канала `jobs`
    - Ожидание завершения воркеров через `wg.Wait` с таймаутом
    - Логирование количества потерянных заданий
    - Возврат количества потерянных заданий
    - _Требования: 6.1, 6.2, 6.3, 6.4, 8.1_

- [x] 3. Контрольная точка — проверка компиляции WorkerPool
  - Убедиться, что `internal/core/pool.go` компилируется без ошибок
  - Убедиться, что `WorkerPool` реализует интерфейс `Registry`
  - Задать вопросы пользователю при возникновении неясностей

- [x] 4. Интеграция в main.go и конфигурацию
  - [x] 4.1 Добавить секцию `worker_pool` в `config.yml` со значениями по умолчанию
    - `workers: 4`, `queue_size: 16`, `generation_timeout: 120s`, `max_retries: 2`, `shutdown_timeout: 30s`
    - _Требования: 7.1, 7.2_

  - [x] 4.2 Добавить поле `WorkerPool core.WorkerPoolConfig` в структуру `config` в `cmd/main.go`
    - Добавить yaml-тег `worker_pool` и env-префикс `WORKER_POOL_`
    - _Требования: 7.2_

  - [x] 4.3 Интегрировать WorkerPool в функцию `run()` в `cmd/main.go`
    - Создать `WorkerPool` после `TracingRegistry`, передать `tracedRegistry` как inner
    - Вызвать `pool.Start(ctx)`
    - Добавить `defer pool.Shutdown(...)` с логированием потерянных заданий
    - Передать `pool` вместо `tracedRegistry` в `core.New`
    - Цепочка: `API → TracingCore → Core → WorkerPool → TracingRegistry → Registry`
    - _Требования: 1.1, 6.1, 6.4, 7.2_

- [x] 5. Контрольная точка — проверка интеграции
  - Убедиться, что проект компилируется без ошибок
  - Убедиться, что цепочка декораторов корректна
  - Задать вопросы пользователю при возникновении неясностей

- [x] 6. Unit-тесты
  - [x] 6.1 Создать файл `internal/core/pool_test.go` с unit-тестами
    - Тест нормализации конфигурации: конкретные примеры (workers=4 не меняется, workers=0 → 1, queue_size=-1 → 0)
    - Тест `Get` — успешный round-trip через пул с mock Registry и mock Plugin
    - Тест `Get` — backpressure: `ErrServerOverloaded` при заполненной очереди
    - Тест `Get` — `ErrShuttingDown` после вызова Shutdown
    - Тест `poolPlugin.Generate` — таймаут: `context.DeadlineExceeded` при медленном Plugin
    - Тест `poolPlugin.Generate` — retry: успех после транзиентной ошибки
    - Тест `poolPlugin.Generate` — retry исчерпаны: возврат последней ошибки
    - Тест `List` — прямое проксирование без очереди
    - Тест `Shutdown` — корректное завершение без потерь
    - Тест `apiError` — маппинг `ErrServerOverloaded` → `ResourceExhausted`, `ErrShuttingDown` → `Unavailable`
    - _Требования: 1.3, 1.4, 2.1, 2.2, 2.3, 4.3, 5.1, 5.3, 6.1, 6.2_

- [ ] 7. Property-тесты
  - [ ]* 7.1 Создать файл `internal/core/pool_prop_test.go` — property-тест для нормализации конфигурации
    - **Свойство 1: Нормализация конфигурации**
    - Генерация случайных `WorkerPoolConfig`, проверка что после нормализации значения корректны
    - **Validates: Требования 1.3, 1.4, 4.4, 5.4**

  - [ ]* 7.2 Property-тест round-trip задания через пул
    - **Свойство 2: Round-trip задания через пул**
    - Генерация случайных параметров плагина, проверка идентичности результата через `WorkerPool.Get` + `poolPlugin.Generate`
    - **Validates: Требования 2.1, 2.2**

  - [ ]* 7.3 Property-тест backpressure при заполненной очереди
    - **Свойство 3: Backpressure при заполненной очереди**
    - Генерация случайных размеров очереди, проверка `ErrServerOverloaded` при переполнении
    - **Validates: Требования 2.3**

  - [ ]* 7.4 Property-тест последовательной обработки в рамках одного воркера
    - **Свойство 4: Последовательная обработка**
    - Генерация последовательностей заданий для пула с 1 воркером, проверка отсутствия пересечений временных интервалов
    - **Validates: Требования 3.3**

  - [ ]* 7.5 Property-тест таймаута генерации
    - **Свойство 5: Таймаут генерации**
    - Генерация случайных таймаутов и задержек, проверка `context.DeadlineExceeded`
    - **Validates: Требования 4.3**

  - [ ]* 7.6 Property-тест повторных попыток при транзиентных ошибках
    - **Свойство 6: Повторные попытки при транзиентных ошибках**
    - Генерация случайных последовательностей ошибок, проверка retry-логики
    - **Validates: Требования 5.1, 5.3**

  - [ ]* 7.7 Property-тест отмены контекста прекращает retry
    - **Свойство 7: Отмена контекста прекращает повторные попытки**
    - Генерация случайных моментов отмены, проверка прекращения retry
    - **Validates: Требования 5.5**

  - [ ]* 7.8 Property-тест shutdown отклоняет новые задания
    - **Свойство 8: Shutdown отклоняет новые задания**
    - Проверка `ErrShuttingDown` для всех вызовов `Get` после `Shutdown`
    - **Validates: Требования 6.1**

  - [ ]* 7.9 Property-тест shutdown дожидается текущих заданий
    - **Свойство 9: Shutdown дожидается текущих заданий**
    - Генерация случайных количеств in-flight заданий, проверка ожидания и подсчёта потерь
    - **Validates: Требования 6.2, 6.3**

- [x] 8. Финальная контрольная точка
  - Убедиться, что все тесты проходят
  - Задать вопросы пользователю при возникновении неясностей

## Примечания

- Задачи с `*` являются опциональными и могут быть пропущены для ускорения MVP
- Каждая задача ссылается на конкретные требования для трассируемости
- Property-тесты используют библиотеку [rapid](https://github.com/flyingmutant/rapid)
- Контрольные точки обеспечивают инкрементальную валидацию
