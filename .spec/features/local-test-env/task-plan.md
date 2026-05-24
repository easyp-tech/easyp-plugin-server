# local-test-env — Task Plan

**Work Type:** Migration
**Date:** 2026-05-24

---

**Test Style Source:** Tier 2
- Evidence: `internal/adapters/registry/registry_test.go` (шаблон удалён, но паттерны известны), `internal/core/pool_test.go`
- Key patterns: стандартный `go test`, table-driven tests, `testing.T`. PBT unavailable — targeted unit tests as substitute.

**Commands:**

| Action | Command | Source |
|--------|---------|--------|
| Test | `go test ./...` | Taskfile.yml |
| Build | `go build -o main ./cmd/main.go` | Taskfile.yml |
| Lint | `golangci-lint run ./...` | Taskfile.yml |
| Generate | `easyp --cfg easyp.yaml generate` | Taskfile.yml |

---

## Матрица покрытия

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1 | T-2 | CP-1 (Equivalence) |
| REQ-1.2 | T-2 | CP-2 (Propagation) |
| REQ-1.3 | T-2 | CP-1 (Equivalence) |
| REQ-2.1 | T-3 | CP-4 (Equivalence) |
| REQ-2.2 | T-3 | CP-3 (Absence) |
| REQ-2.3 | T-3 | CP-4 (Equivalence) |
| REQ-3.1 | T-4 | CP-6 (Propagation) |
| REQ-3.2 | T-4 | CP-5 (Absence) |
| REQ-3.3 | T-4 | CP-7 (Absence) |
| REQ-4.1 | T-5 | CP-8 (Propagation) |
| REQ-4.2 | T-5 | CP-8 (Propagation) |
| REQ-4.3 | T-5 | CP-8 (Propagation) |
| REQ-5.1 | T-5 | CP-9, CP-10 (Absence, Propagation) |
| REQ-5.2 | T-5 | CP-9 (Absence) |
| REQ-6.1 | T-5 | CP-12 (Equivalence) |
| REQ-6.2 | T-5 | CP-12 (Equivalence) |
| REQ-6.3 | T-5 | CP-13 (Propagation) |
| REQ-6.4 | T-5 | CP-11 (Absence) |
| REQ-7.1 | T-5 | CP-14 (Absence) |

---

## T-1: GREEN — Сохранение текущего поведения (Preservation Tests)

*_Requirements: REQ-4.1, REQ-4.2_*
*_Complexity: mechanical_*

GOAL: Убедиться, что существующие Go-тесты проходят ДО внесения изменений.

1. **Запустить `go build -o main ./cmd/main.go`** — убедиться, что проект компилируется.
2. **Запустить `go test ./...`** — зафиксировать baseline. Все тесты должны проходить.

---

## T-2: CODE — Переделать Dockerfile-ы в `registry/`

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.3_*
*_Preservation: CP-2_*
*_Complexity: mechanical_*

GOAL: Упростить все 4 Dockerfile-а: убрать `ENTRYPOINT`, `USER`, `/etc/passwd`. Финальный стейдж — `FROM scratch` + `COPY ... /plugin`.

1. **Изменить `registry/protocolbuffers/go/v1.36.10/Dockerfile`:**
   - Убрать строки: `COPY --from=build --link /etc/passwd /etc/passwd`, `USER nobody`, `ENTRYPOINT [ "/protoc-gen-go" ]`.
   - Заменить `COPY --from=build --link --chown=root:root /go/bin/protoc-gen-go /protoc-gen-go` на `COPY --from=build /go/bin/protoc-gen-go /plugin`.

2. **Изменить `registry/grpc/go/v1.5.1/Dockerfile`:**
   - Убрать строки: `COPY --from=build --link /etc/passwd /etc/passwd`, `USER nobody`, `ENTRYPOINT [ "/protoc-gen-go-grpc" ]`.
   - Заменить `COPY --from=build --link --chown=root:root /go/bin/protoc-gen-go-grpc /protoc-gen-go-grpc` на `COPY --from=build /go/bin/protoc-gen-go-grpc /plugin`.

3. **Изменить `registry/grpc-ecosystem/gateway/v2.27.3/Dockerfile`:**
   - Убрать строки: `COPY --from=build --link --chown=root:root /etc/passwd /etc/passwd`, `USER nobody`, `ENTRYPOINT [ "/protoc-gen-grpc-gateway" ]`.
   - Заменить `COPY --from=build --link --chown=root:root /go/bin/protoc-gen-grpc-gateway /protoc-gen-grpc-gateway` на `COPY --from=build /go/bin/protoc-gen-grpc-gateway /plugin`.

4. **Изменить `registry/grpc-ecosystem/openapiv2/v2.27.3/Dockerfile`:**
   - Убрать строки: `COPY --from=build --link --chown=root:root /etc/passwd /etc/passwd`, `USER nobody`, `ENTRYPOINT [ "/protoc-gen-openapiv2" ]`.
   - Заменить `COPY --from=build --link /go/bin/protoc-gen-openapiv2 /protoc-gen-openapiv2` на `COPY --from=build /go/bin/protoc-gen-openapiv2 /plugin`.

---

## T-3: CODE — Создать скрипт `build-plugins.sh`

*_Requirements: REQ-2.1, REQ-2.2, REQ-2.3_*
*_Preservation: CP-1, CP-2_*
*_Complexity: standard_*

GOAL: Создать bash-скрипт, который обходит все Dockerfile-ы в `registry/` и собирает бинарники через `docker build --output`.

1. **Создать файл `build-plugins.sh` в корне проекта:**
   - Шебанг: `#!/bin/bash`
   - `set -euo pipefail`
   - `export DOCKER_BUILDKIT=1`
   - Обход: `find registry -name Dockerfile | sort`
   - Для каждого Dockerfile: извлечь `{group}/{name}/{version}` из пути `registry/{group}/{name}/{version}/Dockerfile`
   - Вызов: `docker build --output="./plugins/${group}/${name}/${version}/" "registry/${group}/${name}/${version}/"`
   - Проверка: `chmod +x "./plugins/${group}/${name}/${version}/plugin"`
   - Вывод прогресса: `echo "✓ Built {group}/{name}:{version}"`

2. **Сделать `build-plugins.sh` исполняемым:** `chmod +x build-plugins.sh`

---

## T-4: CODE — Создать скрипт `register-plugins.sh`

*_Requirements: REQ-3.1, REQ-3.2, REQ-3.3_*
*_Preservation: CP-1, CP-2_*
*_Complexity: standard_*

GOAL: Создать bash-скрипт, который обходит собранные плагины в `plugins/` и регистрирует их через gRPC API `CreatePlugin`.

1. **Создать файл `register-plugins.sh` в корне проекта:**
   - Шебанг: `#!/bin/bash`
   - `set -euo pipefail`
   - Переменная: `GRPC_HOST="${1:-localhost:8080}"` (host по умолчанию)
   - Обход: `find plugins -name plugin -type f | sort`
   - Для каждого найденного `plugins/{group}/{name}/{version}/plugin`: извлечь `group`, `name`, `version` из пути.
   - Вызов `grpcurl`:
     ```
     grpcurl -plaintext -d "{\"group\":\"${group}\",\"name\":\"${name}\",\"version\":\"${version}\",\"config\":{\"command\":[\"/plugins/${group}/${name}/${version}/plugin\"]}}" "${GRPC_HOST}" api.generator.v1.ServiceAPI/CreatePlugin
     ```
   - Обработка ошибок: если `grpcurl` exit-код != 0, проверить stderr на `ALREADY_EXISTS`. Если да — вывести `"⚠ Already exists: {group}/{name}:{version}"` и `continue`. Иначе — `exit 1`.
   - Вывод прогресса: `echo "✓ Registered {group}/{name}:{version}"`

2. **Сделать `register-plugins.sh` исполняемым:** `chmod +x register-plugins.sh`

---

## T-5: CODE — Обновить инфраструктуру (конфиги, compose, Taskfile, очистка)

*_Requirements: REQ-4.1, REQ-4.2, REQ-4.3, REQ-5.1, REQ-5.2, REQ-6.1, REQ-6.2, REQ-6.3, REQ-6.4, REQ-7.1_*
*_Preservation: CP-1, CP-2, CP-3, CP-4_*
*_Complexity: standard_*

GOAL: Привести конфиги, docker-compose, Taskfile в соответствие с новой архитектурой и удалить устаревшие файлы.

1. **Изменить `config.yml` (строка 12–13):**
   - Заменить:
     ```yaml
     registry:
       domain: "localhost:5005"
     ```
   - На:
     ```yaml
     registry:
       plugins_dir: "/plugins"
       max_output_size: 67108864
     ```

2. **Изменить `config.local.yml` (строка 12–13):**
   - Заменить:
     ```yaml
     registry:
       domain: "localhost:5005"
     ```
   - На:
     ```yaml
     registry:
       plugins_dir: "./plugins"
       max_output_size: 67108864
     ```

3. **Изменить `docker-compose.yml`:**
   - Удалить volume `registry-data:` из секции `volumes:` (строка 7).
   - Удалить сервис `registry:` целиком (строки 163–172).
   - В сервисе `service` → `volumes:` — удалить строку `- "/var/run/docker.sock:/var/run/docker.sock"` (строка 211). Добавить `- "./plugins:/plugins:ro"`.

4. **Изменить `Taskfile.yml`:**
   - Удалить таску `local-push-registry:` (строки 27–32).
   - Удалить таску `local-push-required:` (строки 34–43).
   - Добавить таску `build-plugins:`:
     ```yaml
     build-plugins:
       dir: "{{.USER_WORKING_DIR}}"
       preconditions:
         - "test -f build-plugins.sh"
       cmds:
         - "./build-plugins.sh"
     ```
   - Добавить таску `register-plugins:`:
     ```yaml
     register-plugins:
       dir: "{{.USER_WORKING_DIR}}"
       preconditions:
         - "test -f register-plugins.sh"
       cmds:
         - "./register-plugins.sh"
     ```
   - В таске `run:` → `deps:` — заменить `"local-push-registry"` на `"build-plugins"`.
   - В таске `up-minimal:` → `cmds:` — убрать `registry` из аргументов `docker compose up -d postgres registry` → `docker compose up -d postgres`.

5. **Удалить файл `push.sh`:** `rm push.sh`

---

## T-6: VERIFY — Проверка результата

*_Requirements: REQ-1.1, REQ-2.1, REQ-4.1, REQ-5.1, REQ-6.1, REQ-7.1_*
*_Complexity: standard_*

GOAL: Убедиться, что всё работает после всех изменений.

1. **Запустить `go build -o main ./cmd/main.go`** — проект должен компилироваться.
2. **Запустить `go test ./...`** — все существующие тесты должны проходить.
3. **Проверить `push.sh` удалён:** `test ! -f push.sh`
4. **Проверить что `docker-compose.yml` не содержит `registry`:** `grep -c 'registry' docker-compose.yml` — должно быть 0.
5. **Проверить что `Taskfile.yml` не содержит `local-push`:** `grep -c 'local-push' Taskfile.yml` — должно быть 0.
6. **Проверить что `config.yml` содержит `plugins_dir`:** `grep 'plugins_dir' config.yml`

---

## T-7: GATE — Контрольная точка

*_Requirements: ALL_*
*_Complexity: mechanical_*

GOAL: Финальная проверка всех артефактов и зависимостей.

1. **Запустить `go build -o main ./cmd/main.go`** — компиляция.
2. **Запустить `go test ./...`** — все тесты.
3. **Проверить структуру Dockerfile-ов:**
   - `grep -c ENTRYPOINT registry/*/Dockerfile registry/*/*/Dockerfile registry/*/*/*/Dockerfile` — должно быть 0 совпадений.
   - `grep -c '/plugin' registry/*/*/*/Dockerfile` — должно быть 4 (по одному на каждый Dockerfile).
4. **Проверить наличие скриптов:**
   - `test -x build-plugins.sh`
   - `test -x register-plugins.sh`
5. **Проверить отсутствие устаревших файлов:**
   - `test ! -f push.sh`
