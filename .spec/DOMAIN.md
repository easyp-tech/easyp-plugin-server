<!-- generated: 2026-04-03, template: core.md -->
# Domain Model

All domain types and interfaces are defined in `internal/core/domain.go`.

## Core Entities

### GenerateCodeRequest
```go
type GenerateCodeRequest struct {
    PluginName string                          // Format: "group/name:version"
    Payload    *pluginpb.CodeGeneratorRequest  // Protobuf source files + parameters
}
```

### GenerateCodeResponse
```go
type GenerateCodeResponse struct {
    Payload *pluginpb.CodeGeneratorResponse  // Generated files
}
```

### PluginInfo
```go
type PluginInfo struct {
    ID        uuid.UUID
    Group     string
    Name      string
    Version   string
    Tags      []string
    CreatedAt time.Time
}
```

### PluginFilter
```go
type PluginFilter struct {
    Group   string
    Name    string
    Version string
    Tags    []string
}
```

### CreatePluginRequest
```go
type CreatePluginRequest struct {
    Group   string
    Name    string
    Version string
    Tags    []string
    Config  json.RawMessage
}
```

### UpdatePluginRequest
```go
type UpdatePluginRequest struct {
    Group   string
    Name    string
    Version string
    Tags    []string
    Config  json.RawMessage
}
```

### AuditEntry
```go
type AuditEntry struct {
    ID            uuid.UUID
    OperationType string         // "GENERATE_CODE", "LIST_PLUGINS", "CREATE_PLUGIN", "UPDATE_PLUGIN", "DELETE_PLUGIN"
    PluginName    string
    CallerAddress string
    Status        string         // "success" or "error"
    ErrorCode     string
    ErrorMessage  string
    DurationMs    int64
    Metadata      map[string]any
    CreatedAt     time.Time
}
```

### WorkerPoolConfig
```go
type WorkerPoolConfig struct {
    Workers           int           // Default: 4
    QueueSize         int           // Default: 16
    GenerationTimeout time.Duration // Default: 120s
    MaxRetries        int           // Default: 2
    ShutdownTimeout   time.Duration // Default: 30s
}
```

## Interfaces

### Registry
```go
type Registry interface {
    Get(ctx context.Context, pluginGroup, pluginName, pluginVersion string) (Plugin, error)
    List(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
    Create(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error)
    Update(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error)
    Delete(ctx context.Context, group, name, version string) error
}
```
Implementations: `adapters/registry` (PostgreSQL + Docker), `core.WorkerPool` (decorator), `telemetry.TracingRegistry` (decorator).

### Plugin
```go
type Plugin interface {
    Generate(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error)
    Info(ctx context.Context) *PluginInfo
}
```
Implementations: `adapters/registry` (Docker exec), `core.poolPlugin` (timeout + retry decorator), `telemetry.TracingPlugin` (decorator).

### Metrics
```go
type Metrics interface {
    GenerateCode(ctx context.Context, info PluginInfo) error
    ObserveGenerationDuration(ctx context.Context, pluginName string, duration time.Duration)
    IncGenerationErrors(ctx context.Context, pluginName string, errorType string)
    IncGenerationRetries(ctx context.Context, pluginName string)
}
```
Implementation: `adapters/metrics`.

### AuditLog
```go
type AuditLog interface {
    Save(ctx context.Context, entry AuditEntry) error
}
```
Implementation: `adapters/audit`.

### FeatureGate
```go
type FeatureGate interface {
    Enabled(feature int) bool  // int (not license.Feature) to avoid cyclic imports
    MaxWorkers() int
    MaxPlugins() int           // -1 = unlimited
}
```
Implementation: `license.FeatureGate` (via adapter in `cmd/main.go`).

### CoreService
```go
type CoreService interface {
    Generate(ctx context.Context, req GenerateCodeRequest) (*GenerateCodeResponse, error)
    ListPlugins(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
    CreatePlugin(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error)
    UpdatePlugin(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error)
    DeletePlugin(ctx context.Context, group, name, version string) error
}
```
Implementations: `core.Core`, `telemetry.TracingCore` (decorator).

## Business Errors

For the full business error catalog (codes, gRPC mapping, retry policy) see `ERRORS.md`.

## Plugin Name Format

Format: `{group}/{name}:{version}`

- Example: `protocolbuffers/go:v1.36.10`
- Version `"latest"` resolves via `ORDER BY version DESC LIMIT 1`
- Validation regex: `^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*:(v\d+\.\d+\.\d+|latest)$`

## Audit Constants

```go
const (
    OperationGenerateCode = "GENERATE_CODE"
    OperationListPlugins  = "LIST_PLUGINS"
    OperationCreatePlugin = "CREATE_PLUGIN"
    OperationUpdatePlugin = "UPDATE_PLUGIN"
    OperationDeletePlugin = "DELETE_PLUGIN"
)
const (
    AuditStatusSuccess = "success"
    AuditStatusError   = "error"
)
```
