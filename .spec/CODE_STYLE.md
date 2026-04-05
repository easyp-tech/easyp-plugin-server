<!-- generated: 2026-04-03, template: core.md -->
# Code Style

## Layer Structure

```
Transport (internal/api)        → proto types, gRPC handlers
Application (internal/core)     → domain types, business logic
Adapters (internal/adapters)    → interface implementations (DB, Docker, Prometheus)
Infrastructure (internal/database, grpchelper, telemetry, license, ratelimiter) → shared tooling
```

Rules:
- `core` has **zero** infrastructure imports (no SQL, Docker, gRPC, Prometheus)
- Adapters import `core` for interfaces — never the reverse
- API layer imports `core` for domain types and `api/generator/v1` for proto types
- Infrastructure packages are self-contained, imported by `cmd/main.go` for wiring

## Conversion Patterns

**Proto ↔ Domain (in `internal/api/api.go`):**
```go
// proto → domain (in handler)
filter := core.PluginFilter{
    Group:   strings.TrimSpace(request.GetGroup()),
    Name:    strings.TrimSpace(request.GetName()),
}

// domain → proto (in handler)
response.Plugins = append(response.Plugins, &generator.PluginInfo{
    Id:      p.ID.String(),
    Group:   p.Group,
    Name:    p.Name,
    Version: p.Version,
})
```

Conversion happens **only** in the API layer. Core never sees proto types (except `pluginpb.CodeGeneratorRequest/Response` which are the protobuf plugin contract).

## Naming Conventions

| Category | Convention | Example |
|----------|-----------|---------|
| Files | lowercase, one primary type per file | `pool.go`, `claims.go` |
| Interfaces | defined in `core/domain.go` | `Registry`, `Plugin`, `Metrics` |
| Implementations | unexported struct in adapter package | `registry` struct in `adapters/registry` |
| Constructors | `New()` or `NewXxx()` | `core.New()`, `license.NewFeatureGate()` |
| Test files | `*_test.go` in same package | `pool_test.go`, `claims_test.go` |
| Migrations | `{number}.{description}.sql` | `1.init.sql`, `3.audit_log.sql` |
| Adapters | directory name = interface implemented | `adapters/registry` → `Registry` |

## Interface Conventions

- First parameter is always `context.Context`
- Return `error` as last return value
- Interfaces defined in `core/domain.go` — single source of truth
- Use `int` for cross-package enum parameters to avoid cyclic imports (e.g., `FeatureGate.Enabled(feature int)`)

## Import Ordering

Three groups separated by blank lines:
```go
import (
    // stdlib
    "context"
    "fmt"

    // external
    "google.golang.org/grpc"
    "github.com/prometheus/client_golang/prometheus"

    // internal
    "github.com/easyp-tech/service/internal/core"
)
```

## Error Propagation

- **Core → API:** errors bubble up unwrapped. `ErrorToStatus()` uses `errors.Is()` to map to gRPC codes.
- **Adapter → Core:** adapt infrastructure errors to domain errors (`sql.ErrNoRows` → `core.ErrNotFound`).
- **Wrapping convention:** `fmt.Errorf("funcName: %w", err)` — always include the calling function name.
- **Never** wrap sentinel errors at definition — they stay as `errors.New(...)`.

## Test File Organization

| Pattern | Description |
|---------|-------------|
| Mock types at top | `type mockRegistry struct { getFn func(...) }` |
| `noopLogger()` helper | Shared silent logger for tests |
| Table-driven tests | `tests := []struct{ name string; ... }{ ... }` with `t.Run(tt.name, ...)` |
| Subtests for scenarios | Group related cases under one `Test*` function |
| No external mock library | All mocks are manually written in test files |
