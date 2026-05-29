# Exploration: epctl CLI

## Намерение

Создать CLI-утилиту `epctl` — отдельный бинарник для управления сервисом EasyP. Текущее состояние: вся операционная работа (сборка плагинов, регистрация, диагностика) выполняется через разрозненные bash-скрипты (`build-plugins.sh`, `register-plugins.sh`), Taskfile-таски, и требует внешних инструментов (`grpcurl`, `curl`). Это greenfield-разработка нового бинарника рядом с существующим сервером.

Мотивация:
- Унифицировать операционные инструменты в одном бинарнике
- Убрать зависимость от `grpcurl` и bash-скриптов
- Портировать legacy builder ([.spec/legacy_builder/main.go](file:///Users/zergslaw/Projects/easyp/service/.spec/legacy_builder/main.go)) в продакшн-качество
- Обеспечить кроссплатформенность (bash-скрипты не работают нативно на Windows)

## Исследование

### Текущая структура entry points

Проект имеет один основной бинарник и один smoke-test:

```
cmd/
  main.go              # Сервер — gRPC/HTTP, 472 строки
  mcp-smoke/main.go    # MCP smoke test client, 157 строк
```

Сервер запускается как `go run ./cmd/main.go -cfg config.yml`. Имя бинарника (`filepath.Base(os.Args[0])`) используется как namespace для метрик Prometheus ([cmd/main.go:121](file:///Users/zergslaw/Projects/easyp/service/cmd/main.go#L121)):

```go
appName := filepath.Base(os.Args[0])
```

Это означает, что при переименовании бинарника (`easyp` → `epctl`) namespace метрик изменится. Решение обсуждено с пользователем: захардкодить `const serviceNamespace = "easyp"` для метрик, чтобы отвязать от имени бинарника.

### Bash-скрипты, подлежащие замене

**[build-plugins.sh](file:///Users/zergslaw/Projects/easyp/service/build-plugins.sh)** (45 строк):
- Итерирует `registry/` → ищет Dockerfiles
- Парсит путь `registry/{group}/{name}/{version}/Dockerfile`
- Запускает `docker build --output` → извлекает бинарник в `plugins/`
- Нет параллелизма, нет кэширования, нет прогресс-трекера

**[register-plugins.sh](file:///Users/zergslaw/Projects/easyp/service/register-plugins.sh)** (70 строк):
- Итерирует `plugins/` → ищет бинарники
- Вызывает `grpcurl` → gRPC `CreatePlugin` API
- Генерирует JSON config с путём к бинарнику
- Зависит от внешнего `grpcurl`

### Legacy builder

[.spec/legacy_builder/main.go](file:///Users/zergslaw/Projects/easyp/service/.spec/legacy_builder/main.go) (303 строки) — Go-реализация сборки плагинов с хорошими идеями:
- Читает `plugin.yaml` (не Dockerfiles напрямую)
- Параллельная сборка через `errgroup` с `SetLimit(3)`
- `Tracker` — прогресс-бар с отслеживанием stalled builds
- Кэширование: `needsBuild()` проверяет наличие бинарника
- Запись `build.log` для диагностики ошибок

Проблемы legacy builder:
- Жёстко захардкожен на `apple/swift:v1.25.2` (строки 168-176)
- Standalone `main.go`, не интегрирован в проект
- Нет CLI-фреймворка (flag parsing минимальный)
- Нет тестов

### Формат plugin.yaml

В `registry/` у каждого плагина есть `plugin.yaml` + `Dockerfile`:

```
registry/
  grpc/go/
    plugin.yaml    # binary: protoc-gen-go-grpc, versions: [v1.6.2, v1.5.1, ...]
    Dockerfile
  apple/swift/
    plugin.yaml    # binary: protoc-gen-swift, versions: [v1.38.0, ...]
    Dockerfile
```

Найдено **80 plugin.yaml** файлов и **80 Dockerfiles**. Формат YAML:

```yaml
binary: protoc-gen-go-grpc       # имя бинарника
description: "..."               # опционально
source_url: "..."                # опционально
build_args:                      # опционально, доп. docker build args
  KEY: value
versions:
  - v1.6.2
  - v1.5.1
  # или расширенный формат:
  - version: v1.0.0
```

### SDK клиент

[sdk/](file:///Users/zergslaw/Projects/easyp/service/sdk) уже содержит полноценный Go-клиент для gRPC API:
- `client.go` — `Client` с functional options
- `retry.go` — retry с backoff
- `health.go` — health check
- `filter.go` — client-side plugin filtering
- `interceptors.go` — gRPC interceptors

SDK можно использовать в `epctl` для `plugins list` и `plugins register`.

### Конфигурация сервера

Конфиг валидация ([config.yml](file:///Users/zergslaw/Projects/easyp/service/config.yml), [config.local.yml](file:///Users/zergslaw/Projects/easyp/service/config.local.yml)) — YAML-файл с 30+ параметрами. Сейчас валидация происходит только при запуске сервера (fail-fast). Отдельная `config validate` команда позволит ловить проблемы без запуска.

### Taskfile.yml

[Taskfile.yml](file:///Users/zergslaw/Projects/easyp/service/Taskfile.yml) — 96 строк, содержит таски:
- `up` / `up-minimal` / `down` — docker-compose
- `build-plugins` → вызывает `build-plugins.sh`
- `register-plugins` → вызывает `register-plugins.sh`
- `run` / `setup` — полный цикл
- `run-local` — `go run ./cmd/main.go`
- `generate` / `generate-local` — `easyp generate`

После создания `epctl` Taskfile будет обновлён для вызова `epctl` вместо bash-скриптов.

## Build Tooling

- **Оркестратор:** Taskfile v3 ([Taskfile.yml](file:///Users/zergslaw/Projects/easyp/service/Taskfile.yml))
- **Тесты:** `go test ./...`
- **Сборка:** `go build -o easyp ./cmd/main.go` (сервер), `go build -o epctl ./cmd/epctl/main.go` (CLI — новый)
- **Линтер:** `golangci-lint run ./...` ([.golangci.yml](file:///Users/zergslaw/Projects/easyp/service/.golangci.yml))
- **Кодоген:** `easyp --cfg easyp.yaml generate` (proto → Go stubs + MCP bindings)
- **Источник:** `Taskfile.yml`, `build-plugins.sh`, `register-plugins.sh`

## Рассмотренные варианты

### Вариант A: Единый бинарник с подкомандами

Текущий `cmd/main.go` превращается в multi-command CLI: `easyp serve`, `easyp plugins build`, и т.д.

**Плюсы:**
- Один бинарник для сборки и дистрибуции
- Docker image содержит всё

**Минусы:**
- Namespace метрик ломается (имя бинарника = namespace)
- Docker image раздувается CLI-зависимостями (cobra, docker SDK)
- `plugins build` тянет Docker-зависимости в серверный бинарник
- Нарушает принцип разделения ответственности сервер/клиент

### Вариант B: Два отдельных бинарника (K8s-стиль)

```
cmd/
  easyp/main.go    # Сервер (текущий main.go, минимальные изменения)
  epctl/main.go    # CLI-утилита (новый)
```

**Плюсы:**
- Чистое разделение: сервер остаётся лёгким, CLI — отдельно
- Docker image содержит только `easyp`, без CLI overhead
- Namespace метрик не меняется
- `epctl` можно распространять отдельно (оператор ставит на ноутбук)
- Существующий `cmd/main.go` почти не меняется
- Индустриальный стандарт (kubectl/kube-apiserver, docker/dockerd, etcdctl/etcd)

**Минусы:**
- Два бинарника для сборки/распространения (но GoReleaser уже поддерживает multiple builds)

### Вариант C: Подкоманды в оригинальном easyp CLI

Вложить `plugins build/register/list` и `config validate` в оригинальный `easyp` CLI (другой репозиторий).

**Плюсы:**
- Единая точка входа для разработчика

**Минусы:**
- `plugins build` нуждается в доступе к `registry/`, Dockerfiles — это серверная инфраструктура
- `plugins register` генерирует `config.command` с серверными путями вида `/plugins/grpc/go/v1.5.1/plugin`
- `config validate` валидирует серверный конфиг, не `easyp.yaml`
- `serve` — физически невозможно перенести
- Разные циклы релиза и зависимости

Обсуждено с пользователем и **отвергнуто** — команды тесно связаны с серверной инфраструктурой.

## Ограничения и риски

- **Docker-зависимость для `plugins build`:** команда вызывает `docker build`, Docker daemon должен быть запущен. Это ожидаемо для серверного оператора.
- **GoReleaser:** нужно обновить [.goreleaser.yaml](file:///Users/zergslaw/Projects/easyp/service/.goreleaser.yaml) для сборки двух бинарников.
- **Dockerfile:** нужно обновить [Dockerfile](file:///Users/zergslaw/Projects/easyp/service/Dockerfile) — переименовать `cmd/main.go` → `cmd/easyp/main.go`.
- **Backward compatibility:** bash-скрипты и Taskfile продолжат работать (пока), но постепенно заменяются на `epctl`.
- **`mcp-smoke/`:** оставляем как есть или переносим в `epctl` подкоманду в будущем.

## Рекомендуемое направление

**Вариант B — два бинарника**. Обсуждено и одобрено пользователем.

Структура:
```
cmd/
  easyp/main.go    # Сервер (переезд из cmd/main.go)
  epctl/main.go    # CLI-утилита (новый)
```

CLI-команды `epctl`:
1. **`epctl plugins build`** — сборка плагинов из `registry/` (порт legacy builder)
2. **`epctl plugins register`** — регистрация плагинов через SDK (замена register-plugins.sh + grpcurl)
3. **`epctl plugins list`** — список плагинов через SDK
4. **`epctl config validate`** — валидация серверного YAML-конфига

CLI-фреймворк: стандартная библиотека (`flag` + ручной роутинг подкоманд) или `cobra`. [ASSUMPTION: используем cobra — де-факто стандарт для Go CLI, хорошо поддерживает вложенные подкоманды (`plugins build`, `plugins list`)]

## Границы скоупа

**Must-have (v1):**
- Перенос `cmd/main.go` → `cmd/easyp/main.go` с захардкоженным namespace
- Новый `cmd/epctl/main.go` с CLI-каркасом
- `epctl plugins build` — порт legacy builder с поддержкой всех `plugin.yaml`
- `epctl plugins register --addr host:port` — регистрация через SDK
- `epctl plugins list --addr host:port` — список плагинов через SDK
- `epctl config validate <path>` — валидация YAML-конфига
- Обновление Dockerfile
- Обновление Taskfile.yml

**Deferred (v2):**
- Обновление `.goreleaser.yaml` для двух бинарников
- `epctl plugins inspect <name>` — детальная информация о плагине
- `epctl serve` — прокси к запуску сервера (удобство)
- Перенос `mcp-smoke` в `epctl mcp smoke`
- `epctl migrate` — управление миграциями (status/up/down)
- `epctl health` — проверка здоровья сервиса
- Удаление bash-скриптов (после стабилизации epctl)

**Needs spike:**
- Нет

## Допущения и открытые вопросы

[ASSUMPTION: CLI-фреймворк — cobra. Альтернатива: stdlib `flag` + ручной роутинг, но для вложенных подкоманд (`plugins build`, `plugins list`) cobra значительно удобнее]

[ASSUMPTION: `epctl plugins register` использует SDK из `sdk/` для gRPC-вызовов. SDK уже содержит retry, health check, interceptors]

[ASSUMPTION: `epctl plugins build` вызывает `docker` через `exec.Command`, как legacy builder. Альтернатива — Docker SDK (Go library), но exec проще и достаточен]

[ASSUMPTION: `cmd/easyp/main.go` — минимальные изменения: переезд файла + `const serviceNamespace = "easyp"` вместо `filepath.Base(os.Args[0])`]

**Открытые вопросы:**

1. **Формат вывода `plugins list`:** plain text таблица, JSON, или оба (с флагом `--output json`)? 
2. **`plugins register` — UX:** команда регистрирует ВСЕ плагины из `plugins/` или можно указать конкретный (`epctl plugins register grpc/go:v1.5.1`)?
3. **Cobra vs stdlib:** пользователь предпочитает cobra или минималистичный подход без внешних зависимостей?
