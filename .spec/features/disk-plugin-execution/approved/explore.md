# Exploration: Вызов плагинов с диска вместо Docker

## Намерение

Текущая архитектура выполняет protobuf-плагины через `docker run` — каждый запрос на генерацию **из контейнера сервиса** порождает вложенный Docker-контейнер (Docker-in-Docker через монтированный socket), передаёт `CodeGeneratorRequest` через stdin и получает `CodeGeneratorResponse` через stdout. Предлагается **заменить вложенный Docker-вызов на прямой запуск бинарных плагинов с диска внутри контейнера сервиса**. Плагины будут пробрасываться через Docker volume.

**Модель деплоя:** Сам сервис работает в Docker-контейнере. Плагины — статические бинарники, примонтированные через volume (например `/plugins/{group}/{name}/{version}`). Изоляция сохраняется на уровне контейнера сервиса.

**Мотивация:** отказ от Docker-in-Docker (не нужен Docker socket внутри контейнера), снижение latency (нет overhead создания вложенного контейнера), упрощение архитектуры, исключение Docker Registry из инфраструктуры.

## Исследование

### Текущая архитектура вызова плагинов

Цепочка вызовов прослежена по исходному коду:

1. **[api.go:50-62](file:///Users/zergslaw/Projects/easyp/service/internal/api/api.go#L50-L62)** — gRPC handler `GenerateCode` преобразует proto в `core.GenerateCodeRequest`
2. **[tracing_core.go:34-48](file:///Users/zergslaw/Projects/easyp/service/internal/telemetry/tracing_core.go#L34-L48)** — `TracingCore` добавляет span "core.Generate"
3. **[core.go:47-93](file:///Users/zergslaw/Projects/easyp/service/internal/core/core.go#L47-L93)** — `Core.Generate()`:
   - Парсит имя плагина: `getGroup()` + `getNameAndVersion()`
   - Вызывает `registry.Get(ctx, group, name, version)` → получает `Plugin`
   - Вызывает `plugin.Generate(ctx, req.Payload)` → получает `CodeGeneratorResponse`
   - Записывает метрики и аудит
4. **[pool.go:182-226](file:///Users/zergslaw/Projects/easyp/service/internal/core/pool.go#L182-L226)** — `WorkerPool.Get()`:
   - Неблокирующая очередь (buffered channel)
   - Воркер-горутина вызывает `inner.Get()` → оборачивает результат в `poolPlugin`
5. **[tracing_registry.go:35-56](file:///Users/zergslaw/Projects/easyp/service/internal/telemetry/tracing_registry.go#L35-L56)** — `TracingRegistry.Get()`:
   - Span "registry.Get"
   - Оборачивает Plugin в `TracingPlugin`
6. **[registry.go:83-117](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L83-L117)** — `Registry.Get()`:
   - SQL-запрос к PostgreSQL: `SELECT ... FROM plugins WHERE group_name=$1 AND name=$2 AND version=$3`
   - Парсит `config` (JSON) → `PluginConfig` с Docker-настройками
   - Возвращает `&plugin{}` (реализует `core.Plugin`)
7. **[registry.go:196-280](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L196-L280)** — `plugin.Generate()` — **КЛЮЧЕВОЙ УЧАСТОК**:
   - `proto.Marshal(req)` → бинарные данные
   - Формирует `docker run --rm -i --network=none --memory=128m --cpus=1.0 IMAGE`
   - Применяет конфиг из БД: network, memory, cpus, user, env, tmpfs, read-only
   - `exec.CommandContext(ctx, "docker", args...)` → stdin/stdout
   - `proto.Unmarshal(output)` → `CodeGeneratorResponse`
8. **[pool.go:249-301](file:///Users/zergslaw/Projects/easyp/service/internal/core/pool.go#L249-L301)** — `poolPlugin.Generate()`:
   - Добавляет timeout (120s default)
   - Retry транзиентных ошибок (Docker exit 125/126/127, connection refused)
9. **[tracing_plugin.go:43-91](file:///Users/zergslaw/Projects/easyp/service/internal/telemetry/tracing_plugin.go#L43-L91)** — `TracingPlugin.Generate()`:
   - Span "plugin.Generate" + дочерний span "docker.exec"
   - Histogram `plugin.execution.duration`
   - Pyroscope tag wrapping

### Что привязано к Docker

| Компонент | Файл | Docker-зависимость |
|---|---|---|
| `plugin.Generate()` | `adapters/registry/registry.go:196-280` | `exec.CommandContext("docker", ...)` — основной вызов |
| `DockerConfig` struct | `adapters/registry/registry.go:32-41` | Конфиг для Docker: network, memory, cpus, user, env, tmpfs |
| `PluginConfig` struct | `adapters/registry/registry.go:44-46` | Обёртка: `Docker *DockerConfig` |
| `plugin.domain` field | `adapters/registry/registry.go:64` | URL Docker registry для формирования image name |
| `Registry.domain` field | `adapters/registry/registry.go:52` | Домен registry (`localhost:5005`) |
| `isTransient()` | `core/pool.go:309-340` | Проверяет Docker exit codes 125/126/127 |
| `TracingPlugin` | `telemetry/tracing_plugin.go:52-56` | Span "docker.exec" с атрибутами Docker |
| `registryConfig.Domain` | `cmd/main.go:77-78` | Конфиг `REGISTRY_DOMAIN` |
| Docker-compose registry | `docker-compose.yml` | Сервис `registry` на порту 5005 |
| Plugin Dockerfiles | `registry/` | Multi-stage Dockerfile'ы плагинов |
| Push script | `push.sh` | `docker build + docker push` |

### Протокол общения с плагином

Протокол **не зависит от Docker** — это стандартный протобуф-плагин протокол:
- stdin: `proto.Marshal(CodeGeneratorRequest)`
- stdout: бинарные данные → `proto.Unmarshal → CodeGeneratorResponse`

Любой бинарник на диске, читающий stdin и пишущий stdout в этом формате, будет работать.

### Схема БД

Таблица `plugins` ([1.init.sql](file:///Users/zergslaw/Projects/easyp/service/migrate/1.init.sql)):
- `config jsonb` — сейчас хранит `DockerConfig`
- При переходе на диск — поле `config` должно хранить `DiskConfig` (путь к бинарнику) или гибридную структуру

### Тестирование

Проект использует стандартный `go test ./...`. Тестов на `Registry.Get()` и `plugin.Generate()` с реальным Docker нет — они требуют Docker daemon.

## Build Tooling

- **Оркестратор:** Taskfile v3 (`Taskfile.yml`)
- **Тест:** `go test ./...`
- **Сборка:** `go build -o main ./cmd/main.go`
- **Линтер:** `golangci-lint run ./...`
- **Генерация:** `easyp --cfg easyp.yaml generate`
- **Миграции:** автоматические при старте (через `migrations.Run`)
- **Источник:** [Taskfile.yml](file:///Users/zergslaw/Projects/easyp/service/Taskfile.yml)

## Рассмотренные варианты

### Вариант A: Прямая замена `docker run` → `exec.CommandContext(binary)` внутри контейнера сервиса

Плагин — статический бинарник, примонтированный через Docker volume в контейнер сервиса. `plugin.Generate()` вызывает его напрямую через `exec.CommandContext(ctx, binaryPath)` — без порождения вложенного Docker-контейнера.

**Плюсы:**
- Минимальные изменения — заменяется только `registry.go:196-280`
- Протокол stdin/stdout остаётся тем же
- Нет зависимости от Docker daemon/socket внутри контейнера сервиса
- Значительно ниже latency (нет создания вложенного контейнера)
- Изоляция сохраняется на уровне контейнера сервиса
- Ресурсные лимиты задаются на контейнер сервиса (memory, CPU) — как и раньше
- Исключается Docker Registry из инфраструктуры

**Минусы:**
- Нет per-plugin ресурсных лимитов (ранее каждый плагин имел `--memory=128m`, `--cpus=1.0`)
- Нужно управлять путями к бинарникам (volume mount convention)
- Изменение `config` jsonb в БД: вместо DockerConfig → путь к бинарнику

**Сложность:** Средняя

### Вариант B: Абстракция Runner (интерфейс) — Docker или Disk

Ввести интерфейс `PluginRunner` с двумя реализациями: `DockerRunner` (текущий) и `DiskRunner` (новый). Выбор через конфигурацию.

**Плюсы:**
- Гибкость: можно использовать Docker и Disk одновременно
- Обратная совместимость

**Минусы:**
- Больше кода
- Сложнее тестирование
- Оверинжиниринг: Docker-in-Docker полностью заменяется, поддержка двух runner'ов не нужна

**Сложность:** Высокая

## Ограничения и риски

1. **Изоляция сохраняется** — сервис работает в Docker-контейнере, плагины пробрасываются через volume. Контейнер сервиса обеспечивает изоляцию от хоста (сеть, файловая система). Вредоносный плагин ограничен sandbox'ом контейнера.
2. **Потеря per-plugin ресурсных лимитов** — ранее каждый вложенный контейнер имел `--memory=128m`, `--cpus=1.0`. Теперь все плагины делят ресурсы контейнера сервиса. Это приемлемо: WorkerPool и так ограничивает параллелизм (N воркеров), а лимиты контейнера сервиса задаются в docker-compose/k8s.
3. **Volume mount convention** — бинарники плагинов должны быть примонтированы в известное место (например `/plugins/`). Нужна конвенция именования.
4. **Совместимость** — `isTransient()` в pool.go проверяет Docker exit codes (125/126/127). Для обычных процессов эти коды не актуальны — нужно пересмотреть.
5. **Миграция конфига** — поле `config` в таблице `plugins` содержит `DockerConfig`. Нужна миграция схемы.
6. **Обратная совместимость API** — gRPC API и SDK не затрагиваются (они работают с `CodeGeneratorRequest/Response`).
7. **TracingPlugin** — span "docker.exec" нужно переименовать → "process.exec".
8. **docker-compose** — сервис `registry` (Docker Registry v2) становится ненужным.
9. **`push.sh` и `registry/`** — Dockerfile'ы плагинов и push-скрипт теряют актуальность.
10. **Docker socket** — больше не нужен: убрать mount `/var/run/docker.sock` из docker-compose/deployment.

## Рекомендуемое направление

**Вариант A — прямая замена Docker на вызов бинарника с диска.**

Обоснование:
- Задача явно формулируется как отказ от Docker, а не как добавление альтернативы
- Минимальный объём изменений при максимальном эффекте
- Протокол stdin/stdout не меняется
- Декораторы (TracingCore, TracingRegistry, TracingPlugin, WorkerPool) остаются, меняется только нижний слой

## Границы скоупа

### Must-have (v1):
- Замена `plugin.Generate()`: `docker run` → `exec.CommandContext(binaryPath)`
- Новый `PluginConfig` с путём к бинарнику вместо Docker-настроек
- SQL-миграция: обновление `config` jsonb schema
- Адаптация `isTransient()` для обычных процессов
- Обновление `TracingPlugin`: "docker.exec" → "process.exec"
- Обновление `Registry.New()`: убрать domain parameter
- Обновление конфига в `cmd/main.go`: убрать `registryConfig.Domain`
- Удаление `DockerConfig`, `PluginConfig.Docker`

### Deferred (v2):
- Автоматическое скачивание/обновление бинарников плагинов
- Per-plugin ресурсные лимиты (cgroups / rlimit)
- Удаление `registry/` (Dockerfile'ы) и `push.sh`
- Чистка `docker-compose.yml` от сервиса `registry`

### Needs spike:
- Нет

## Допущения и открытые вопросы

### Допущения

1. [ASSUMPTION: сервис деплоится в Docker-контейнере. Плагины монтируются через volume. Изоляция обеспечивается контейнером сервиса]
2. [ASSUMPTION: бинарники плагинов — статические, уже присутствуют в volume на момент запуска контейнера]
3. [ASSUMPTION: протокол взаимодействия stdin/stdout protobuf остаётся без изменений]
4. [ASSUMPTION: обратная совместимость gRPC API не нарушается — клиенты не знают о способе выполнения плагина]
5. [ASSUMPTION: ресурсные лимиты контейнера сервиса (memory, CPU) достаточны для N параллельных плагинов, где N = WorkerPool.Workers]
5. [ASSUMPTION: `latest` версия будет резолвиться через БД как и раньше, а конкретный путь к бинарнику будет в поле `config`]

### Решения по открытым вопросам

1. **Конвенция путей в volume:** Принята конвенция `{plugins_dir}/{group}/{name}/{version}/plugin`.
2. **Что хранить в `config` в БД:** В конфигурации плагина (jsonb) будет явно храниться путь до бинарника (например, `{"binary_path": "..."}`).
3. **Dockerfile сервиса:** Будет обновлен — удалим зависимость от Docker CLI и добавим инструкцию `VOLUME` для директории с плагинами.
