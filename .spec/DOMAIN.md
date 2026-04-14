<!-- generated: 2026-04-14, template: core.md -->
# Domain Model

All domain types are defined in `internal/core/domain.go` — single source of truth.

## 1. Core Entities

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
    ID        uuid.UUID   // Auto-generated UUID
    Group     string      // Plugin group (e.g., "protocolbuffers")
    Name      string      // Plugin name (e.g., "go")
    Version   string      // Semver or "latest" (e.g., "v1.36.10")
    Tags      []string    // Categorization tags
    CreatedAt time.Time   // Registration timestamp
}
```

### CreatePluginRequest
```go
type CreatePluginRequest struct {
    Group   string          // ^[a-z][a-z0-9-]*$
    Name    string          // ^[a-z][a-z0-9-]*$
    Version string          // ^v\d+\.\d+\.\d+$ or "latest"
    Config  json.RawMessage // Docker execution config (JSONB)
    Tags    []string
}
```

### UpdatePluginRequest
```go
type UpdatePluginRequest struct {
    Group   string          // Immutable lookup key
    Name    string          // Immutable lookup key
    Version string          // Immutable lookup key
    Config  json.RawMessage // New config
    Tags    []string        // New tags
}
```

### PluginFilter
```go
type PluginFilter struct {
    Group   string    // Filter by group (exact match)
    Name    string    // Filter by name (exact match)
    Version string    // Filter by version (exact match)
    Tags    []string  // Filter by tags (array containment)
}
```

### AuditEntry
```go
type AuditEntry struct {
    ID            uuid.UUID
    OperationType string         // GENERATE_CODE, LIST_PLUGINS, CREATE_PLUGIN, UPDATE_PLUGIN, DELETE_PLUGIN
    PluginName    string         // Target plugin name
    CallerAddress string         // Client IP
    Status        string         // "success" or "error"
    ErrorCode     string         // gRPC status code (on error)
    ErrorMessage  string         // Error description (on error)
    DurationMs    int64          // Operation duration in milliseconds
    Metadata      map[string]any // Extra data (file counts, plugin counts, etc.)
    CreatedAt     time.Time      // Event timestamp
}
```

## 2. Enums / Value Objects

### Feature
```go
type Feature int

const (
    FeatureCodeGeneration  Feature = iota // Community
    FeaturePluginListing                  // Community
    FeatureMCPServerTools                 // Community
    FeatureRateLimiting                   // Community
    FeaturePluginCRUD                     // Community
    FeatureMultiTenancy                   // Enterprise only
    FeatureResponseCaching                // Enterprise only
    FeatureAudit                          // Enterprise only
)
```

### Audit Operation Types
```go
const (
    OperationGenerateCode = "GENERATE_CODE"
    OperationListPlugins  = "LIST_PLUGINS"
    OperationCreatePlugin = "CREATE_PLUGIN"
    OperationUpdatePlugin = "UPDATE_PLUGIN"
    OperationDeletePlugin = "DELETE_PLUGIN"
)
```

### Audit Statuses
```go
const (
    AuditStatusSuccess = "success"
    AuditStatusError   = "error"
)
```

## 3. Business Errors

For the full business error catalog (codes, gRPC mapping, retry policy) see `ERRORS.md`.

## 4. Interfaces

### CoreService — Business Logic Facade
```go
type CoreService interface {
    Generate(ctx context.Context, req GenerateCodeRequest) (*GenerateCodeResponse, error)
    ListPlugins(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
    CreatePlugin(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error)
    UpdatePlugin(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error)
    DeletePlugin(ctx context.Context, group, name, version string) error
}
```

### Registry — Plugin Storage + Execution
```go
type Registry interface {
    Get(ctx context.Context, pluginGroup, pluginName, pluginVersion string) (Plugin, error)
    List(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
    Create(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error)
    Update(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error)
    Delete(ctx context.Context, group, name, version string) error
}
```

### Plugin — Code Generator
```go
type Plugin interface {
    Generate(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error)
    Info(ctx context.Context) *PluginInfo
}
```

### Metrics — Business Metrics
```go
type Metrics interface {
    GenerateCode(ctx context.Context, info PluginInfo) error
    ObserveGenerationDuration(ctx context.Context, pluginName string, duration time.Duration)
    IncGenerationErrors(ctx context.Context, pluginName string, errorType string)
    IncGenerationRetries(ctx context.Context, pluginName string)
}
```

### AuditLog — Audit Event Persistence
```go
type AuditLog interface {
    Save(ctx context.Context, entry AuditEntry) error
}
```

### FeatureGate — License Feature Checks
```go
type FeatureGate interface {
    Enabled(feature Feature) bool
    MaxWorkers() int
    MaxPlugins() int  // -1 = unlimited
}
```
