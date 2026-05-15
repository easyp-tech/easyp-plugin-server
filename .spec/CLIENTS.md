<!-- generated: 2026-05-15, template: clients.md -->
# Clients

Client SDKs and tools for EasyP Service.

## Go SDK (`sdk/`)

Go client library for the EasyP gRPC API.

### Installation

```go
import "github.com/easyp-tech/service/sdk"
```

### Usage

```go
client, err := sdk.New(
    "localhost:8080",
    sdk.WithRetry(3, time.Second),
    sdk.WithHealthCheck(true),
)
```

### Features

| Feature | File | Description |
|---------|------|-------------|
| Functional options | `client.go` | `WithXxx()` pattern for configuration |
| Retry | `retry.go` | Automatic retry with configurable backoff |
| Health check | `health.go` | gRPC health check client |
| Client-side filtering | `filter.go` | Filter plugins by group, name, version, tags |
| Interceptors | `interceptors.go` | Client-side gRPC interceptors |

## MCP Server

Built-in MCP server exposed via HTTP.

### Endpoint

`POST /mcp` — streamable HTTP transport

### Tools

| Tool | Description | Read-Only |
|------|-------------|:---------:|
| `plugins_list` | List available plugins with filtering | ✅ |
| `easyp_config_describe` | Describe easyp.yaml configuration schema | ✅ |

### Implementation

- `internal/api/mcp.go` — HTTP handler factory
- `internal/api/mcp_tools.go` — Tool definitions
- `api/generator/v1/generator.mcp.go` — Generated MCP bindings (via `protoc-gen-mcp`)

### Connection

Any MCP-compatible client can connect:
```json
{
  "mcpServers": {
    "easyp": {
      "url": "http://localhost:8083/mcp"
    }
  }
}
```
