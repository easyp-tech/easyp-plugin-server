<!-- generated: 2026-04-03, template: errors.md -->
# Errors

## Error Architecture

```
┌─────────────────────────────────────────────┐
│  gRPC Client                                 │
│  Receives: gRPC status code + message        │
├─────────────────────────────────────────────┤
│  API Layer (internal/api)                    │
│  ErrorToStatus(): domain errors → gRPC codes │
│  Wraps with fmt.Errorf("funcName: %w", err) │
├─────────────────────────────────────────────┤
│  Application Layer (internal/core)           │
│  Returns sentinel errors (ErrNotFound, etc.) │
│  Wraps adapter errors with context           │
├─────────────────────────────────────────────┤
│  Adapter Layer (internal/adapters)           │
│  Converts sql.ErrNoRows → core.ErrNotFound  │
│  Wraps infra errors with context             │
├─────────────────────────────────────────────┤
│  Infrastructure                              │
│  Raw errors: sql.ErrNoRows, exec.ExitError,  │
│  net.Error, context.DeadlineExceeded         │
└─────────────────────────────────────────────┘
```

Errors propagate **upward**. Each layer wraps with `fmt.Errorf("funcName: %w", err)`. The API layer converts to gRPC status codes via `ErrorToStatus()`.

## Business Error Catalog

Defined in `internal/core/domain.go`:

| Error | gRPC Code | Description | Retryable |
|-------|-----------|-------------|-----------|
| `ErrNotFound` | `NotFound` | Plugin not found in registry | No |
| `ErrInvalidPluginName` | `InvalidArgument` | Plugin name doesn't match format regex | No |
| `ErrGenerationFailed` | `Internal` | Code generation failed (Docker execution error) | Depends* || `ErrAlreadyExists` | `AlreadyExists` | Plugin with same group/name/version already exists | No |
| `ErrMaxPluginsExceeded` | `ResourceExhausted` | Max plugins limit reached (license) | No || `ErrServerOverloaded` | `ResourceExhausted` | WorkerPool queue full, request rejected | Yes |
| `ErrShuttingDown` | `Unavailable` | Server is shutting down, no new requests | Yes |
| `context.DeadlineExceeded` | `DeadlineExceeded` | Request timeout exceeded | Yes |
| `context.Canceled` | `Canceled` | Client canceled the request | No |
| *(any other error)* | `Internal` | Unexpected internal error | No |

*`ErrGenerationFailed` may wrap a transient Docker error — retry is handled internally by WorkerPool.

## Error-to-gRPC Mapping

`internal/api/api.go` — `ErrorToStatus()`:

```go
func ErrorToStatus(err error) *status.Status {
    code := codes.Internal
    switch {
    case errors.Is(err, core.ErrNotFound):
        code = codes.NotFound
    case errors.Is(err, core.ErrInvalidPluginName):
        code = codes.InvalidArgument
    case errors.Is(err, core.ErrGenerationFailed):
        code = codes.Internal
    case errors.Is(err, core.ErrAlreadyExists):
        code = codes.AlreadyExists
    case errors.Is(err, core.ErrMaxPluginsExceeded):
        code = codes.ResourceExhausted
    case errors.Is(err, core.ErrServerOverloaded):
        code = codes.ResourceExhausted
    case errors.Is(err, core.ErrShuttingDown):
        code = codes.Unavailable
    case errors.Is(err, context.DeadlineExceeded):
        code = codes.DeadlineExceeded
    case errors.Is(err, context.Canceled):
        code = codes.Canceled
    }
    return status.New(code, err.Error())
}
```

This function is used as `GRPCCodesConverterHandler` in the interceptor chain (position 7).

## Error Wrapping Convention

**Adapter → Core:**
```go
// adapters/registry — convert infrastructure errors to domain errors
func (r *registry) Get(ctx context.Context, group, name, version string) (Plugin, error) {
    // ... SQL query ...
    if errors.Is(err, sql.ErrNoRows) {
        return nil, core.ErrNotFound  // direct sentinel, no wrap
    }
    return nil, fmt.Errorf("query plugin: %w", err)  // wrap unknown errors
}
```

**Core → API:**
```go
// core/core.go — wrap with function context
plugin, err := c.registry.Get(ctx, group, name, version)
if err != nil {
    return nil, fmt.Errorf("c.registry.Get: %w", err)
}
```

**API handler:**
```go
// api/api.go — wrap with function context, ErrorToStatus handles the rest
resp, err := api.app.Generate(ctx, core.GenerateCodeRequest{...})
if err != nil {
    return nil, fmt.Errorf("api.app.Generate: %w", err)
}
```

## Retry Policy

### WorkerPool (server-side)

`internal/core/pool.go` — `isTransient()` determines which errors trigger retry:

| Error | Transient | Reason |
|-------|-----------|--------|
| `exec.ExitError` code 125 | Yes | Docker daemon error |
| `exec.ExitError` code 126 | Yes | Permission denied (container) |
| `exec.ExitError` code 127 | Yes | Command not found (container) |
| `"connection refused"` in message | Yes | Docker daemon not ready |
| `"daemon"` in message | Yes | Docker daemon issue |
| `"temporary failure"` in message | Yes | Transient network error |
| `context.DeadlineExceeded` | **No** | User's deadline, don't retry |
| Everything else | No | Permanent error |

Max retries: `WorkerPoolConfig.MaxRetries` (default: 2, so 3 total attempts).

### SDK (client-side)

`sdk/retry.go` — retries on specific gRPC codes with exponential backoff + jitter:

| gRPC Code | Retried |
|-----------|---------|
| `Unavailable` | Yes |
| `ResourceExhausted` | Yes |
| `DeadlineExceeded` | Yes |
| All others | No |

Max retries: `config.maxRetries` (default: 3). Base delay: 100ms. Max delay: 5s.
