<!-- generated: 2026-05-15, template: errors.md -->
# Errors

Business error catalog for EasyP Service.

## Domain Errors

All domain errors are sentinel variables defined in `internal/core/domain.go`.

| Error | gRPC Status | Error Code | Description |
|-------|-------------|------------|-------------|
| `ErrNotFound` | `NotFound` | `NOT_FOUND` | Plugin not found in registry |
| `ErrInvalidPluginName` | `InvalidArgument` | `INVALID_PLUGIN_NAME` | Plugin name doesn't match format `{group}/{name}:{version}` |
| `ErrGenerationFailed` | `Internal` | `GENERATION_FAILED` | Code generation failed (Docker container error) |
| `ErrServerOverloaded` | `ResourceExhausted` | `SERVER_OVERLOADED` | WorkerPool queue is full |
| `ErrShuttingDown` | `Unavailable` | `SHUTTING_DOWN` | Server is shutting down, not accepting new requests |
| `ErrAlreadyExists` | `AlreadyExists` | `ALREADY_EXISTS` | Plugin already registered |
| `ErrMaxPluginsExceeded` | `ResourceExhausted` | `MAX_PLUGINS_EXCEEDED` | License limit on plugin count reached |
| `ErrFeatureDenied` | `PermissionDenied` | `FEATURE_DENIED` | Feature not available in current license tier |

## Context Errors

| Error | gRPC Status | Description |
|-------|-------------|-------------|
| `context.DeadlineExceeded` | `DeadlineExceeded` | Request timeout |
| `context.Canceled` | `Canceled` | Request cancelled by client |

## Error Mapping

The `api.ErrorToStatus()` function in `internal/api/api.go` converts domain errors to gRPC status codes:

```go
func ErrorToStatus(err error) *status.Status {
    code := codes.Internal  // default
    switch {
    case errors.Is(err, core.ErrNotFound):           code = codes.NotFound
    case errors.Is(err, core.ErrInvalidPluginName):  code = codes.InvalidArgument
    case errors.Is(err, core.ErrServerOverloaded):   code = codes.ResourceExhausted
    case errors.Is(err, core.ErrAlreadyExists):      code = codes.AlreadyExists
    case errors.Is(err, core.ErrMaxPluginsExceeded): code = codes.ResourceExhausted
    case errors.Is(err, core.ErrShuttingDown):       code = codes.Unavailable
    case errors.Is(err, core.ErrFeatureDenied):      code = codes.PermissionDenied
    case errors.Is(err, context.DeadlineExceeded):   code = codes.DeadlineExceeded
    case errors.Is(err, context.Canceled):           code = codes.Canceled
    }
    return status.New(code, err.Error())
}
```

## Error Classification for Audit

The `errorCode()` function in `internal/core/core.go` produces string codes for audit entries:

```go
func errorCode(err error) string {
    switch {
    case errors.Is(err, ErrNotFound):           return "NOT_FOUND"
    case errors.Is(err, ErrInvalidPluginName):  return "INVALID_PLUGIN_NAME"
    case errors.Is(err, ErrGenerationFailed):   return "GENERATION_FAILED"
    case errors.Is(err, ErrServerOverloaded):   return "SERVER_OVERLOADED"
    case errors.Is(err, ErrShuttingDown):       return "SHUTTING_DOWN"
    case errors.Is(err, ErrAlreadyExists):      return "ALREADY_EXISTS"
    case errors.Is(err, ErrMaxPluginsExceeded): return "MAX_PLUGINS_EXCEEDED"
    case errors.Is(err, ErrFeatureDenied):      return "FEATURE_DENIED"
    default:                                     return "INTERNAL"
    }
}
```

## Transient Error Detection

The WorkerPool retries transient errors during code generation. An error is considered transient if:

| Condition | Transient? |
|-----------|-----------|
| `context.DeadlineExceeded` | No |
| Docker exit code 125 (daemon error) | Yes |
| Docker exit code 126 (command not found) | Yes |
| Docker exit code 127 (permission denied) | Yes |
| Error message contains "connection refused" | Yes |
| Error message contains "daemon" | Yes |
| Error message contains "temporary failure" | Yes |
| All other errors | No |

## Error Wrapping Convention

Every call site wraps errors with the calling function name:

```go
return nil, fmt.Errorf("c.registry.Get: %w", err)
```

This creates a traceable chain: `api.GenerateCode → core.Generate → c.registry.Get → <root cause>`.
