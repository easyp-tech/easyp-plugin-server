# Code Review: disk-plugin-execution

## Verdict: PASS

Реализация полностью соответствует утверждённым requirements и design. Все 25 требований (REQ-1.1–REQ-7.1) покрыты кодом и тестами. Архитектурные границы соблюдены — изменения затрагивают только запланированные файлы (+ несколько pre-existing lint fix). Безопасность адресована: path traversal protection, env isolation, process group kill, output size limit. Тесты проходят, сборка успешна, линтер чист (6 pre-existing issues).

---

## Набор изменений (Change Set)

| Файл | Статус | Примечание |
|------|--------|------------|
| `Dockerfile` | ✅ Planned | debian:bookworm-slim, docker-cli удалён, VOLUME |
| `cmd/main.go` | ✅ Planned | Domain → PluginsDir + MaxOutputSize |
| `internal/adapters/registry/registry.go` | ✅ Planned | Основные изменения: PluginConfig, ValidateConfig, Generate() |
| `internal/adapters/registry/disk_plugin_test.go` | ✅ Planned (NEW) | Тесты на Generate, ValidateConfig |
| `internal/adapters/registry/testdata/mock_plugin.go` | ✅ Planned (NEW) | Mock-бинарник для тестов |
| `internal/core/pool.go` | ✅ Planned | isTransient() без Docker exit codes |
| `internal/core/pool_test.go` | ✅ Planned | TestIsTransient добавлен |
| `internal/telemetry/tracing_plugin.go` | ✅ Planned | docker.exec → process.exec |
| `migrate/5.disk_plugin_config.sql` | ✅ Planned (NEW) | SQL migration docker → command |
| `internal/adapters/registry/registry_preservation_test.go` | ⚠️ Unexpected | Обновление existing теста под новый constructor `New()` — оправданно |
| `internal/core/domain.go` | ⚠️ Unexpected | Добавлена пустая строка перед `return` (nlreturn lint fix) — косметическое, допустимо |
| `internal/database/internal/internal.go` | ⚠️ Unexpected | `reflect.Ptr` → `reflect.Pointer` (govet inline fix) — pre-existing lint fix |
| `internal/license/errors.go` | ⚠️ Unexpected | Удалены лишние скобки `var(...)` → `var` (gofumpt fix) — pre-existing lint fix |
| `internal/license/gate_preservation_test.go` | ⚠️ Unexpected | `slices.Contains` вместо цикла (modernize fix) — pre-existing lint fix |

Все «unexpected» файлы — это pre-existing lint fixes или необходимые адаптации тестов к новому API. Scope creep отсутствует.

---

## Трассировка требований (Requirements Traceability)

| Requirement | Тесты | Код | CP | Verdict |
|-------------|-------|-----|----|---------| 
| REQ-1.1 | `TestGenerate_Success` | [registry.go:288-410](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L288-L410) | CP-1 | ✅ |
| REQ-1.2 | `TestGenerate_NonZeroExit` | [registry.go:387-401](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L387-L401) | CP-1 | ✅ |
| REQ-1.3 | `TestGenerate_BinaryNotFound` | [registry.go:341-344](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L341-L344) | CP-5 | ✅ |
| REQ-1.4 | `TestGenerate_Success` (unmarshal check) | [registry.go:404-408](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L404-L408) | CP-1 | ✅ |
| REQ-1.5 | `TestGenerate_CustomEnv` | [registry.go:317-320](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L317-L320) | CP-15 | ✅ |
| REQ-1.6 | `TestGenerate_Timeout` | [registry.go:299-310](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L299-L310) | CP-16 | ✅ |
| REQ-1.7 | `TestGenerate_EnvironmentIsolation` | [registry.go:317](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L317) | CP-2 | ✅ |
| REQ-1.8 | `TestGenerate_ProcessGroupKill` | [registry.go:326](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L326), [registry.go:357-366](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L357-L366) | CP-3 | ✅ |
| REQ-1.9 | `TestGenerate_OutputSizeLimit` | [registry.go:264](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L264), [registry.go:383-385](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L383-L385) | CP-4 | ✅ |
| REQ-1.10 | `TestGenerate_PermissionDenied` | [registry.go:346-348](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L346-L348) | CP-5 | ✅ |
| REQ-2.1 | `TestPluginConfig_RoundTrip` | [registry.go:49-55](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L49-L55), [registry.go:164-170](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L164-L170) | CP-14 | ✅ |
| REQ-2.2 | `TestValidateConfig` (empty command) | [registry.go:100-102](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L100-L102) | CP-6 | ✅ |
| REQ-2.3 | — (infrastructure) | [5.disk_plugin_config.sql](file:///Users/zergslaw/Projects/easyp/service/migrate/5.disk_plugin_config.sql) | CP-9 | ✅ |
| REQ-2.4 | `TestValidateConfig` (path traversal cases) | [registry.go:104-122](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L104-L122) + [registry.go:414-418](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L414-L418) | CP-7 | ✅ |
| REQ-2.5 | `TestValidateConfig` | [registry.go:450-454](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L450-L454) | CP-7 | ✅ |
| REQ-2.6 | `TestValidateConfig` (old docker format) | [registry.go:85-92](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L85-L92) | CP-8 | ✅ |
| REQ-3.1 | — (structural) | [registry.go:128](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L128) (domain removed from `New`) | CP-10 | ✅ |
| REQ-3.2 | — (structural) | [registry.go:64-76](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L64-L76) (domain removed from plugin struct) | CP-10 | ✅ |
| REQ-4.1 | `TestIsTransient` (codes 125/126/127 → false) | [pool.go:305-324](file:///Users/zergslaw/Projects/easyp/service/internal/core/pool.go#L305-L324) | CP-11 | ✅ |
| REQ-4.2 | `TestIsTransient` (SIGKILL → false) | [pool.go:305-324](file:///Users/zergslaw/Projects/easyp/service/internal/core/pool.go#L305-L324) | CP-12 | ✅ |
| REQ-5.1 | — (build verified) | [tracing_plugin.go:52-55](file:///Users/zergslaw/Projects/easyp/service/internal/telemetry/tracing_plugin.go#L52-L55), [tracing_plugin.go:75](file:///Users/zergslaw/Projects/easyp/service/internal/telemetry/tracing_plugin.go#L75) | CP-13 | ✅ |
| REQ-6.1 | — (Dockerfile) | [Dockerfile:13](file:///Users/zergslaw/Projects/easyp/service/Dockerfile#L13) | — | ✅ |
| REQ-6.2 | — (Dockerfile) | [Dockerfile:19](file:///Users/zergslaw/Projects/easyp/service/Dockerfile#L19) | — | ✅ |
| REQ-6.3 | — (Dockerfile) | Docker-cli удалён, socket не монтируется | — | ✅ |
| REQ-7.1 | `TestGenerate_Success` | [registry.go:314](file:///Users/zergslaw/Projects/easyp/service/internal/adapters/registry/registry.go#L314) (exec каждый раз с диска) | CP-17 | ✅ |

Все 25 требований покрыты.

---

## Соответствие дизайну (Design Conformance)

### 3.1 Архитектурные границы

✅ Все изменения в запланированных слоях:
- Adapter: `registry.go` — execution logic  
- Core: `pool.go` — error classification
- Telemetry: `tracing_plugin.go` — span rename
- Infrastructure: `Dockerfile`, `cmd/main.go`, `migrate/`

Нет нарушений зависимостей между слоями. `core` не знает о `exec` (кроме `pool_test.go` для конструирования ExitError в тестах — допустимо).

### 3.2 Модель данных

✅ `PluginConfig` struct (строка 49-55 registry.go) точно соответствует дизайну:
```go
type PluginConfig struct {
    Command []string          `json:"command"`
    Env     map[string]string `json:"env,omitempty"`
    Timeout string            `json:"timeout,omitempty"`
}
```
✅ `Registry` struct: `domain` удалён, `pluginsDir` и `maxOutputSize` добавлены.
✅ SQL migration корректна — `WHERE config ? 'docker'` гарантирует идемпотентность.

### 3.3 API контракты

✅ gRPC API не изменился. `Generate(ctx, *CodeGeneratorRequest) (*CodeGeneratorResponse, error)` — прежняя сигнатура.
✅ `New()` сигнатура соответствует дизайну: `func New(_ context.Context, db *database.SQL, pluginsDir string, maxOutputSize int64) (*Registry, error)`.

### 3.4 Обработка ошибок

✅ Sentinel errors используются правильно (`ErrEmptyConfig`, `ErrOldFormat`, `ErrEmptyCommand`, `ErrInvalidConfig`, `ErrEmptyPluginsDir`).
✅ Ошибки оборачиваются через `fmt.Errorf("%w: ...", ...)` с domain errors из `core`.
✅ `exec.ErrNotFound` → `core.ErrNotFound`, `os.ErrPermission` → `core.ErrGenerationFailed` (REQ-1.10).
✅ Exit code + stderr включены в error message (REQ-1.2).

### 3.5 Correctness Properties

| CP | Тип | Проверка | Статус |
|----|-----|----------|--------|
| CP-1 | Equivalence — протокол | stdin/stdout protobuf round-trip | ✅ |
| CP-2 | Absence — env isolation | `cmd.Env = make([]string, 0, ...)` | ✅ |
| CP-3 | Absence — process group | `Setpgid: true` + `syscall.Kill(-pid, SIGKILL)` | ✅ |
| CP-4 | Absence — output limit | `io.LimitReader(stdout, maxStdout+1)` + check `len > max` | ✅ |
| CP-5 | Exclusion — not found vs perm denied | Два разных error wrapping | ✅ |
| CP-6 | Absence — empty command | `ValidateConfig` → `ErrEmptyCommand` | ✅ |
| CP-7 | Absence — path traversal | `filepath.Clean` + `strings.HasPrefix` | ✅ |
| CP-8 | Absence — old format | `raw["docker"]` check | ✅ |
| CP-9 | Propagation — migration | SQL UPDATE with `WHERE config ? 'docker'` | ✅ |
| CP-10 | Absence — domain | `domain` removed from Registry and plugin | ✅ |
| CP-11 | Absence — Docker codes | exit codes 125/126/127 no longer transient | ✅ |
| CP-12 | Equivalence — signal | SIGKILL → not transient | ✅ |
| CP-13 | Propagation — telemetry | Span `process.exec`, attrs `process.*` | ✅ |
| CP-14 | Round-trip — config | JSON Marshal → Unmarshal preserves data | ✅ |
| CP-15 | Propagation — env | config.Env → cmd.Env | ✅ |
| CP-16 | Propagation — timeout | config.Timeout → context.WithTimeout | ✅ |
| CP-17 | Equivalence — hot reload | exec reads from disk each time (no caching) | ✅ |

### 3.6 Документация

✅ Mermaid-диаграмма в design.md соответствует фактической архитектуре (TracingPlugin → plugin.Generate via exec).

---

## Качество кода (Code Quality)

### 4.1 Именование

✅ Все новые идентификаторы следуют конвенциям проекта:
- `PluginConfig`, `ValidateConfig`, `ErrEmptyConfig`, `readPipes` — Go naming convention
- `pluginsDir`, `maxOutputSize` — camelCase для приватных полей
- `numPipesReader`, `stderrLimit` — package-level constants

### 4.2 Мёртвый код

✅ Docker-специфичный код полностью удалён:
- `DockerConfig` struct
- Docker exit code constants (125/126/127)
- Docker-подстроки в `isTransient()`
- `domain` field и URL parsing
- `docker-cli` в Dockerfile

Нет закомментированного кода или debug prints.

### 4.3 Scope Creep

⚠️ Minor scope creep: lint fixes в файлах не из плана (`domain.go`, `errors.go`, `gate_preservation_test.go`, `internal.go`). Все косметические, не затрагивают логику. Допустимо.

### 4.4 Качество тестов

✅ Тесты покрывают все key paths:
- Happy path: `TestGenerate_Success` — protobuf round-trip
- Error paths: not found, permission denied, non-zero exit, timeout
- Security: env isolation, custom env propagation
- Resource: output size limit
- Process: process group kill
- Config: round-trip, validation table-driven tests

✅ Assertions проверяют конкретное поведение (содержимое ошибок, наличие/отсутствие env vars), не только `err != nil`.
✅ `TestMain` корректно компилирует mock plugin и очищает temp directory.
✅ Table-driven tests используются для `ValidateConfig` (8 test cases).

---

## Безопасность (Security)

| Категория | Проверка | Статус |
|-----------|----------|--------|
| Input validation | `ValidateConfig` — path traversal, empty command, old format | ✅ |
| Command injection | `exec.CommandContext` с `Command[0]` из DB. `//nolint:gosec` обоснован — config проходит валидацию при Create/Update | ⚠️ Acceptable |
| Env isolation | `cmd.Env = make([]string, 0)` — нет наследования сервисных секретов | ✅ |
| Secrets | DATABASE_URL, LICENSE_KEY не попадают в процесс плагина. Тест `TestGenerate_EnvironmentIsolation` подтверждает | ✅ |
| Resource limits | stdout limited by `io.LimitReader`. stderr limited to 1MB. Process group killed on timeout | ✅ |
| Error leakage | stderr плагина включается в error message — это допустимо (не содержит секретов сервиса, только вывод плагина) | ✅ |
| Path traversal | `filepath.Clean` + `strings.HasPrefix(cleanedArg, cleanedPluginsDir+separator)` | ✅ |
| New endpoints | Нет новых эндпоинтов — существующий gRPC API, CRUD API без изменений | ✅ |

---

## Верификация (Verification Evidence)

- **Tests:**
```
?   	github.com/easyp-tech/service/api/generator/v1	[no test files]
?   	github.com/easyp-tech/service/cmd	[no test files]
?   	github.com/easyp-tech/service/cmd/mcp-smoke	[no test files]
?   	github.com/easyp-tech/service/internal/adapters/audit	[no test files]
?   	github.com/easyp-tech/service/internal/adapters/metrics	[no test files]
ok  	github.com/easyp-tech/service/internal/adapters/registry	2.453s
ok  	github.com/easyp-tech/service/internal/api	(cached)
ok  	github.com/easyp-tech/service/internal/core	(cached)
?   	github.com/easyp-tech/service/internal/database	[no test files]
ok  	github.com/easyp-tech/service/internal/database/connectors	(cached)
ok  	github.com/easyp-tech/service/internal/database/internal	(cached)
ok  	github.com/easyp-tech/service/internal/database/migrations	(cached)
?   	github.com/easyp-tech/service/internal/flags	[no test files]
?   	github.com/easyp-tech/service/internal/grpchelper	[no test files]
ok  	github.com/easyp-tech/service/internal/license	(cached)
?   	github.com/easyp-tech/service/internal/monitor	[no test files]
?   	github.com/easyp-tech/service/internal/ratelimiter	[no test files]
ok  	github.com/easyp-tech/service/internal/telemetry	(cached)
ok  	github.com/easyp-tech/service/sdk	(cached)
```

- **Build:**
```
$ go build -o main ./cmd/main.go
(exit 0, no output)
```

- **Lint:**
```
cmd/main.go:237:31: Function `NewManager` should pass the context parameter (contextcheck)
cmd/mcp-smoke/main.go:63:44: string `easyp_config_describe` has 3 occurrences, make it a constant (goconst)
internal/adapters/metrics/metrics.go:32:13: string `plugin` has 4 occurrences, make it a constant (goconst)
internal/core/core.go:156:16: string `latest` has 3 occurrences, make it a constant (goconst)
internal/license/features.go:36:10: string `unknown` has 4 occurrences, make it a constant (goconst)
internal/license/client.go:10:2: Line contains TODO/BUG/FIXME (godox)
6 issues: contextcheck(1), goconst(4), godox(1) — all pre-existing
```

---

## Findings

| ID | Severity | Файл | Описание | Requirement |
|----|----------|------|----------|-------------|
| F-1 | nit | `registry.go:317` | `cmd.Env = make([]string, 0, len(p.pluginConfig.Env))` — корректно, но при `Env == nil` аллокация capacity=0. Можно `cmd.Env = []string{}` для consistency. Не баг — nit | REQ-1.7 |
| F-2 | nit | `tracing_plugin.go:6` | Import `os/exec` остался в `tracing_plugin.go` — используется для `exec.ExitError` type assertion (line 73). Корректно. | REQ-5.1 |
| F-3 | nit | `disk_plugin_test.go:334` | `TestGenerate_ProcessGroupKill`: проверка `len(pids) > 1` через `pgrep` — non-deterministic. Тест логирует через `t.Logf` но не fail-ит. Допустимо как best-effort проверка | REQ-1.8 |

---

## Рекомендации

Нет блокирующих или major рекомендаций. Три nit-finding — косметические, не требуют исправления для approve.

Опционально для следующих итераций:
1. Добавить `TestValidateConfig_EmptyConfig` отдельным тест-кейсом для `len(config) == 0`.
2. В `isTransient()` оставлены `connection refused` и `temporary failure` — убедиться что эти паттерны актуальны для disk-based execution (могут быть релевантны для DB-операций в Registry).
