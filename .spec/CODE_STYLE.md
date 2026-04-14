<!-- generated: 2026-04-14, template: core.md -->
# Code Style

## 1. Layer Structure

```
┌────────────────────────────────────────┐
│  Transport (api/, grpchelper/)          │  Public proto types, gRPC handlers
│  Converts proto ↔ domain types         │
├────────────────────────────────────────┤
│  Application (core/)                    │  Domain types only, no proto imports
│  Business logic + interfaces            │
├────────────────────────────────────────┤
│  Adapters (adapters/)                   │  Implement core interfaces
│  Internal structs for DB/Docker         │  Convert to/from domain types
├────────────────────────────────────────┤
│  Infrastructure (database/, telemetry/) │  Shared utilities
│  Never imported by core/                │
└────────────────────────────────────────┘
```

Rules:
- `core/` never imports `api/`, `adapters/`, or `grpchelper/`
- `api/` imports `core/` for interfaces and types
- `adapters/` imports `core/` for interfaces and types
- `telemetry/` imports `core/` interfaces to create decorator wrappers

## 2. Conversion Pattern

**API → Core (request):**
```go
// internal/api/api.go
filter := core.PluginFilter{
    Group:   strings.TrimSpace(request.GetGroup()),
    Name:    strings.TrimSpace(request.GetName()),
    Version: strings.TrimSpace(request.GetVersion()),
    Tags:    compactStrings(request.GetTags()),
}
```

**Core → API (response):**
```go
// internal/api/api.go
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

## 3. Naming Conventions

| Category | Pattern | Example |
|----------|---------|---------|
| Files | `snake_case.go` | `audit_interceptor.go` |
| Test files | `*_test.go` | `api_test.go` |
| Packages | lowercase, single word | `core`, `api`, `registry` |
| Exported types | PascalCase | `WorkerPool`, `PluginInfo` |
| Unexported types | camelCase | `grpcMetrics`, `poolPlugin` |
| Sentinel errors | `Err` prefix | `ErrNotFound`, `ErrInvalidPluginName` |
| Feature enums | `Feature` prefix | `FeatureCodeGeneration` |
| Constants | PascalCase exported | `OperationGenerateCode`, `AuditStatusSuccess` |
| Constructors | `New` prefix | `New()`, `NewServer()`, `NewWorkerPool()` |
| Interfaces | Noun/verb-based | `Registry`, `Plugin`, `Metrics` |

## 4. Interface Conventions

- `context.Context` is always the first argument
- Return `error` as the last return value
- Interfaces defined by the consumer (`core/domain.go`), not the implementor
- No pointer receivers on interfaces
- Document each method with godoc comment

```go
type Registry interface {
    Get(ctx context.Context, pluginGroup, pluginName, pluginVersion string) (Plugin, error)
    List(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
    Create(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error)
    Update(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error)
    Delete(ctx context.Context, group, name, version string) error
}
```

## 5. Import Ordering

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
)
```

## 6. Error Propagation

- **Adapter → Core**: Convert infrastructure errors to domain sentinels where appropriate, otherwise wrap with `fmt.Errorf("context: %w", err)`
- **Core → API**: Return domain errors as-is (sentinels)
- **API → Client**: `ErrorToStatus()` maps domain errors → gRPC status codes
- **Interceptors**: Log errors, never swallow them
- **Rule**: Use `errors.Is()` for comparison, never `==`

```go
// Adapter wraps DB error
if errors.Is(err, sql.ErrNoRows) {
    return nil, core.ErrNotFound
}
return nil, fmt.Errorf("get plugin %s/%s:%s: %w", group, name, version, err)

// API maps domain error
case errors.Is(err, core.ErrNotFound):
    code = codes.NotFound
```

## 7. Logging Conventions

- Logger: `log/slog` with JSON handler
- Context propagation: `monitor.WithContext(ctx, log)` / `monitor.FromContext(ctx)`
- Trace correlation: `TraceLoggingInterceptor` injects `trace_id` into slog attributes
- Log levels:
  - `Error`: operation failures, panics, shutdown errors
  - `Warn`: license issues, audit overflow, degraded functionality
  - `Info`: server start/stop, connection events
  - `Debug`: request details, config values
- Never log sensitive data (license keys, tokens, full request payloads)

## 8. Concurrency Patterns

- **Worker pool**: `core.WorkerPool` — N goroutines reading from a buffered channel
- **Audit worker**: Single goroutine consuming from buffered channel (cap 1000)
- **Context cancellation**: Propagated via `signal.NotifyContext` → `errgroup.WithContext`
- **Graceful shutdown**: gRPC `GracefulStop()` → telemetry flush → audit drain → pool drain
- **Rate limiter cleanup**: Background goroutine with configurable interval
- **License watcher**: 60s ticker checking token expiration
- **No mutexes in hot path**: WorkerPool uses channels, RateLimiter uses `sync.Map`

## 9. Test File Organization

| Pattern | Description |
|---------|-------------|
| `api_test.go` | API handler tests (mock CoreService) |
| `crud_test.go` | Core CRUD business logic tests |
| `pool_test.go` | WorkerPool concurrency tests |
| `client_test.go` | SDK client tests |
| `claims_test.go` | License claims parsing tests |
| `features_test.go` | Feature enum tests |
| `registry_migration_test.go` | Registry migration tests |
| `registry_preservation_test.go` | Registry data preservation tests |

## 10. Quick Reference

| Aspect | Core | Adapters | API |
|--------|------|----------|-----|
| Types | Domain structs | Internal DB structs | Proto types |
| Errors | Sentinel vars | Wrap with fmt.Errorf | ErrorToStatus mapping |
| Logging | Via context | Via context | Via interceptors |
| Tracing | Via decorator | Direct (OTLP) | Via interceptors |
| Testing | Mock interfaces | Integration tests | Mock CoreService |
