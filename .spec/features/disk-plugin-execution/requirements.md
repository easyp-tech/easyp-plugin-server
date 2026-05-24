# Выполнение плагинов с диска — Требования

**Status:** Draft
**Author:** Antigravity
**Date:** 2026-05-24

## Обзор

Перевод системы выполнения protobuf-плагинов с Docker-in-Docker на прямой вызов процессов внутри контейнера сервиса. Сервис работает в Docker-контейнере, плагины — исполняемые файлы (статические бинарники или скрипты), примонтированные через volume. Изоляция сохраняется на уровне контейнера сервиса. Базовый Docker-образ сервиса — минимальный (`debian:bookworm-slim` + Go binary), расширяемый пользователем через `FROM` для добавления нужных интерпретаторов (Python, Node.js и т.д.).

Конфигурация плагина в JSONB поле `config` таблицы `plugins` переходит с формата `{"docker": {...}}` на формат `{"command": [...], "env": {...}, "timeout": "..."}`, где `command` — массив, описывающий запуск плагина (аналог Docker `ENTRYPOINT`).

## Глоссарий

| Термин | Определение | Code Artifact |
|--------|-------------|---------------|
| `PluginConfig` | Структура конфигурации плагина, десериализуемая из JSONB поля `config` таблицы `plugins`. Содержит `command`, `env`, `timeout` | `internal/adapters/registry/registry.go` |
| `DockerConfig` | Текущая структура Docker-конфигурации (network, memory, cpus и т.д.) — подлежит удалению | `internal/adapters/registry/registry.go` |
| `command` | Массив строк — команда запуска плагина. Первый элемент — исполняемый файл или интерпретатор, остальные — аргументы. Пример: `["python3", "/plugins/.../plugin.py"]` или `["/plugins/.../plugin"]` | `internal/adapters/registry/registry.go` |
| `plugins_dir` | Базовая директория монтирования плагинов внутри контейнера, конфигурируемая через переменную окружения | `cmd/main.go` |

## User Stories

- As a **platform operator**, I want плагины вызывались как процессы с диска, а не через Docker-in-Docker so that не нужен Docker socket внутри контейнера сервиса и снижается latency.
- As a **platform operator**, I want использовать Python/Node.js плагины наравне с Go-бинарниками so that не ограничиваюсь одним языком для плагинов.
- As a **platform operator**, I want расширять базовый Docker-образ сервиса добавлением нужных интерпретаторов so that я контролирую рантаймы и их версии.

## Требования

### Группа 1: Выполнение плагина

**REQ-1.1** WHEN сервис получает запрос на генерацию кода для зарегистрированного плагина, the system SHALL запустить плагин по команде из поля `command` конфигурации через `exec.CommandContext(ctx, command[0], command[1:]...)`, передав сериализованный `CodeGeneratorRequest` в stdin и прочитав `CodeGeneratorResponse` из stdout.

**REQ-1.2** WHEN процесс плагина завершается с ненулевым кодом возврата, the system SHALL вернуть ошибку, содержащую код возврата и содержимое stderr.

**REQ-1.3** WHEN исполняемый файл из `command[0]` не найден по указанному пути, the system SHALL вернуть ошибку с информацией о том, что исполняемый файл не найден.

**REQ-1.4** WHEN выходные данные плагина не могут быть десериализованы как `CodeGeneratorResponse`, the system SHALL вернуть ошибку десериализации.

**REQ-1.5** WHEN в конфигурации плагина задано поле `env`, the system SHALL передать указанные переменные окружения процессу плагина.

**REQ-1.6** WHEN в конфигурации плагина задано поле `timeout`, the system SHALL использовать его как per-plugin таймаут выполнения вместо значения по умолчанию.

**REQ-1.7** WHEN запускается процесс плагина, the system SHALL передать ему **только** переменные окружения, явно указанные в `config.env`, не наследуя окружение сервиса (пароли БД, ключи лицензий и прочие секреты не должны быть доступны плагину).

**REQ-1.8** WHEN запускается процесс плагина, the system SHALL создать для него отдельную process group (`Setpgid`), и при таймауте или отмене контекста — убивать всю process group (родительский процесс и всех его потомков).

**REQ-1.9** WHEN процесс плагина пишет в stdout или stderr, the system SHALL ограничить объём читаемых данных конфигурируемым лимитом (по умолчанию 64MB). При превышении лимита — завершить процесс и вернуть ошибку.

**REQ-1.10** WHEN исполняемый файл из `command[0]` существует, но не имеет прав на исполнение (`permission denied`), the system SHALL вернуть ошибку с информацией о недостаточных правах доступа (отличную от ошибки «файл не найден»).

### Группа 2: Конфигурация плагина в БД

**REQ-2.1** WHEN происходит чтение плагина из БД (метод `Get`), the system SHALL десериализовать JSONB поле `config` в структуру `PluginConfig`, содержащую поля: `command` (массив строк), `env` (map строк, опционально), `timeout` (строка duration, опционально).

**REQ-2.2** WHEN поле `command` в конфигурации плагина пустое, отсутствует или содержит пустой массив, the system SHALL вернуть ошибку при попытке выполнения плагина.

**REQ-2.3** WHEN применяется SQL-миграция, the system SHALL обновить существующие seed-записи в таблице `plugins`: заменить значения `config` с формата `{"docker": {...}}` на формат `{"command": ["{plugins_dir}/{group}/{name}/{version}/plugin"]}`.

**REQ-2.4** WHEN создаётся новый плагин через CRUD-операцию `Create`, the system SHALL валидировать: (a) поле `command` непустое, (b) хотя бы один элемент `command` является абсолютным путём внутри `plugins_dir` (после `filepath.Clean` путь должен начинаться с `plugins_dir`).

**REQ-2.5** WHEN обновляется конфигурация плагина через CRUD-операцию `Update`, the system SHALL применять ту же валидацию `command`, что и при `Create` (REQ-2.4).

**REQ-2.6** WHEN клиент отправляет `config` в старом формате `{"docker": {...}}` через Create или Update API, the system SHALL отклонить запрос с ошибкой валидации (поле `command` отсутствует). Это осознанный breaking change — старый формат более не поддерживается.

### Группа 3: Инициализация Registry

**REQ-3.1** WHEN создаётся экземпляр `Registry` (конструктор `New`), the system SHALL не требовать параметр `domain` (URL Docker Registry), который использовался для формирования имени Docker-образа.

**REQ-3.2** WHEN структура `plugin` возвращается из `Get`, the system SHALL не хранить поле `domain` (Docker Registry URL) в экземпляре плагина.

### Группа 4: Обработка ошибок в WorkerPool

**REQ-4.1** WHEN `WorkerPool` классифицирует ошибку выполнения плагина как транзиентную, the system SHALL не проверять Docker-специфичные коды возврата (125 — daemon error, 126 — cannot invoke, 127 — not found).

**REQ-4.2** WHEN процесс плагина завершается с сигналом (SIGKILL, SIGTERM), the system SHALL классифицировать это как нетранзиентную ошибку.

### Группа 5: Телеметрия

**REQ-5.1** WHEN записывается трассировочный спан выполнения плагина (декоратор `TracingPlugin`), the system SHALL использовать имя спана `process.exec` вместо `docker.exec` и записывать атрибуты: `process.command`, `process.exit_code`.

### Группа 6: Dockerfile и деплой

**REQ-6.1** WHEN собирается базовый Docker-образ сервиса, the system SHALL использовать `debian:bookworm-slim` как базовый образ (glibc-совместимость для Python/Node.js плагинов с native extensions), включать только Go binary сервиса, без Docker CLI и без интерпретаторов.

**REQ-6.2** WHEN собирается Docker-образ сервиса, the system SHALL объявлять директорию для монтирования плагинов через инструкцию `VOLUME`.

**REQ-6.3** WHEN контейнер сервиса запускается, the system SHALL не монтировать Docker socket (`/var/run/docker.sock`).

### Группа 7: Обновление плагинов (hot reload)

**REQ-7.1** WHEN бинарник плагина обновляется в примонтированном volume (новая версия файла), the system SHALL использовать обновлённый файл при следующем вызове без перезапуска сервиса (свойство прямого вызова `exec` — каждый запуск читает файл с диска заново).

## Топологический порядок

```
REQ-2.1 → REQ-2.3 → REQ-1.1 → REQ-1.2, REQ-1.3, REQ-1.4, REQ-1.5, REQ-1.6, REQ-1.7, REQ-1.8, REQ-1.9, REQ-1.10
Причина: Формат конфига (2.1) и миграция данных (2.3) должны быть готовы до замены логики выполнения (1.1).

REQ-3.1 → REQ-3.2 → REQ-1.1
Причина: Конструктор Registry (3.1) и структура plugin (3.2) должны быть обновлены до реализации нового Generate.

REQ-4.1, REQ-4.2 (параллельно с группой 1 — после REQ-1.1)
REQ-5.1 (параллельно — после REQ-1.1)
REQ-6.1, REQ-6.2, REQ-6.3 (независимо — можно в любой момент)
REQ-7.1 (свойство архитектуры — не требует реализации, требует документирования)
REQ-2.4, REQ-2.5, REQ-2.6 (параллельно — после REQ-2.1)
```

## Принятые решения

| Вопрос | Решение |
|--------|---------|
| Формат хранения команды запуска | `command` как массив строк (аналог Docker ENTRYPOINT). Универсально для Go/Python/Node |
| Оставлять ли `config` jsonb | Да — расширяемое хранилище без ALTER TABLE |
| Валидация при Create/Update | `command` непустой + хотя бы один элемент — абсолютный путь внутри `plugins_dir` (path traversal protection) |
| Изоляция env | `cmd.Env` = только явные `config.env`, не наследовать окружение сервиса |
| Process group | `Setpgid: true` + kill всей group при timeout/cancel (защита от zombie/fork-bomb) |
| Интерпретаторы в образе | Базовый образ минимальный (`debian:bookworm-slim`). Пользователь расширяет через `FROM easyp/service:latest` |
| Базовый образ | `debian:bookworm-slim` вместо alpine — glibc-совместимость (как PostgreSQL, Nginx, Node.js) |
| Конвенция путей | `{plugins_dir}/{group}/{name}/{version}/plugin` |
| Output size limit | Ограничение stdout/stderr до 64MB (конфигурируемо). Защита от OOM |
| Breaking change CRUD API | Осознанный: старый формат `{"docker": {...}}` отклоняется. Клиенты обновляются |
| Hot reload | Плагины обновляются в volume без перезапуска сервиса (свойство exec) |

## Verification Commands

| Action   | Command                                 | Source       |
|----------|-----------------------------------------|--------------|
| Test     | `go test ./...`                         | Taskfile.yml |
| Build    | `go build -o main ./cmd/main.go`        | Taskfile.yml |
| Lint     | `golangci-lint run ./...`               | Taskfile.yml |
| Generate | `easyp --cfg easyp.yaml generate`       | Taskfile.yml |
