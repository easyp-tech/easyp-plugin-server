<!-- generated: 2026-05-15, template: api.md -->
# API

gRPC API contract for EasyP Service.

## Service Definition

Proto: `api/generator/v1/generator.proto`

Package: `generator.v1`

### RPCs

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| `GenerateCode` | `GenerateCodeRequest` | `GenerateCodeResponse` | Run plugin in Docker, return generated code |
| `Plugins` | `PluginsRequest` | `PluginsResponse` | List available plugins with optional filtering |
| `CreatePlugin` | `CreatePluginRequest` | `CreatePluginResponse` | Register a new plugin in the registry |
| `UpdatePlugin` | `UpdatePluginRequest` | `UpdatePluginResponse` | Modify plugin config and tags |
| `DeletePlugin` | `DeletePluginRequest` | `DeletePluginResponse` | Remove a plugin from the registry |

### MCP Tools

Endpoint: `POST /mcp` (streamable HTTP transport)

| Tool | Description |
|------|-------------|
| `plugins_list` | List available plugins (wraps `Plugins` RPC) |
| `easyp_config_describe` | Describe easyp.yaml configuration schema |

## Error Mapping

See `ERRORS.md` for the full error catalog and gRPC status code mapping.

## Interceptor Chain

Order matters — applied in this sequence:

1. **trace_logging** — OpenTelemetry trace context extraction
2. **realip** — Extract real client IP from headers
3. **prometheus** — gRPC method latency/status metrics
4. **structured_logging** — slog request/response logging
5. **panic_recovery** — Recover panics, increment `panics_total` counter
6. **validation** — Protobuf field validation
7. **error_code_conversion** — `ErrorToStatus()` domain→gRPC mapping
8. **rate_limit** — Per-IP token bucket (via FeatureGate)
9. **license** — Feature availability check
10. **audit** — Async audit event via channel

## Plugin Name Format

```
{group}/{name}:{version}
```

Regex: `^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*:(v\d+\.\d+\.\d+|latest)$`

Examples: `protocolbuffers/go:v1.36.10`, `grpc/go:v1.5.1`

## Health Check

- **gRPC Health:** standard `grpc.health.v1.Health` service
- **HTTP Health:** `GET /health` — includes PostgreSQL connectivity check
