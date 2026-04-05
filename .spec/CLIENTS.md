<!-- generated: 2026-04-04, template: clients.md -->
# Clients (SDK)

## Go SDK (`sdk/`)

### Quick Start

```go
client, err := sdk.NewClient("localhost:8080", sdk.WithInsecure())
defer client.Close()

// Generate code
resp, err := client.GenerateCode(ctx, "protocolbuffers/go:v1.36.10", codeGenReq)

// List plugins
plugins, err := client.ListPlugins(ctx)

// List with filter
plugins, err := client.ListPlugins(ctx, sdk.PluginFilter{Group: "grpc"})
```

### Defaults

| Parameter | Default |
|-----------|---------|
| Transport | TLS (`credentials.NewTLS`) |
| Max retries | 3 |
| Retry base delay | 100ms |
| Retry max delay | 5s |
| GenerateCode timeout | 30s |
| ListPlugins timeout | 10s |
| Health check interval | 30s (disabled by default) |

### Options (Functional Options Pattern)

```go
sdk.NewClient(addr,
    sdk.WithInsecure(),                   // Disable TLS
    sdk.WithTransportCredentials(creds),  // Custom TLS
    sdk.WithMaxRetries(5),                // Retry attempts
    sdk.WithRetryBaseDelay(200*time.Millisecond),
    sdk.WithRetryMaxDelay(10*time.Second),
    sdk.WithGenerateCodeTimeout(60*time.Second),
    sdk.WithListPluginsTimeout(20*time.Second),
    sdk.WithUnaryInterceptor(interceptor), // Custom interceptor
    sdk.WithHealthCheck(true),             // Enable health monitor
    sdk.WithHealthCheckInterval(15*time.Second),
    sdk.WithKeepaliveParams(params),       // gRPC keepalive
)
```

### Retry Strategy

Built-in `retryUnaryInterceptor` (first in interceptor chain):

- **Transient codes:** `UNAVAILABLE`, `DEADLINE_EXCEEDED`, `RESOURCE_EXHAUSTED`
- **Backoff:** exponential with 25% random jitter
- **Formula:** `min(baseDelay * 2^attempt + jitter, maxDelay)`
- Respects context cancellation between attempts

### Client-Side Filtering

`sdk.PluginFilter` supports local filtering after server response:

```go
type PluginFilter struct {
    Group   string
    Name    string
    Version string
    Tags    []string
}
```

Server-side filtering sends fields to the gRPC endpoint. If `PluginFilter.isEmpty()`, no client-side filtering is applied.

### Health Monitor

When enabled via `WithHealthCheck(true)`, a background goroutine periodically calls gRPC Health Check. Stopped on `client.Close()`.

### Interceptor Chain

Client interceptor order:
1. Retry interceptor (built-in)
2. User-provided interceptors (via `WithUnaryInterceptor`)

### Timeout Handling

`withTimeout()` picks the earlier of:
- User's existing context deadline
- `now + defaultTimeout` (per-method)

If user's deadline is earlier, it is preserved.

## MCP Clients

Any MCP-compatible client can connect to `http://localhost:8083/mcp` (Streamable HTTP).

Available tools:
- `plugins_list` — discover available plugins
- `easyp_config_describe` — easyp.yaml schema reference
