<!-- generated: 2026-04-14, template: bootstrap.md -->
# Agent Rules — EasyP Service

Rules for AI agents working on this project.

## Code Style

- Go 1.26+, follow Effective Go and Go Code Review Comments
- Use `log/slog` for structured logging via `monitor.FromContext(ctx)`
- No external test frameworks — standard `testing` package only
- Imports: stdlib → third-party → project (sorted within groups)
- Line length soft limit: 120 chars
- No `init()` functions

## Naming Conventions

- Files: `snake_case.go`, test files: `*_test.go`
- Packages: lowercase, single word when possible
- Interfaces: verb-based (`Registry`, `Plugin`, `AuditLog`), not `IRegistry`
- Errors: `Err` prefix (`ErrNotFound`, `ErrInvalidPluginName`)
- Constants: `CamelCase` for exported, `camelCase` for unexported
- Feature flags: `Feature` prefix enum (`FeatureCodeGeneration`)

## Error Handling

- Domain errors are sentinel: `var ErrX = errors.New("description")`
- Wrap with context: `fmt.Errorf("operation: %w", err)`
- Map domain → gRPC in `api.ErrorToStatus()`
- Never swallow errors; log at the top, wrap at the bottom
- Use `errors.Is()` for comparison, not `==`

## Testing

- Standard `go test`, no external framework
- Mocks defined in test files, not generated
- Table-driven tests preferred
- Test functions: `TestFunctionName` or `TestType_Method`
- Use `t.Helper()` in test helpers
- Integration tests use build tags when needed

## Dependencies

- Dependency injection via constructor functions (`New()`)
- No global state; pass dependencies explicitly
- Interfaces defined by consumers, not implementors
- `go mod tidy` after adding/removing dependencies

## Architecture Rules

- Domain types and interfaces only in `internal/core/domain.go`
- Business logic only in `internal/core/core.go`
- Adapters implement core interfaces; placed in `internal/adapters/`
- Tracing via decorator pattern (`telemetry/tracing_*.go`), never in business logic
- Database access only through `database.SQL` wrapper
- gRPC interceptor order matters — see `grpchelper.NewServer()`

## Formatting

- `gofmt` / `goimports` applied automatically
- Protobuf: run `easyp lint` before committing `.proto` changes
- YAML config: 2-space indent
- SQL migrations: `-- up` / `-- down` markers, sequential numbering
