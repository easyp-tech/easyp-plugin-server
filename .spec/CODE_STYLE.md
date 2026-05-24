<!-- generated: 2026-05-24, template: core.md -->
# Code Style

Project-specific code conventions for EasyP Service.

## 1. Layer Structure

```
Transport (gRPC / HTTP)
  ↕ proto types ↔ domain types
Application (core.Core)
  ↕ domain interfaces
Adapters (registry, audit, metrics)
  ↕ raw DB / Docker / external APIs
Infrastructure (database, telemetry, grpchelper)
```

**Rules per layer:**
- **Transport** — uses generated proto types; converts to/from domain types
- **Application** — uses only domain types from `core/domain.go`; never imports adapter packages
- **Adapters** — implement core interfaces; may use external libraries
- **Infrastructure** — provides cross-cutting utilities; imported by all layers

## 2. Struct Tags by Layer

| Layer | Tags | Example |
|-------|------|---------|
| Domain (`core/`) | None | `type PluginInfo struct { ... }` |
| Config (`cmd/`) | `env:""` + `yaml:""` | `Host string \`env:"HOST" yaml:"host"\`` |
| Proto (generated) | `protobuf:""` + `json:""` | Auto-generated, do not modify |

Tag case rule: `snake_case` for all tag types (json, yaml, xml, bson, avro, mapstructure) — enforced by `tagliatelle` linter.

## 3. Conversion Pattern

### Proto → Domain (API layer)

```go
// In internal/api/api.go:
filter := core.PluginFilter{
    Group:   strings.TrimSpace(request.GetGroup()),
    Name:    strings.TrimSpace(request.GetName()),
    Version: strings.TrimSpace(request.GetVersion()),
    Tags:    compactStrings(request.GetTags()),
}
```

### Domain → Proto (API layer)

```go
func pluginInfoToProto(info *core.PluginInfo) *generator.PluginInfo {
    return &generator.PluginInfo{
        Id:        info.ID.String(),
        Group:     info.Group,
        Name:      info.Name,
        Version:   info.Version,
        Tags:      info.Tags,
        CreatedAt: timestamppb.New(info.CreatedAt),
    }
}
```

## 4. Naming Conventions

### Files

| Layer | Pattern | Example |
|-------|---------|---------|
| Domain | `domain.go`, `core.go` | `internal/core/domain.go` |
| Adapters | `<adapter_name>.go` | `internal/adapters/registry/registry.go` |
| API | `api.go`, `mcp.go` | `internal/api/api.go` |
| Tracing | `tracing_<target>.go` | `internal/telemetry/tracing_core.go` |
| Tests | `<file>_test.go` (same package) | `internal/core/crud_test.go` |
| Infrastructure | descriptive name | `internal/database/sql.go` |

### Types

| Category | Visibility | Example |
|----------|-----------|---------|
| Domain entities | Exported | `PluginInfo`, `AuditEntry` |
| Domain interfaces | Exported | `Registry`, `Plugin`, `FeatureGate` |
| Config structs | Unexported | `config`, `server`, `ports` |
| Adapter implementations | Exported | `Store`, `Registry`, `Worker` |

### Enums

```go
type Feature int

const (
    FeatureCodeGeneration Feature = iota
    FeaturePluginListing
    // ...
)
```

Pattern: typed constant with `iota`, string names in a parallel array.

## 5. Interface Conventions

1. **Context first:** `func (r *Registry) Get(ctx context.Context, ...) (Plugin, error)`
2. **Error always last:** all methods that can fail return `error` as the last value
3. **No `I` prefix:** `Registry`, not `IRegistry`
4. **Single-method interfaces preferred** for composability
5. **Interface in consumer package:** `core.Registry` is defined in `internal/core`, implemented in `internal/adapters/registry`

## 6. Import Ordering

Enforced by `gci` formatter:

```go
import (
    // 1. Standard library
    "context"
    "fmt"
    "log/slog"

    // 2. Third-party
    "github.com/prometheus/client_golang/prometheus"
    "google.golang.org/grpc"

    // 3. Project packages
    "github.com/easyp-tech/service/internal/core"
    "github.com/easyp-tech/service/internal/database"
)
```

## 7. Test File Organization

| Pattern | Location | Example |
|---------|----------|---------|
| Unit tests | Same directory, same package | `core/crud_test.go` |
| Integration tests | Same directory | `license/claims_test.go` |
| Test helpers | Same file or shared test file | Inline mocks in test files |

No `_test` package suffix — tests are internal (can access unexported symbols).

## 8. Quick Reference

| Aspect | Application Layer | Adapters Layer | API Layer |
|--------|-------------------|----------------|-----------|
| Types | Domain structs | Implementation structs | Proto types |
| Errors | Sentinel `Err*` vars | Wrap with `fmt.Errorf` | `ErrorToStatus()` mapping |
| Logging | `monitor.FromContext(ctx)` | Same | Same |
| Tracing | Via decorator (`TracingCore`) | Via decorator (`TracingRegistry`) | Via gRPC interceptor |
| Tags | None | DB-specific (sqlx) | Proto-generated |

## 9. Error Propagation

```
Adapter error → fmt.Errorf("c.registry.Get: %w", err)
  → Core wraps with domain context
    → API maps via ErrorToStatus() → gRPC status code
```

Rules:
- **Wrap at every call site** with `fmt.Errorf("function: %w", err)`
- **Domain errors** are sentinel: `core.ErrNotFound`, `core.ErrInvalidPluginName`, etc.
- **Error classification** via `errorCode()` produces string codes for audit
- **Never create new domain errors** outside `core/domain.go`

## 10. Logging Conventions

- **Logger:** `*slog.Logger` (stdlib, structured, JSON output)
- **Initialization:** `slog.NewJSONHandler` with `AddSource: true`
- **Context propagation:** `monitor.WithContext(ctx, log)` / `monitor.FromContext(ctx)`
- **Trace correlation:** `telemetry.TraceHandler` injects `trace_id` and `span_id` into every log record

| Layer | Log Level | What to Log |
|-------|-----------|-------------|
| API | Info | Request received, response sent |
| Core | Warn | Audit send cancelled, retry attempts |
| Adapters | Error | DB connection failures, Docker errors |
| Infrastructure | Info/Error | Server start/stop, telemetry init |

**Never log:** raw credentials, license keys, full request payloads.

## 11. Concurrency Patterns

- **errgroup** for parallel server startup (gRPC, metrics, health, MCP)
- **WorkerPool** for bounded Docker execution (channel-based job queue)
- **Audit Worker** — single goroutine consuming from buffered channel
- **Signal handling** — `signal.NotifyContext` + `forceShutdown` goroutine (15s hard deadline)
- **Graceful shutdown sequence:**
  1. Context cancelled (signal received)
  2. gRPC `GracefulStop()` — stop accepting new requests
  3. HTTP servers `Shutdown()` — drain active connections
  4. Telemetry flush
  5. WorkerPool `Shutdown()` — drain job queue
  6. Audit worker `Shutdown()` — flush remaining events
  7. DB connection close
