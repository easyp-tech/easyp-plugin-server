<!-- generated: 2026-04-03, template: development.md -->
# Testing

## Test Package Naming

Tests use **internal test packages** (same package as production code):

```go
package core  // not core_test — tests can access unexported types
```

This allows testing unexported functions like `isTransient()`, `getGroup()`, etc.

## Test File Structure

Example from `internal/core/pool_test.go`:

```go
package core

// --- Mock types (at file top) ---

type mockRegistry struct {
    getFn  func(ctx context.Context, group, name, version string) (Plugin, error)
    listFn func(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
}

func (m *mockRegistry) Get(ctx context.Context, group, name, version string) (Plugin, error) {
    return m.getFn(ctx, group, name, version)
}

type mockPlugin struct {
    generateFn func(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error)
    infoFn     func(ctx context.Context) *PluginInfo
}

type mockMetrics struct{}  // no-op implementation

// --- Helper ---
func noopLogger() *slog.Logger {
    return slog.New(slog.NewTextHandler(nopWriter{}, nil))
}

// --- Table-driven tests ---
func TestNewWorkerPool_ConfigNormalization(t *testing.T) {
    tests := []struct {
        name string
        in   WorkerPoolConfig
        want WorkerPoolConfig
    }{
        {
            name: "valid config unchanged",
            in:   WorkerPoolConfig{Workers: 4, QueueSize: 16, ...},
            want: WorkerPoolConfig{Workers: 4, QueueSize: 16, ...},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // assertions
        })
    }
}
```

## Key Patterns

### Manual Mocks

All mocks are **manually defined** in test files — no `mockgen` or `testify/mock`:

```go
type mockRegistry struct {
    getFn  func(ctx context.Context, group, name, version string) (Plugin, error)
    listFn func(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
}
```

Mock behavior is injected via function fields, allowing per-test customization.

### Table-Driven Tests

Standard Go pattern with subtests:

```go
tests := []struct {
    name string
    in   SomeInput
    want SomeOutput
}{...}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test logic
    })
}
```

### MCP Integration Tests

`internal/mcpserver/server_test.go` uses `httptest.Server` with a real MCP client:

```go
func TestMCPServer(t *testing.T) {
    // Create real MCP server with mock CoreService
    // Start httptest.Server
    // Connect real MCP client
    // Call plugins_list tool
    // Assert results
}
```

### No-op Logger Helper

Used across test files to suppress log output:

```go
func noopLogger() *slog.Logger {
    return slog.New(slog.NewTextHandler(nopWriter{}, nil))
}
```

## Test Files

| Package | Test File | Focus |
|---------|-----------|-------|
| `core` | `pool_test.go` | WorkerPool: config normalization, get/shutdown, retry, backpressure |
| `license` | `claims_test.go` | Claims parsing, community defaults |
| `license` | `features_test.go` | Feature enum, `IsEnterprise()`, `Valid()`, `String()` |
| `mcpserver` | `server_test.go` | MCP server integration with httptest |
| `sdk` | `client_test.go` | Client creation, GenerateCode, ListPlugins, Close |
| `sdk` | `retry_test.go` | Retry interceptor: backoff, jitter, retryable codes |
| `sdk` | `health_test.go` | Health monitor start/stop |
| `sdk` | `filter_test.go` | Plugin list filtering |
| `sdk` | `interceptors_test.go` | Logging and metrics interceptors |
| `telemetry` | `trace_handler_test.go` | Trace context propagation into slog records |

## Commands

```bash
# All unit tests
go test ./...

# Specific package
go test ./internal/core/...

# MCP integration test
go test ./internal/mcpserver -run TestMCPServer -count=1

# With race detector
go test -race ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./internal/license/...
```
