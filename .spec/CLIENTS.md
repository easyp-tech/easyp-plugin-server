<!-- generated: 2026-04-14, template: clients.md -->
# Clients

## 1. Available Clients

| Client | Location | Technology | Purpose |
|--------|----------|------------|---------|
| Go SDK | `sdk/` | Go, gRPC | Programmatic access to EasyP service |
| MCP Smoke Test | `cmd/mcp-smoke/` | Go, HTTP | MCP protocol smoke testing |
| easyp CLI | External (`easyp-tech/easyp`) | Go, gRPC | Primary user-facing client (lint, generate, breaking) |

## 2. Go SDK

### Overview
Full-featured gRPC client with retry, health monitoring, and filtering.

**Package**: `github.com/easyp-tech/service/sdk`

### API

```go
client, err := sdk.NewClient(ctx, "localhost:8080",
    sdk.WithInsecure(),
    sdk.WithMaxRetries(3),
    sdk.WithTimeout(30*time.Second),
)
defer client.Close()

// Generate code
resp, err := client.GenerateCode(ctx, pluginName, codeGenRequest)

// List plugins
plugins, err := client.ListPlugins(ctx, sdk.PluginFilter{Group: "grpc"})
```

### Features

| Feature | Description | Default |
|---------|-------------|---------|
| Retry | Exponential backoff on transient errors | 3 retries |
| Health check | Background monitor (gRPC health protocol) | 30s interval |
| Timeout | Per-RPC deadline | 30s Generate, 10s List |
| Keepalive | gRPC keepalive parameters | Enabled |
| Filtering | Client-side plugin filtering | — |
| Interceptors | Logging and metrics | Optional |

### Configuration (Functional Options)

```go
sdk.WithInsecure()                    // No TLS
sdk.WithMaxRetries(n)                 // Retry count
sdk.WithTimeout(d)                    // RPC timeout
sdk.WithHealthCheckInterval(d)        // Health monitor interval
sdk.WithKeepalive(params)             // gRPC keepalive
sdk.WithUnaryInterceptors(...)        // Custom unary interceptors
sdk.WithStreamInterceptors(...)       // Custom stream interceptors
```

### Files

| File | Description |
|------|-------------|
| `sdk/client.go` | Client struct, NewClient, GenerateCode, ListPlugins, Close |
| `sdk/config.go` | Functional options (WithXxx) |
| `sdk/filter.go` | PluginFilter for client-side filtering |
| `sdk/health.go` | Background health monitor |
| `sdk/interceptors.go` | Logging/metrics interceptors |
| `sdk/retry.go` | Retry with exponential backoff |
| `sdk/doc.go` | Package documentation |

## 3. MCP Protocol

The service exposes an MCP endpoint for LLM tool integration:

- **Endpoint**: `http://host:8083/mcp`
- **Transport**: StreamableHTTP
- **Tools**: `plugins_list`, `generate_code`, `easyp_config_describe`
- **MCP smoke test**: `go run ./cmd/mcp-smoke --endpoint http://localhost:8083/mcp`

## 4. API Communication

```
┌──────────────┐  gRPC (:8080)  ┌──────────────┐
│  Go SDK      │──────────────→│  EasyP       │
│  easyp CLI   │                │  Service     │
└──────────────┘                │              │
                                │              │
┌──────────────┐  HTTP (:8083)  │              │
│  LLM / MCP   │──────────────→│              │
│  Client      │  /mcp          └──────────────┘
└──────────────┘
```
