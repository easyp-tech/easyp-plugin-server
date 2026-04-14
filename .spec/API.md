<!-- generated: 2026-04-14, template: api.md -->
# API

## 1. Overview

gRPC API defined in `api/generator/v1/generator.proto`. Framework: `google.golang.org/grpc`.

Secondary API: MCP over HTTP at `/mcp` for LLM tool integration.

## 2. Middleware Stack (Interceptor Chain)

Request processing order (unary):

| Order | Interceptor | Purpose | Package |
|-------|------------|---------|---------|
| 1 | TraceLogging | Inject trace_id into slog context | `grpchelper` |
| 2 | RealIP | Extract client IP from headers | `go-grpc-middleware/realip` |
| 3 | Prometheus | Record gRPC metrics (latency, codes) | `go-grpc-middleware/prometheus` |
| 4 | Structured Logging | Log request start/finish with payload | `go-grpc-middleware/logging` |
| 5 | Recovery | Catch panics → `codes.Internal` + counter | `go-grpc-middleware/recovery` |
| 6 | Validator | Protobuf field validation | `go-grpc-middleware/validator` |
| 7 | Code Conversion | Domain errors → gRPC status codes | `grpchelper` |
| 8 | Rate Limit | Per-IP token bucket | `go-grpc-middleware/ratelimit` |
| 9 | License | Feature gate check per method | `api` |
| 10 | Audit | Record operation to async channel | `api` |

## 3. Endpoint Reference

**Service: `api.generator.v1.ServiceAPI`**

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| `GenerateCode` | `GenerateCodeRequest` | `GenerateCodeResponse` | Execute protobuf plugin in Docker, return generated files |
| `Plugins` | `PluginsRequest` | `PluginsResponse` | List available plugins with optional filters |
| `CreatePlugin` | `CreatePluginRequest` | `CreatePluginResponse` | Register a new plugin |
| `UpdatePlugin` | `UpdatePluginRequest` | `UpdatePluginResponse` | Update plugin config and tags |
| `DeletePlugin` | `DeletePluginRequest` | `DeletePluginResponse` | Remove a plugin |

**Handler source:** `internal/api/api.go`

## 4. Request / Response Types

### GenerateCodeRequest
```protobuf
message GenerateCodeRequest {
  google.protobuf.compiler.CodeGeneratorRequest code_generator_request = 1;
  string plugin_name = 2;  // Format: "group/name:version"
}
```

### PluginsRequest (filtering)
```protobuf
message PluginsRequest {
  optional string group = 1;
  optional string name = 2;
  optional string version = 3;
  repeated string tags = 4;
}
```

### PluginsResponse
```protobuf
message PluginsResponse {
  repeated PluginInfo plugins = 1;
  int32 total = 2;
}
```

### PluginInfo
```protobuf
message PluginInfo {
  string id = 1;
  string group = 2;
  string name = 3;
  string version = 4;
  google.protobuf.Timestamp created_at = 5;
  repeated string tags = 6;
}
```

## 5. Error Mapping

Domain errors are mapped to gRPC status codes by `ErrorToStatus()` in `internal/api/api.go`:

| Domain Error | gRPC Code | When |
|-------------|-----------|------|
| `ErrNotFound` | `NotFound` | Plugin not in registry |
| `ErrInvalidPluginName` | `InvalidArgument` | Name fails regex validation |
| `ErrGenerationFailed` | `Internal` | Docker execution failed |
| `ErrServerOverloaded` | `ResourceExhausted` | Worker pool queue full |
| `ErrAlreadyExists` | `AlreadyExists` | Plugin already registered |
| `ErrMaxPluginsExceeded` | `ResourceExhausted` | License plugin limit reached |
| `ErrShuttingDown` | `Unavailable` | Server shutting down |
| `ErrFeatureDenied` | `PermissionDenied` | Feature not in license |
| `context.DeadlineExceeded` | `DeadlineExceeded` | Timeout |
| `context.Canceled` | `Canceled` | Client canceled |
| (default) | `Internal` | Unknown error |

## 6. Rate Limiting

- **Strategy**: Per-IP token bucket (`golang.org/x/time/rate`)
- **Defaults**: 10 req/sec, burst 20
- **Feature-gated**: Only active when `FeatureRateLimiting` is enabled
- **Headers returned**:
  - `x-ratelimit-limit` — Configured burst
  - `x-ratelimit-remaining` — Tokens left
  - `x-ratelimit-reset` — Seconds until token replenishment
- **Configuration**:
  ```yaml
  rate_limit:
    requests_per_second: 10.0
    burst: 20
    cleanup_interval: 10m
  ```

## 7. Proto Schema

**File**: `api/generator/v1/generator.proto`

**Generation command**:
```bash
easyp --cfg easyp.yaml generate
```

**Generated files**:
- `generator.pb.go` — Protobuf types
- `generator_grpc.pb.go` — gRPC client/server stubs
- `generator.mcp.go` — MCP tool bindings

## 8. MCP API

**Endpoint**: `GET/POST http://host:8083/mcp`
**Transport**: StreamableHTTP
**Server name**: `easyp-service-mcp`

**Tools exposed**:
- `plugins_list` — List available plugins
- `generate_code` — Execute code generation
- `easyp_config_describe` — Describe easyp configuration

**Source**: `internal/api/mcp.go`, `internal/api/mcp_tools.go`

## 9. Validation

- **Protobuf validation**: `grpc_validator.UnaryServerInterceptor()` validates protobuf field constraints
- **Business validation**: `core.Core` validates plugin names with regex:
  - Group/Name: `^[a-z][a-z0-9-]*$`
  - Version: `^v\d+\.\d+\.\d+$` or `latest`
- **Input sanitization**: `strings.TrimSpace()` on all string inputs in API handlers
- **Empty tag filtering**: `compactStrings()` removes empty/whitespace-only tags

## 10. Server Configuration

gRPC server features:
- Insecure credentials (TLS terminated by reverse proxy)
- OpenTelemetry instrumentation (`otelgrpc.NewServerHandler()`)
- Keepalive: 50s idle, 10s timeout, 30s min between pings
- gRPC reflection enabled
- Health service registered
