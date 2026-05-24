# local-test-env — Implementation Summary

**Date:** 2026-05-24

## Выполненные задачи

- [x] **T-1** GREEN — Baseline (go build + go test проходят)
- [x] **T-2** CODE — Переделаны 4 Dockerfile-а (убраны ENTRYPOINT/USER/passwd, финальный стейдж `/plugin`)
- [x] **T-3** CODE — Создан `build-plugins.sh`
- [x] **T-4** CODE — Создан `register-plugins.sh`
- [x] **T-5** CODE — Обновлены config.yml, config.local.yml, docker-compose.yml, Taskfile.yml, удалён push.sh
- [x] **T-6** VERIFY — Все проверки пройдены
- [x] **T-7** GATE — Финальная контрольная точка пройдена

## Изменённые файлы

| File | Change |
|------|--------|
| `registry/protocolbuffers/go/v1.36.10/Dockerfile` | Упрощён: `FROM scratch` + `COPY /plugin` |
| `registry/grpc/go/v1.5.1/Dockerfile` | Упрощён: `FROM scratch` + `COPY /plugin` |
| `registry/grpc-ecosystem/gateway/v2.27.3/Dockerfile` | Упрощён: `FROM scratch` + `COPY /plugin` |
| `registry/grpc-ecosystem/openapiv2/v2.27.3/Dockerfile` | Упрощён: `FROM scratch` + `COPY /plugin` |
| `build-plugins.sh` | **NEW** — Скрипт сборки бинарников через `docker build --output` |
| `register-plugins.sh` | **NEW** — Скрипт регистрации через gRPC CreatePlugin |
| `config.yml` | `registry.domain` → `registry.plugins_dir` + `max_output_size` |
| `config.local.yml` | `registry.domain` → `registry.plugins_dir` + `max_output_size` |
| `docker-compose.yml` | Убран `registry`, `registry-data`, `docker.sock` → `./plugins:/plugins:ro` |
| `Taskfile.yml` | Убраны `local-push-*`, добавлены `build-plugins` + `register-plugins` |
| `push.sh` | **DELETED** |

## Результат верификации

- ✓ `go build` — компилируется
- ✓ `go test ./...` — все тесты проходят
- ✓ Нет ENTRYPOINT в Dockerfile-ах
- ✓ `/plugin` присутствует в 4 Dockerfile-ах
- ✓ `build-plugins.sh` и `register-plugins.sh` исполняемые
- ✓ `push.sh` удалён
