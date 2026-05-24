<!-- generated: 2026-05-24, template: bootstrap.md -->
# Agent Rules

Mandatory rules for AI agents working on this project.

## Code Style

- **All linters enabled** — `golangci-lint` runs with `default: all`, only `exhaustruct` and `wsl`/`wsl_v5` disabled
- **Max line length:** 140 characters
- **Min variable name length:** 2 characters (exceptions: `ok`, `id`, `fn`, `ch`, `wg`, `rl`, `ip`, `st`, `eg`, `mu`, `db`)
- **Import order:** standard → third-party → project (`github.com/easyp-tech/service`)
- **Formatters:** `gci`, `gofmt`, `gofumpt`, `goimports`
- **Comments:** English only; every exported symbol must have a godoc comment starting with its name
- **No inline comments** on `if`/`for`/`return` lines unless genuinely non-obvious

## Naming Conventions

| Category | Convention | Example |
|----------|-----------|---------|
| Files | `snake_case.go` | `worker_pool.go` |
| Packages | short, lowercase, no underscores | `grpchelper`, `ratelimiter` |
| Exported types | PascalCase | `WorkerPool`, `PluginInfo` |
| Private types | camelCase | `grpcMetrics` |
| Interfaces | action noun, no `I` prefix | `Registry`, `Plugin`, `FeatureGate` |
| Errors | `Err` prefix, sentinel | `ErrNotFound`, `ErrServerOverloaded` |
| Constants | PascalCase (exported), camelCase (private) | `OperationGenerateCode`, `pluginNameParts` |
| Struct tags | `snake_case` for json/yaml/xml/bson/avro/mapstructure | `json:"plugin_name"` |

## Error Handling

- **Domain errors** are sentinel variables in `internal/core/domain.go`
- **Wrap errors** with `fmt.Errorf("function: %w", err)` at each call site
- **Error mapping:** `api.ErrorToStatus()` converts domain errors → gRPC status codes
- **Never use `errors.New()` for domain errors** — all domain errors must be package-level `var`
- **Error classification:** `errorCode()` in `core.go` maps errors to string codes for audit

## Testing

- **Framework:** standard `go test`, `testify/assert` for assertions
- **Test package:** same package (internal tests), not `_test` suffix packages
- **Mocks:** defined inline in test files, not generated
- **Table-driven tests:** use slice of structs with `t.Run(tt.name, ...)`
- **No external test frameworks** (gomock, counterfeiter, etc.)
- **Run:** `go test ./...`

## Dependencies

- **Go modules** — `go mod tidy` to manage
- **No `math/rand`** — use `math/rand/v2` or `crypto/rand` (enforced by depguard)
- **Protobuf:** generated via `easyp --cfg easyp.yaml generate`
- **Add new deps** with `go get`, then `go mod tidy`

## Formatting

- **Auto-format:** `gofumpt` (strict superset of `gofmt`)
- **Import sorting:** enforced by `gci` (standard → default → project prefix)
- **Lint before commit:** `golangci-lint run ./...`

## Architecture Rules

- **Domain types** live in `internal/core/domain.go` — single source of truth
- **Business logic** in `internal/core/core.go` — thin, delegates to Registry
- **Tracing** via decorator pattern (`telemetry/tracing_*.go`), never mixed into business logic
- **Adapters** implement core interfaces; placed in `internal/adapters/`
- **Database access** always through `database.SQL` wrapper, never raw `sqlx.DB`
- **Context propagation** — always pass `context.Context` as first argument
- **Logger** — use `monitor.FromContext(ctx)` to get context-aware slog logger
- **Plugin execution** — local binary execution from `plugins/` directory, not Docker containers at runtime
