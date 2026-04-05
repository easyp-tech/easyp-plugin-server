<!-- generated: 2026-04-03, template: bootstrap.md -->
# Agent Rules

Mandatory rules for AI agents working on this project.

## Code Style

- Follow standard Go conventions (Effective Go, Go Code Review Comments)
- Tracing is added via **decorator pattern** (`telemetry/tracing_*.go`) — never mix tracing into business logic
- Domain errors are **sentinel** (`var ErrX = errors.New(...)`) — never wrap with `fmt.Errorf` at definition site
- Error wrapping at call site: `fmt.Errorf("funcName: %w", err)` — always include the function name as context
- Keep `core` package free of infrastructure imports (no SQL, no Docker, no gRPC)
- Adapters implement core interfaces — placed in `internal/adapters/`

## Naming Conventions

- **Plugin names:** `{group}/{name}:{version}` — regex `^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*:(v\d+\.\d+\.\d+|latest)$`
- **Files:** one primary type per file, file named after the type (lowercase, underscores for multi-word)
- **Interfaces:** defined in `internal/core/domain.go` — single source of truth
- **Test files:** `*_test.go` in the same package (internal tests), mocks defined in test files
- **Migration files:** `{number}.{description}.sql` — numeric prefix determines order
- **Adapters:** directory name matches the interface they implement (e.g., `adapters/registry/` → `Registry`)

## Error Handling

- Domain errors live in `internal/core/domain.go`: `ErrNotFound`, `ErrInvalidPluginName`, `ErrGenerationFailed`, `ErrServerOverloaded`, `ErrShuttingDown`
- Error-to-gRPC mapping in `internal/grpchelper/grpc_codes.go` via `GRPCCodesConverterHandler`
- Audit interceptor uses **non-blocking channel send** — drops events if buffer full (capacity 1000)
- WorkerPool classifies Docker exit codes 125/126/127 as **transient** (retryable)

## Testing

- Standard `go test` — no external test framework
- Test mocks are **manually defined** in test files — no code generation (mockgen, etc.)
- Table-driven tests with subtests (`t.Run`)
- MCP integration tests use `httptest.Server` + real MCP client
- SDK tests use subtests for retry, health, interceptors

## Dependencies

- Database access **always** through `database.SQL` wrapper — never raw `sqlx.DB`
- gRPC server/client creation through `grpchelper` package — never raw `grpc.NewServer()`
- Telemetry initialized via `telemetry.Setup()` — never manual OTel provider setup
- License management through `license.Manager` — never parse PASETO tokens directly

## Formatting

- `gofmt` / `goimports` for all Go files
- Import order: stdlib → external → internal (separated by blank lines)
- No `init()` functions — explicit initialization in `cmd/main.go`
