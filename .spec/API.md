<!-- generated: 2026-04-04, template: api.md -->
# API

## Transport

gRPC on port **8080** (H2, insecure credentials in-container). Reflection enabled.

## Service Definition

```
service ServiceAPI {
    rpc GenerateCode(GenerateCodeRequest) returns (GenerateCodeResponse);
    rpc Plugins(PluginsRequest) returns (PluginsResponse);
    rpc CreatePlugin(CreatePluginRequest) returns (CreatePluginResponse);
    rpc UpdatePlugin(UpdatePluginRequest) returns (UpdatePluginResponse);
    rpc DeletePlugin(DeletePluginRequest) returns (DeletePluginResponse);
}
```

Proto source: `api/generator/v1/`

## Handlers (`internal/api/api.go`)

### GenerateCode

1. Converts proto request → `core.GenerateCodeRequest{PluginName, Payload}`
2. Calls `core.CoreService.Generate(ctx, req)`
3. Returns `CodeGeneratorResponse` payload or error

### Plugins

1. Trims/compacts filter fields from `PluginsRequest`
2. Builds `core.PluginFilter{Group, Name, Version, Tags}`
3. Calls `core.CoreService.ListPlugins(ctx, filter)`
4. Converts `[]core.PluginInfo` → `[]*generator.PluginInfo` with `timestamppb`

### CreatePlugin

1. Converts `*structpb.Struct` config → `json.RawMessage` via `protojson.Marshal`
2. Trims/compacts fields, builds `core.CreatePluginRequest`
3. Calls `core.CoreService.CreatePlugin(ctx, req)`
4. Returns `CreatePluginResponse` with created `PluginInfo`
5. Enterprise-only: gated by `FeaturePluginCRUD` via `LicenseInterceptor`

### UpdatePlugin

1. Converts `*structpb.Struct` config → `json.RawMessage` via `protojson.Marshal`
2. Trims/compacts fields, builds `core.UpdatePluginRequest`
3. Calls `core.CoreService.UpdatePlugin(ctx, req)`
4. Returns `UpdatePluginResponse` with updated `PluginInfo`
5. Enterprise-only: gated by `FeaturePluginCRUD` via `LicenseInterceptor`

### DeletePlugin

1. Trims group/name/version fields
2. Calls `core.CoreService.DeletePlugin(ctx, group, name, version)`
3. Returns empty `DeletePluginResponse`
4. Enterprise-only: gated by `FeaturePluginCRUD` via `LicenseInterceptor`

## Error Mapping

`ErrorToStatus()` in `internal/api/api.go`:

| Domain Error | gRPC Code |
|-------------|-----------|
| `core.ErrNotFound` | `NOT_FOUND` |
| `core.ErrInvalidPluginName` | `INVALID_ARGUMENT` |
| `core.ErrGenerationFailed` | `INTERNAL` || `core.ErrAlreadyExists` | `ALREADY_EXISTS` |
| `core.ErrMaxPluginsExceeded` | `RESOURCE_EXHAUSTED` || `core.ErrServerOverloaded` | `RESOURCE_EXHAUSTED` |
| `core.ErrShuttingDown` | `UNAVAILABLE` |
| `context.DeadlineExceeded` | `DEADLINE_EXCEEDED` |
| `context.Canceled` | `CANCELED` |
| (default) | `INTERNAL` |

Registered as `GRPCCodesConverterHandler` in `grpchelper.NewServer`.

## Interceptor Chain

Built in `grpchelper.NewServer` → `buildUnaryInterceptors()`. Order matters:

| # | Interceptor | Package | Purpose |
|---|------------|---------|---------|
| 1 | TraceLogging | `grpchelper` | Injects trace/span IDs into slog |
| 2 | RealIP | `realip` | Extracts client IP from headers |
| 3 | ServerMetrics | `grpc-prometheus` | Prometheus request metrics |
| 4 | Logging | `logging` | Structured request/response logging |
| 5 | Recovery | `grpc_recovery` | Panic → `INTERNAL` + counter |
| 6 | Validator | `grpc_validator` | Proto field validation |
| 7 | CodeConverter | `grpchelper` | `ErrorToStatus()` conversion |
| 8 | RateLimit | `ratelimiter` | Per-IP token bucket (extra) |
| 9 | License | `api.LicenseInterceptor` | Feature gate check (extra) |
| 10 | Audit | `api.AuditInterceptor` | Async audit log (extra) |

Interceptors 1-7 are built-in; 8-10 are passed via `extraUnary`/`extraStream`.

## MCP Endpoint

Port **8083**, path `/mcp`. Streamable HTTP (not SSE).

Tools:
- `plugins_list` — plugin discovery (from `ServiceAPI.Plugins`)
- `easyp_config_describe` — easyp.yaml schema helpers (from `easyp` library)

See `internal/mcpserver/server.go`.

## Health Check

gRPC Health Check Protocol on the same port (8080). Status set to `SERVING` on registration.
HTTP health on port **8082**.

## Adding a New RPC

1. Add to `api/generator/v1/*.proto`
2. Run `easyp --cfg easyp.yaml generate`
3. Add handler method on `API` struct in `internal/api/api.go`
4. Map new domain errors in `ErrorToStatus()` if needed
5. If Enterprise-only: `licenseInterceptor.RegisterMethodFeature(fullMethodName, feature)`
6. Update audit `methodToOperationType()` mapping in `audit_interceptor.go`
