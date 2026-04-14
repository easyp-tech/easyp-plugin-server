# Перенос аудита в бизнес-логику — Task Plan

**Status:** Draft
**Date:** 2026-04-14

---

**Тип работы:** Migration — реструктуризация существующего поведения (аудит переносится из interceptor'а в core) без изменения наблюдаемых результатов для потребителей `audit_log`.

---

**Test Style Source:** Tier 2
- Evidence: `internal/core/crud_test.go` — hand-written mocks с function fields, table-driven tests, `t.Parallel()`, `errors.Is()`, `t.Fatalf`/`t.Errorf`
- Key patterns: internal package tests (`package core`), mock structs (`mockRegistry`, `mockFeatureGate`), `testing.T` стандартная библиотека

**Commands:**

| Действие | Команда | Источник |
|----------|---------|----------|
| Тесты | `go test ./...` | Design §2.8 |
| Сборка | `go build ./cmd/main.go` | Design §2.8 |
| Линтер | `golangci-lint run` | Design §2.8 |
| Кодогенерация | `easyp --cfg easyp.yaml generate` | Design §2.8 |

---

## Матрица покрытия

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1 | T-2, T-3 | CP-1 (Propagation — аудит при успехе) |
| REQ-1.2 | T-2, T-3 | CP-2 (Propagation — аудит при ошибке) |
| REQ-1.3 | T-2, T-3 | CP-1, CP-2 |
| REQ-1.4 | T-2, T-3 | CP-3 (Propagation — caller IP) |
| REQ-2.1 | T-2, T-3 | CP-4 (Absence — гарантия доставки) |
| REQ-2.2 | T-2, T-3 | CP-5 (Absence — no block on cancel) |
| REQ-3.1 | T-2, T-3 | CP-6 (Equivalence — metadata) |
| REQ-3.2 | T-2, T-3 | CP-6 |
| REQ-4.1 | T-4 | CP-7 (Absence — нет interceptor'а) |
| REQ-4.2 | T-3 | CP-8 (Equivalence — транспорты) |
| REQ-5.1 | T-1 | CP-9 (Equivalence — Worker/Store) |
| REQ-5.2 | T-1 | CP-9 |

---

## T-1: GREEN — Preservation tests: зафиксировать текущее поведение

*_Requirements: REQ-5.1, REQ-5.2_*

IMPORTANT: Эти тесты должны проходить ДО любых изменений в production-коде.
NOTE: Фиксируем поведение существующих core-методов и Worker/Store, чтобы миграция не сломала их.
DO NOT: Модифицировать production-код в этой задаче.
*_Test_Style:_* `internal/core/crud_test.go`

Подзадачи:
- [ ] 1. В `internal/core/crud_test.go` — убедиться что существующие тесты (`TestGenerate_PreservationAfterCRUD`, `TestListPlugins_PreservationAfterCRUD`, `TestCreatePlugin_*`, `TestUpdatePlugin_*`, `TestDeletePlugin_*`) проходят: `go test ./internal/core/...`
- [ ] 2. В `internal/adapters/audit/` — убедиться что тесты Worker/Store (если есть) проходят: `go test ./internal/adapters/audit/...`
- [ ] 3. Запустить полный набор тестов: `go test ./...` — всё зелёное.

---

## T-2: CODE — Реализовать аудит в core layer

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.3, REQ-1.4, REQ-2.1, REQ-2.2, REQ-3.1, REQ-3.2_*
*_Preservation: CP-9 (Worker/Store без изменений)_*

CRITICAL: Каждая подзадача — один файл. После каждой подзадачи запускать `go test ./internal/core/...`.
DO NOT: Рефакторить не связанный код. DO NOT: Вводить абстракции, не описанные в design document.

Подзадачи:
- [ ] 1. Создать `internal/core/context.go` — тип `callerIPKey struct{}`, функции `WithCallerIP(ctx, ip) context.Context` и `CallerIPFromContext(ctx) string` (возвращает `"unknown"` по умолчанию). — `go test ./internal/core/...`
- [ ] 2. Модифицировать `internal/core/core.go` — добавить поле `auditCh chan<- AuditEntry` в struct `Core`, параметр `auditCh` в `New()`, приватный метод `sendAudit(ctx, entry)` с blocking send (`select { case c.auditCh <- entry: case <-ctx.Done(): }`). — `go test ./internal/core/...`
- [ ] 3. Модифицировать `internal/core/core.go` — встроить вызовы `sendAudit` в методы `Generate`, `ListPlugins`, `CreatePlugin`, `UpdatePlugin`, `DeletePlugin`. Каждый метод: измеряет duration, формирует `AuditEntry` с `CallerIPFromContext`, `OperationType`, `Metadata`, отправляет через `sendAudit`. — `go test ./internal/core/...`
- [ ] 4. Модифицировать `internal/core/crud_test.go` — обновить все вызовы `New()`, добавив audit-канал. Добавить тесты: `TestGenerate_AuditSuccess`, `TestGenerate_AuditError`, `TestListPlugins_AuditSuccess`, `TestCreatePlugin_AuditSuccess`, `TestUpdatePlugin_AuditSuccess`, `TestDeletePlugin_AuditSuccess`. — `go test ./internal/core/...`
- [ ] 5. Создать тесты в `internal/core/crud_test.go` — `TestSendAudit_ContextCancelled`, `TestSendAudit_BlockingWhenFull`, `TestCallerIPFromContext_Set`, `TestCallerIPFromContext_NotSet`. — `go test ./internal/core/...`

После всех подзадач: `go build ./cmd/main.go` и `golangci-lint run`.

---

## T-3: CODE — Обновить wiring и добавить caller IP middleware

*_Requirements: REQ-1.4, REQ-4.2_*
*_Preservation: CP-1, CP-2, CP-3, CP-8 (единообразие транспортов)_*

CRITICAL: Каждая подзадача — один файл.
IMPORTANT: После каждой подзадачи: `go test ./...`

Подзадачи:
- [ ] 1. Модифицировать `cmd/main.go` — передать `auditCh` в `core.New()`. Удалить создание `AuditInterceptor` и его из `extraUnary` slice. — `go test ./...`
- [ ] 2. Модифицировать `internal/grpchelper/server.go` — добавить caller IP middleware в unary/stream цепочку: извлечение из `peer.FromContext` → `core.WithCallerIP(ctx, addr)`. — `go test ./...`

После всех подзадач: `go build ./cmd/main.go` и `golangci-lint run`.

---

## T-4: CODE — Удалить AuditInterceptor

*_Requirements: REQ-4.1_*
*_Preservation: CP-1, CP-2, CP-7 (отсутствие interceptor'а)_*

CRITICAL: Один файл — одна подзадача.

Подзадачи:
- [ ] 1. Удалить файл `internal/api/audit_interceptor.go`. — `go build ./cmd/main.go`
- [ ] 2. Модифицировать `internal/api/api_test.go` — удалить тесты, связанные с `AuditInterceptor` (если есть). — `go test ./internal/api/...`

После всех подзадач: `go build ./cmd/main.go` и `golangci-lint run`.

---

## T-5: VERIFY — Повторный прогон preservation tests

*_Requirements: REQ-5.1, REQ-5.2_*

CRITICAL: Preservation-тесты из T-1 должны по-прежнему проходить.
GOAL: Подтвердить что миграция не сломала существующее поведение.

Инструкции:
1. `go test ./internal/core/...` — все тесты зелёные.
2. `go test ./internal/adapters/audit/...` — Worker/Store тесты зелёные.
3. `go test ./...` — полный набор зелёный.

---

## T-6: GATE — Финальная проверка

*_Requirements: ALL_*

CRITICAL: Эта задача должна быть последней. Не выполнять пока все предыдущие задачи не завершены.

Инструкции:
1. `go test ./...` — 100% тестов проходят.
2. `go build ./cmd/main.go` — без ошибок.
3. `golangci-lint run` — без нарушений.
4. Проверить матрицу покрытия — каждый requirement имеет хотя бы один проходящий тест.
5. Убедиться что `internal/api/audit_interceptor.go` удалён.
6. Убедиться что `AuditInterceptor` не упоминается в `cmd/main.go`.
7. Если любая проверка не проходит — вернуться к соответствующей задаче.
