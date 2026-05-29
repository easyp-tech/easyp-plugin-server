# epctl CLI — Requirements

**Status:** Draft
**Author:** AI agent
**Date:** 2026-05-29

## Обзор

Создание CLI-утилиты `epctl` как отдельного бинарника для управления сервисом EasyP. Перенос серверного entry point из `cmd/main.go` в `cmd/easyp/main.go` с фиксацией namespace метрик. Замена bash-скриптов (`build-plugins.sh`, `register-plugins.sh`) на подкоманды `epctl`. Обновление Taskfile и Dockerfile.

## Глоссарий

| Термин | Определение | Code Artifact |
|--------|-------------|---------------|
| `path-filter` | Необязательный аргумент вида `group` или `group/name`, ограничивающий операцию до подмножества плагинов | `cmd/epctl/` |
| `plugin.yaml` | YAML-файл в `registry/{group}/{name}/` с описанием плагина: binary, versions, build_args | `registry/` |
| `PluginConfig` | Конфигурация плагина, передаваемая в gRPC `CreatePlugin` (command, env, timeout) | `internal/adapters/registry/registry.go` |
| `serviceNamespace` | Константа с именем сервиса для метрик Prometheus, не зависящая от имени бинарника | `cmd/easyp/main.go` |

## User Stories

- As a **сервисный оператор**, I want собирать плагины одной командой (`epctl plugins build`) so that я не зависел от bash-скриптов и Docker знания.
- As a **сервисный оператор**, I want регистрировать плагины без `grpcurl` (`epctl plugins register`) so that не нужны внешние инструменты.
- As a **сервисный оператор**, I want видеть список плагинов в терминале (`epctl plugins list`) so that я могу быстро проверить состояние сервиса.
- As a **сервисный оператор**, I want валидировать конфиг до запуска сервера (`epctl config validate`) so that ловить ошибки конфигурации заранее.
- As a **CI/CD pipeline**, I want получать JSON-вывод от всех команд (`--output json`) so that парсить результат программно.

## Требования

### 1. Структура проекта

**REQ-1.1** WHEN проект собирается командой `go build ./cmd/easyp/`, the system SHALL создать серверный бинарник из `cmd/easyp/main.go`.

**REQ-1.2** WHEN проект собирается командой `go build ./cmd/epctl/`, the system SHALL создать CLI-бинарник из `cmd/epctl/main.go`.

**REQ-1.3** WHEN сервер запускается из `cmd/easyp/main.go`, the system SHALL использовать константу `serviceNamespace = "easyp"` для namespace всех Prometheus-метрик вместо `filepath.Base(os.Args[0])`.

**REQ-1.4** WHEN `cmd/main.go` (старый entry point) запрашивается в сборке, the system SHALL не компилироваться — файл удалён и перемещён в `cmd/easyp/main.go`.

### 2. epctl plugins build

**REQ-2.1** WHEN оператор запускает `epctl plugins build`, the system SHALL обнаружить все `plugin.yaml` файлы в директории `registry/`, прочитать их, и собрать каждую версию каждого плагина через `docker build --output` в директорию `plugins/{group}/{name}/{version}/`.

**REQ-2.2** WHEN оператор запускает `epctl plugins build <path-filter>`, the system SHALL ограничить сборку плагинами, чей путь `{group}` или `{group}/{name}` совпадает с указанным фильтром.

**REQ-2.3** WHEN бинарник плагина уже существует в `plugins/{group}/{name}/{version}/` (файл `plugin` или файл с именем из поля `binary`), the system SHALL пропустить сборку этой версии и сообщить о кэш-попадании.

**REQ-2.4** WHEN Docker build завершается ошибкой и флаг `--continue-on-error` не установлен, the system SHALL прекратить сборку оставшихся плагинов и завершиться с exit code 1.

**REQ-2.5** WHEN Docker build завершается ошибкой и флаг `--continue-on-error` установлен, the system SHALL продолжить сборку оставшихся плагинов и по завершению вывести суммарный отчёт (сколько собрано, сколько ошибок, сколько из кэша).

**REQ-2.6** WHEN сборка завершена, the system SHALL вывести суммарный отчёт: количество собранных, ошибочных и кэшированных плагинов, общее время выполнения.

**REQ-2.7** WHEN `plugin.yaml` содержит поле `build_args`, the system SHALL передать каждую пару ключ-значение как `--build-arg KEY=VALUE` при вызове `docker build`.

**REQ-2.8** WHEN выходной бинарник не называется `plugin`, the system SHALL переименовать его в `plugin` и установить права `0755`.

**REQ-2.9** WHEN в директории `registry/` не найдено ни одного `plugin.yaml`, the system SHALL вывести предупреждение и завершиться с exit code 0.

### 3. epctl plugins register

**REQ-3.1** WHEN оператор запускает `epctl plugins register --addr <host:port>`, the system SHALL обнаружить все собранные плагины в директории `plugins/` и зарегистрировать каждый через gRPC `CreatePlugin` API.

**REQ-3.2** WHEN оператор запускает `epctl plugins register --addr <host:port> <path-filter>`, the system SHALL ограничить регистрацию плагинами, чей путь `{group}` или `{group}/{name}` совпадает с указанным фильтром.

**REQ-3.3** WHEN gRPC `CreatePlugin` возвращает ошибку `AlreadyExists`, the system SHALL пропустить этот плагин и вывести предупреждение, не прерывая регистрацию остальных.

**REQ-3.4** WHEN gRPC `CreatePlugin` возвращает иную ошибку, the system SHALL прекратить регистрацию и завершиться с exit code 1, выводя описание ошибки.

**REQ-3.5** WHEN регистрация плагина выполняется, the system SHALL сформировать `PluginConfig` с полем `command`, содержащим путь к бинарнику вида `{plugins_prefix}/{group}/{name}/{version}/plugin`.

**REQ-3.6** WHEN флаг `--addr` не указан, the system SHALL использовать значение по умолчанию `localhost:8080`.

**REQ-3.7** WHEN флаг `--plugins-prefix` указан, the system SHALL использовать его значение вместо стандартного `/plugins` при формировании `PluginConfig.command`.

**REQ-3.8** WHEN регистрация завершена, the system SHALL вывести суммарный отчёт: количество зарегистрированных, пропущенных (already exists), и общее количество обработанных плагинов.

### 4. epctl plugins list

**REQ-4.1** WHEN оператор запускает `epctl plugins list --addr <host:port>`, the system SHALL получить список плагинов через gRPC `Plugins` API и вывести таблицу с колонками: group, name, version, tags, created_at.

**REQ-4.2** WHEN флаг `--group`, `--name`, `--version`, или `--tags` указан, the system SHALL передать соответствующие фильтры в gRPC `Plugins` API.

**REQ-4.3** WHEN gRPC `Plugins` API возвращает пустой список, the system SHALL вывести сообщение "No plugins found" и завершиться с exit code 0.

**REQ-4.4** WHEN gRPC-соединение не устанавливается, the system SHALL вывести сообщение об ошибке с адресом сервера и завершиться с exit code 1.

**REQ-4.5** WHEN флаг `--addr` не указан, the system SHALL использовать значение по умолчанию `localhost:8080`.

### 5. epctl config validate

**REQ-5.1** WHEN оператор запускает `epctl config validate <path>`, the system SHALL прочитать YAML-файл, десериализовать его в структуру `config` из `cmd/easyp/main.go`, и сообщить об успехе или ошибках.

**REQ-5.2** WHEN YAML-файл не существует или не читается, the system SHALL вывести сообщение об ошибке с путём к файлу и завершиться с exit code 1.

**REQ-5.3** WHEN YAML содержит неизвестные поля, the system SHALL вывести предупреждение с перечислением неизвестных полей.

**REQ-5.4** WHEN YAML валиден структурно, the system SHALL вывести "Config is valid" и завершиться с exit code 0.

**REQ-5.5** WHEN аргумент `<path>` не указан, the system SHALL вывести usage-справку и завершиться с exit code 1.

### 6. Глобальные флаги и вывод

**REQ-6.1** WHEN флаг `--output json` указан для любой команды, the system SHALL выводить результат в формате JSON вместо human-readable текста.

**REQ-6.2** WHEN флаг `--output` не указан или равен `text`, the system SHALL выводить результат в human-readable формате (таблицы, текстовые сообщения).

**REQ-6.3** WHEN команда завершается ошибкой и `--output json` установлен, the system SHALL вывести JSON-объект с полем `error` и завершиться с exit code 1.

### 7. Инфраструктура

**REQ-7.1** WHEN `Taskfile.yml` таска `build-plugins` выполняется, the system SHALL вызывать `epctl plugins build` вместо `./build-plugins.sh`.

**REQ-7.2** WHEN `Taskfile.yml` таска `register-plugins` выполняется, the system SHALL вызывать `epctl plugins register` вместо `./register-plugins.sh`.

**REQ-7.3** WHEN Docker image собирается из `Dockerfile`, the system SHALL собирать `cmd/easyp/main.go` в бинарник `/easyp` и использовать его как `ENTRYPOINT`.

**REQ-7.4** WHEN файлы `build-plugins.sh` и `register-plugins.sh` присутствуют в репозитории, the system SHALL их удалить — их функциональность полностью заменена `epctl`.

## Топологический порядок

```
REQ-1.1 → REQ-1.3 → REQ-7.3
Причина: серверный бинарник должен переехать и собираться (1.1),
         namespace должен быть зафиксирован (1.3),
         затем Dockerfile обновляется (7.3).

REQ-1.2 → REQ-2.* → REQ-3.* → REQ-7.1, REQ-7.2
Причина: CLI-бинарник должен компилироваться (1.2),
         затем реализуются команды build (2.*) и register (3.*),
         затем Taskfile обновляется (7.1, 7.2).

REQ-4.* (независимый — может выполняться параллельно с REQ-2-3)
REQ-5.* (независимый — может выполняться параллельно с REQ-2-3)
REQ-6.* (зависит от всех команд — реализуется сквозно)
```

## Команды верификации

| Действие | Команда | Источник |
|----------|---------|----------|
| Test | `go test ./...` | Taskfile.yml |
| Build (server) | `go build -o easyp ./cmd/easyp/` | Taskfile.yml |
| Build (CLI) | `go build -o epctl ./cmd/epctl/` | новый |
| Lint | `golangci-lint run ./...` | .golangci.yml |
| Generate | `easyp --cfg easyp.yaml generate` | Taskfile.yml |
