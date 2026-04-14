# Code Review: audit-logs-rework

## Verdict: PASS

Все 12 требований (REQ-1.1 — REQ-5.2) прослеживаются в коде и тестах. Архитектурные границы соблюдены: аудит в core, caller IP через transport middleware, Worker/Store без изменений. Единственный `major` finding (F-1: пустой `ErrorCode`) исправлен в рамках fix cycle 1 — добавлен `errorCode()` классификатор доменных ошибок. Тесты, сборка и линтер проходят.

## Change Set

| File | Status | Notes |
|------|--------|-------|
| `internal/core/core.go` | ✅ Planned | Основные изменения: auditCh, sendAudit, auditSuccess, auditError, errorCode, аудит во всех 5 методах |
| `internal/core/context.go` | ✅ Planned | NEW — WithCallerIP, CallerIPFromContext |
| `internal/core/crud_test.go` | ✅ Planned | 10 новых тестов, обновление 15 вызовов New() |
| `internal/api/audit_interceptor.go` | ✅ Planned | DELETED |
| `internal/api/api_test.go` | ⚠️ Not Changed | Не содержал audit-тестов — удалять нечего. Ожидаемо. |
| `cmd/main.go` | ✅ Planned | Wiring: auditCh/log → core.New(), удалён AuditInterceptor из chain |
| `internal/grpchelper/server.go` | ✅ Planned | Caller IP middleware (unary + stream) после realip |

## Requirements Traceability

| Requirement | Test(s) | Code | CP | Verdict |
|-------------|---------|------|----|---------|
| REQ-1.1 | `TestGenerate_AuditSuccess`, `TestListPlugins_AuditSuccess`, `TestCreatePlugin_AuditSuccess`, `TestUpdatePlugin_AuditSuccess`, `TestDeletePlugin_AuditSuccess` | `core.go` — auditSuccess вызов в каждом из 5 методов | CP-1 | ✅ |
| REQ-1.2 | `TestGenerate_AuditError` | `core.go` — auditError вызов на каждом error-пути | CP-2 | ✅ |
| REQ-1.3 | Все audit-тесты проверяют OperationType, PluginName, Status, ErrorCode, ErrorMessage, Metadata | `auditSuccess`, `auditError` заполняют все поля AuditEntry | CP-1, CP-2 | ✅ |
| REQ-1.4 | `TestCallerIPFromContext_Set`, `TestCallerIPFromContext_NotSet` | `context.go` — CallerIPFromContext; `server.go` — callerIPUnaryInterceptor | CP-3 | ✅ |
| REQ-2.1 | `TestSendAudit_BlockingWhenFull` | `core.go` — sendAudit: `select { case c.auditCh <- entry: case <-ctx.Done(): }` | CP-4 | ✅ |
| REQ-2.2 | `TestSendAudit_ContextCancelled` | `core.go` — sendAudit: ctx.Done() branch | CP-5 | ✅ |
| REQ-3.1 | `TestGenerate_AuditSuccess` (file_count), `TestListPlugins_AuditSuccess` (plugin_count) | `core.go` — Metadata map[string]any в auditSuccess | CP-6 | ✅ |
| REQ-3.2 | (implicit — map[string]any extensibility) | Metadata — open-ended map, Schema без изменений | CP-6 | ✅ |
| REQ-4.1 | (structural — no interceptor reference in main.go) | `cmd/main.go` — AuditInterceptor удалён из chain, файл удалён | CP-7 | ✅ |
| REQ-4.2 | (structural — MCP calls CoreService which now audits) | `api/mcp.go` вызывает CoreService → Generate/ListPlugins → аудит в core | CP-8 | ✅ |
| REQ-5.1 | (no changes needed — Worker/Store untouched) | `adapters/audit/worker.go`, `adapters/audit/audit.go` — без изменений | CP-9 | ✅ |
| REQ-5.2 | (no changes needed — Worker.Shutdown untouched) | `adapters/audit/worker.go` Shutdown — без изменений | CP-9 | ✅ |

## Design Conformance

### 3.1 Architectural Boundaries
✅ Аудит в core layer (`internal/core/`). Caller IP middleware в transport layer (`internal/grpchelper/`). Зависимости направлены корректно: `grpchelper` → `core` (только для context key), `core` не зависит от transport.

### 3.2 Data Models
✅ `Core` struct соответствует design §2.5 (поля `auditCh`, `logger`). `callerIPKey` — приватный тип в context.go. Примечание: design указывает `New()` без `logger`, но `sendAudit` требует логирование (design §2.3: "логирует warning, не паникует") — `logger` добавлен обоснованно.

### 3.3 API Contracts
✅ Внешний API (protobuf) не изменился. Внутренний — `New()` принял два новых параметра (`auditCh`, `logger`) — breaking change допустимо по решению пользователя.

### 3.4 Error Handling
✅ `sendAudit`: blocking с ctx.Done() fallback + log.Warn. `CallerIPFromContext`: "unknown" при отсутствии. `errorCode`: классификация через errors.Is.

### 3.5 Correctness Properties
- CP-1 (Аудит при успехе): ✅ Все 5 методов вызывают auditSuccess после успешного завершения.
- CP-2 (Аудит при ошибке): ✅ Все error paths вызывают auditError.
- CP-3 (Caller IP из context): ✅ callerIPUnaryInterceptor/callerIPStreamInterceptor → core.WithCallerIP, CallerIPFromContext в auditSuccess/auditError.
- CP-4 (Blocking send): ✅ select без default case.
- CP-5 (Context cancellation): ✅ ctx.Done() branch.
- CP-6 (Расширяемость Metadata): ✅ map[string]any, без schema changes.
- CP-7 (Отсутствие AuditInterceptor): ✅ Файл удалён, нет ссылок в main.go.
- CP-8 (Единообразие транспортов): ✅ MCP и gRPC идут через CoreService → Core → audit.
- CP-9 (Worker/Store без изменений): ✅ Файлы не затронуты.

### 3.6 Documentation Consistency
✅ Mermaid-диаграмма в design (§2.2) соответствует реализации: Transport → Core → chan → Worker → Store. AuditInterceptor помечен как deleted.

## Code Quality

### 4.1 Naming & Clarity
✅ `sendAudit`, `auditSuccess`, `auditError`, `errorCode`, `callerIPUnaryInterceptor`, `CallerIPFromContext`, `WithCallerIP` — названия ясные и следуют Go-конвенциям.

### 4.2 Dead Code & Debug Artifacts
✅ Нет закомментированного кода, TODO, debug-логов. `audit_interceptor.go` удалён.

### 4.3 Scope Creep
✅ Реализация строго в рамках задач T-1 — T-6. Нет лишних рефакторингов.

### 4.4 Test Quality
✅ 10 тестов: проверяют OperationType, Status, PluginName, ErrorCode, ErrorMessage, Metadata ключи, context cancellation, blocking behavior. Используют `t.Parallel()`, hand-written mocks, helpers `testAuditCh`, `drainAudit`, `nopLogger`. Тесты не просто "no error" — проверяют конкретные поля audit entry.

## Security

Новых публичных endpoint'ов не добавлено. Изменения внутренние (core + middleware):
- **Input validation**: caller IP извлекается из `peer.FromContext` (gRPC runtime), не из user input. ✅
- **Data exposure**: caller IP записывается в audit_log (уже существующее поведение). ErrorMessage может содержать внутренние детали → допустимо для audit log (не exposed to client). ✅
- **Error leakage**: audit entries не возвращаются клиенту, только в БД. ✅
- **Injection**: Metadata сериализуется через jsonb (параметризованный SQL в Store). ✅
- **Secrets**: нет hardcoded secrets. ✅

No security issues found in changed files.

## Verification Evidence

- **Tests:**
```
ok      github.com/easyp-tech/service/internal/adapters/registry     1.334s
ok      github.com/easyp-tech/service/internal/api      1.704s
ok      github.com/easyp-tech/service/internal/core     2.284s
ok      github.com/easyp-tech/service/internal/database/connectors   (cached)
ok      github.com/easyp-tech/service/internal/database/internal     (cached)
ok      github.com/easyp-tech/service/internal/database/migrations   (cached)
ok      github.com/easyp-tech/service/internal/license  3.083s
ok      github.com/easyp-tech/service/internal/telemetry        2.627s
ok      github.com/easyp-tech/service/sdk       (cached)
```

- **Build:**
```
$ go build ./cmd/main.go
(no output — clean build)
```

- **Lint:**
```
10 issues — all pre-existing:
* errcheck: 4 (mcp-smoke, metrics, sdk)
* ineffassign: 1 (registry.go)
* staticcheck: 4 (main.go, license_interceptor.go)
* unused: 1 (mcp_tools.go)
None in modified files.
```

## Findings

| ID | Severity | File | Description | Requirement |
|----|----------|------|-------------|-------------|
| F-1 | major | `internal/core/core.go` | `auditError` не заполняла `ErrorCode` — поле оставалось пустой строкой. Старый `AuditInterceptor` использовал gRPC status code. | REQ-1.3 |

**F-1 Resolution:** Добавлена функция `errorCode(err error) string` — классифицирует доменные ошибки (`ErrNotFound` → `"NOT_FOUND"`, `ErrFeatureDenied` → `"FEATURE_DENIED"`, default → `"INTERNAL"`). Тест `TestGenerate_AuditError` дополнен проверкой `ErrorCode != ""`. Все тесты проходят.

## Recommendations

Нет открытых рекомендаций — единственный finding (F-1) исправлен.
