<!-- generated: 2026-04-14, template: errors.md -->
# Errors

## 1. Error Architecture

```
┌──────────────────────────────────────────────────────┐
│  Transport Layer (api/)                               │
│  ErrorToStatus() maps domain errors → gRPC codes     │
│  Interceptors log errors, never swallow               │
├──────────────────────────────────────────────────────┤
│  Application Layer (core/)                            │
│  Returns domain sentinel errors                       │
│  Wraps adapter errors with fmt.Errorf context         │
├──────────────────────────────────────────────────────┤
│  Adapter Layer (adapters/)                            │
│  Converts infra errors → domain sentinels            │
│  Wraps unknown errors with fmt.Errorf                │
├──────────────────────────────────────────────────────┤
│  Infrastructure                                       │
│  Raw errors: sql.ErrNoRows, Docker daemon errors     │
└──────────────────────────────────────────────────────┘
```

- **Adapters** create/convert errors: `sql.ErrNoRows` → `core.ErrNotFound`
- **Core** returns sentinels or wraps with context
- **API** maps errors to gRPC status codes via `ErrorToStatus()`
- **Errors are logged** by interceptors (structured logging), returned to client as gRPC status

## 2. Business Error Catalog

All domain errors defined in `internal/core/domain.go`:

| Name | gRPC Code | Description |
|------|-----------|-------------|
| `ErrNotFound` | `NotFound` | Plugin or resource not found |
| `ErrInvalidPluginName` | `InvalidArgument` | Plugin name fails regex validation |
| `ErrGenerationFailed` | `Internal` | Docker container code generation failed |
| `ErrServerOverloaded` | `ResourceExhausted` | WorkerPool queue full, cannot accept request |
| `ErrShuttingDown` | `Unavailable` | Server is shutting down, rejecting new work |
| `ErrAlreadyExists` | `AlreadyExists` | Plugin with same group/name/version already registered |
| `ErrMaxPluginsExceeded` | `ResourceExhausted` | License plugin limit reached |
| `ErrFeatureDenied` | `PermissionDenied` | Feature not available in current license tier |

Context errors also mapped in `ErrorToStatus()`:

| Error | gRPC Code |
|-------|-----------|
| `context.DeadlineExceeded` | `DeadlineExceeded` |
| `context.Canceled` | `Canceled` |

License errors defined in `internal/license/errors.go`:

| Name | Description |
|------|-------------|
| `ErrInvalidToken` | License token format is invalid |
| `ErrSignatureInvalid` | PASETO signature verification failed |
| `ErrTokenExpired` | License token has expired |
| `ErrInvalidClaims` | Claims payload is malformed |
| `ErrFileNotFound` | License file path does not exist |

## 3. Error Wrapping Convention

**Adapter → Core:**
```go
// internal/adapters/registry/registry.go
if errors.Is(err, sql.ErrNoRows) {
    return nil, core.ErrNotFound
}
return nil, fmt.Errorf("get plugin %s/%s:%s: %w", group, name, version, err)
```

**Core → API:**
```go
// internal/api/api.go
resp, err := api.app.Generate(ctx, core.GenerateCodeRequest{...})
if err != nil {
    return nil, fmt.Errorf("api.app.Generate: %w", err)
}
```

**API → Client (ErrorToStatus):**
```go
// internal/api/api.go
func ErrorToStatus(err error) *status.Status {
    code := codes.Internal
    switch {
    case errors.Is(err, core.ErrNotFound):
        code = codes.NotFound
    case errors.Is(err, core.ErrInvalidPluginName):
        code = codes.InvalidArgument
    case errors.Is(err, core.ErrServerOverloaded):
        code = codes.ResourceExhausted
    // ...
    }
    return status.New(code, err.Error())
}
```

## 4. Error Response Format

gRPC status with code and message:
```
status.New(codes.NotFound, "api.app.Generate: get plugin grpc/go:v1.5.1: not found")
```

Client receives `status.Status` with:
- `Code()` — gRPC status code
- `Message()` — Error message chain (wrapping preserved)

## 5. Sentinel Errors vs Error Types

This project uses **sentinel errors exclusively** — no typed error structs.

**Sentinel errors** (`var ErrX = errors.New("...")`):
- All domain errors in `core/domain.go`
- All license errors in `license/errors.go`
- Compared with `errors.Is()` after `fmt.Errorf("...: %w", err)` wrapping
- Simple and sufficient for the project's error taxonomy

## 6. Retry Policy

| Error | Retryable | Strategy |
|-------|-----------|----------|
| `ErrServerOverloaded` | Yes | Client-side exponential backoff (SDK) |
| `ErrShuttingDown` | Yes | Reconnect to different server |
| `ErrGenerationFailed` | Yes | WorkerPool auto-retries (configurable max_retries) |
| Docker transient errors | Yes | WorkerPool classifies and retries |
| `ErrNotFound` | No | — |
| `ErrInvalidPluginName` | No | — |
| `ErrFeatureDenied` | No | — |
| `ErrAlreadyExists` | No | — |
| `context.DeadlineExceeded` | No | Permanent in WorkerPool |

## 7. Error Logging

- **Interceptor layer** logs all errors via structured logging middleware
- **gRPC recovery interceptor** logs panics as Error level, increments `panics_total` counter
- **Audit interceptor** records error_code and error_message in audit entries
- **Never double-logged**: errors are logged by the interceptor chain, handlers just return them
- **Audit worker** logs overflow warnings when channel is full
