# local-test-env — Requirements

**Status:** Draft
**Date:** 2026-05-24

## Обзор

Адаптация локального окружения разработчика к новому способу выполнения плагинов (бинарники с диска вместо Docker-контейнеров). Включает: переделку Dockerfile-ов из `registry/` для сборки бинарников через `docker build --output`, скрипты для сборки и регистрации плагинов, очистку docker-compose от устаревших сервисов (registry, docker.sock), обновление конфигов и Taskfile.

## Глоссарий

| Термин | Определение | Code Artifact |
|--------|------------|---------------|
| Plugin binary | Скомпилированный Go-бинарник плагина, принимающий `CodeGeneratorRequest` на stdin и отдающий `CodeGeneratorResponse` на stdout | `./plugins/{group}/{name}/{version}/plugin` |
| `PluginConfig` | JSON-конфигурация плагина с массивом `command`, опциональными `env` и `timeout` | `internal/adapters/registry/registry.go` → `PluginConfig` |
| `build-plugins.sh` | Скрипт сборки plugin binary из Dockerfile-ов через `docker build --output` | `build-plugins.sh` |
| `register-plugins.sh` | Скрипт регистрации собранных плагинов через gRPC API `CreatePlugin` | `register-plugins.sh` |

---

## Требования

### 1. Dockerfile-ы

**REQ-1.1** WHEN `docker build --output=./plugins/{group}/{name}/{version}/ registry/{group}/{name}/{version}/` выполняется для любого Dockerfile из `registry/`, the system SHALL создать файл `./plugins/{group}/{name}/{version}/plugin` — исполняемый Linux/amd64 бинарник.

**REQ-1.2** WHEN Dockerfile собирается, the system SHALL использовать UPX-сжатие для минимизации размера бинарника.

**REQ-1.3** WHEN финальный стейдж Dockerfile описывается, the system SHALL содержать только `FROM scratch` и `COPY` бинарника в `/plugin` — без `ENTRYPOINT`, `USER`, или `/etc/passwd`.

### 2. Скрипт сборки (`build-plugins.sh`)

**REQ-2.1** WHEN `build-plugins.sh` запускается, the system SHALL найти все файлы `Dockerfile` в директории `registry/` рекурсивно и собрать каждый через `docker build --output`.

**REQ-2.2** WHEN `docker build` для любого плагина завершается с ошибкой, the system SHALL немедленно остановить выполнение скрипта с ненулевым exit-кодом (`set -e`).

**REQ-2.3** WHEN `build-plugins.sh` завершается успешно, the system SHALL создать для каждого Dockerfile файл `./plugins/{group}/{name}/{version}/plugin` с правами на исполнение.

### 3. Скрипт регистрации (`register-plugins.sh`)

**REQ-3.1** WHEN `register-plugins.sh` запускается, the system SHALL для каждого собранного плагина в директории `plugins/` вызвать gRPC метод `CreatePlugin` с полями `group`, `name`, `version` и `config.command = ["/plugins/{group}/{name}/{version}/plugin"]`.

**REQ-3.2** WHEN gRPC-вызов `CreatePlugin` возвращает ошибку `ALREADY_EXISTS`, the system SHALL пропустить этот плагин и продолжить с остальными (не завершаться с ошибкой).

**REQ-3.3** WHEN gRPC-вызов `CreatePlugin` возвращает любую другую ошибку (кроме `ALREADY_EXISTS`), the system SHALL немедленно остановить выполнение с ненулевым exit-кодом.

### 4. Конфигурация

**REQ-4.1** WHEN сервис запускается с `config.yml`, the system SHALL использовать поле `registry.plugins_dir` (по умолчанию `/plugins`) вместо устаревшего `registry.domain`.

**REQ-4.2** WHEN сервис запускается с `config.yml`, the system SHALL использовать поле `registry.max_output_size` (по умолчанию `67108864`, 64 МБ).

**REQ-4.3** WHEN сервис запускается с `config.local.yml`, the system SHALL использовать `registry.plugins_dir` указывающий на локальную директорию `./plugins`.

### 5. Docker Compose

**REQ-5.1** WHEN `docker-compose.yml` описывает сервис `service`, the system SHALL монтировать volume `./plugins:/plugins` вместо `/var/run/docker.sock`.

**REQ-5.2** WHEN `docker-compose.yml` описывает инфраструктуру, the system SHALL не содержать сервис `registry` и volume `registry-data`.

### 6. Taskfile

**REQ-6.1** WHEN `task build-plugins` выполняется, the system SHALL вызвать `build-plugins.sh` для сборки всех плагинов.

**REQ-6.2** WHEN `task register-plugins` выполняется, the system SHALL вызвать `register-plugins.sh` для регистрации всех плагинов через gRPC API.

**REQ-6.3** WHEN `task run` выполняется, the system SHALL использовать `build-plugins` вместо устаревшей `local-push-registry` в зависимостях.

**REQ-6.4** WHEN Taskfile описывает таски, the system SHALL не содержать `local-push-registry` и `local-push-required`.

### 7. Удаление устаревших файлов

**REQ-7.1** WHEN проект собран, the system SHALL не содержать файл `push.sh` в корне репозитория.

---

## Порядок зависимостей (Topological Order)

```
REQ-1.1..1.3 → REQ-2.1..2.3 → REQ-3.1..3.3
Причина: Dockerfile-ы (1.x) нужны для скрипта сборки (2.x), который создаёт бинарники для скрипта регистрации (3.x).

REQ-4.1..4.3 (независимы — можно параллельно с 1–3)
REQ-5.1..5.2 (независимы — можно параллельно с 1–3)
REQ-6.1..6.4 → зависят от REQ-2.x и REQ-3.x (таски ссылаются на скрипты)
REQ-7.1 (независим)
```

---

## Команды верификации

| Действие | Команда | Источник |
|----------|---------|----------|
| Test | `go test ./...` | Taskfile.yml |
| Build | `go build -o main ./cmd/main.go` | Taskfile.yml |
| Lint | `golangci-lint run ./...` | Taskfile.yml |
| Generate | `easyp --cfg easyp.yaml generate` | Taskfile.yml |
