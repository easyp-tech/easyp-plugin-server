<!-- generated: 2026-04-14, template: feature-flags.md -->
# Feature Flags

## 1. Overview

- **System**: Custom license-based `FeatureGate` (not a third-party feature flag service)
- **Architecture**: Features defined as Go enum constants, checked via `FeatureGate.Enabled()`
- **Evaluation**: Synchronous, in-memory — claims loaded from PASETO token on startup
- **Default behavior**: Community tier features always enabled (fail-open for Community)

## 2. Feature Inventory

| Flag | Type | Tier | Description | Default |
|------|------|------|-------------|---------|
| `FeatureCodeGeneration` | boolean | Community | Protobuf code generation | Enabled |
| `FeaturePluginListing` | boolean | Community | List available plugins | Enabled |
| `FeatureMCPServerTools` | boolean | Community | MCP protocol tools | Enabled |
| `FeatureRateLimiting` | boolean | Community | Per-IP rate limiting | Enabled |
| `FeaturePluginCRUD` | boolean | Community | Create/Update/Delete plugins | Enabled |
| `FeatureMultiTenancy` | boolean | Enterprise | Multi-tenant isolation | Disabled |
| `FeatureResponseCaching` | boolean | Enterprise | Response caching | Disabled |
| `FeatureAudit` | boolean | Enterprise | Audit logging | Disabled |

## 3. Lifecycle

1. **Definition**: Features defined as `Feature` enum in `internal/core/domain.go` and mirrored as `feature` in `internal/license/features.go`
2. **Activation**: Enabled by license claims (claims contains `Features []string`)
3. **Evaluation**: `FeatureGate.Enabled(feature)` checks tier + claims
4. **Permanent**: Features are tied to license tiers, not toggled dynamically

## 4. Implementation Pattern

### Feature Definition
```go
// internal/core/domain.go
type Feature int

const (
    FeatureCodeGeneration  Feature = iota
    FeaturePluginListing
    FeatureMCPServerTools
    FeatureRateLimiting
    FeaturePluginCRUD
    FeatureMultiTenancy
    FeatureResponseCaching
    FeatureAudit
)
```

### Feature Check
```go
// internal/license/gate.go
func (g *FeatureGate) Enabled(feature Feature) bool {
    // Enterprise → all features enabled
    // Community → community features + claims.Features
}
```

### Enterprise Detection
```go
// internal/license/features.go
func (f feature) IsEnterprise() bool {
    return f == featureMultiTenancy || f == featureResponseCaching || f == featureAudit
}
```

### Usage in Business Logic
```go
// internal/core/core.go
if !c.gate.Enabled(FeaturePluginCRUD) {
    return nil, ErrFeatureDenied
}
```

### Usage in Interceptor
```go
// internal/api/license_interceptor.go
// Maps gRPC methods to required features
// Returns PermissionDenied if feature disabled
```

## 5. Configuration

Features are not configured individually — they're derived from the license tier:

| Tier | Features | Workers | Plugins |
|------|----------|---------|---------|
| Community | CodeGeneration, PluginListing, MCPServerTools, RateLimiting, PluginCRUD | 4 | 10 |
| Enterprise | All features | Configurable | Configurable (-1 = unlimited) |

License token contains explicit feature list in claims.

## 6. Adding a New Feature

1. Add constant to `Feature` enum in `internal/core/domain.go`
2. Add string name to `featureNames` array in `domain.go`
3. Mirror in `internal/license/features.go` (add constant + string)
4. Update `IsEnterprise()` if Enterprise-only
5. Add check in business logic: `if !c.gate.Enabled(FeatureNewFeature) { return ErrFeatureDenied }`
6. Optionally add to license interceptor for gRPC method-level gating
