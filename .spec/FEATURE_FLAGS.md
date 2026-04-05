<!-- generated: 2026-04-04, template: feature_flags.md -->
# Feature Flags

## Overview

EasyP uses a license-based feature gate system instead of traditional feature flags. Features are controlled by the PASETO license token, not runtime toggles.

## Architecture

```
License Token (PASETO v4.public)
    ↓ parsed by
LicenseManager (internal/license/manager.go)
    ↓ cached Claims
FeatureGate (internal/license/gate.go)
    ↓ Enabled(feature) / MaxWorkers() / MaxPlugins()
Consumers: LicenseInterceptor, RateLimiter, Core (via adapter)
```

## Feature Registry

```go
// internal/license/features.go
const (
    FeatureCodeGeneration  Feature = iota // Community — basic code generation
    FeaturePluginListing                  // Community — list available plugins
    FeatureMCPServerTools                 // Community — MCP tool access
    FeatureRateLimiting                   // Community — per-IP rate limiting
    FeaturePluginCRUD                     // Community — plugin management
    FeatureMultiTenancy                   // Enterprise — multi-tenant isolation
    FeatureResponseCaching                // Enterprise — response caching
    FeatureAudit                          // Enterprise — audit logging
)
```

## Decision Algorithm

`FeatureGate.Enabled(feature)`:

1. `!feature.Valid()` → **false**
2. `claims.Tier == "enterprise"` → **true** (all features)
3. `feature.IsEnterprise()` → increment `feature_denied` metric → **false**
4. Check `feature ∈ claims.Features` → result

## Resource Limits

Beyond feature toggles, the license controls resource limits:

| Method | Community Default | Enterprise |
|--------|------------------|------------|
| `MaxWorkers()` | 4 | Per-token |
| `MaxPlugins()` | 10 | Per-token (-1 = unlimited) |

WorkerPool reads `MaxWorkers()` to size the goroutine pool. Core reads `MaxPlugins()` to limit plugin registry size.

## Consumers

| Consumer | Feature Checked | Action on Deny |
|----------|----------------|----------------|
| `LicenseInterceptor` | Method → Feature map (CRUD → `FeaturePluginCRUD`) | `PERMISSION_DENIED` |
| `RateLimiter.Limit()` | `FeatureRateLimiting` | Skip rate limiting (pass-through) |
| `Core.CreatePlugin()` | `MaxPlugins()` | `ErrMaxPluginsExceeded` |
| Worker Pool (planned) | `MaxWorkers()` | Limit pool size |

## Core FeatureGate Interface

To avoid circular dependency `core ↔ license`, core defines its own interface with `int` feature IDs:

```go
// internal/core/domain.go
type FeatureGate interface {
    Enabled(feature int) bool
    MaxWorkers() int
    MaxPlugins() int
}
```

Bridged via `featureGateAdapter` in `cmd/main.go`.

## Adding a New Feature Flag

1. Add constant to `internal/license/features.go` (before `featureCount`)
2. Add name to `featureNames` array
3. If enterprise-only: add to `IsEnterprise()` switch
4. Check in consumer: call `gate.Enabled(feature)` or `licenseInterceptor.RegisterMethodFeature()`
5. Update `communityFeatures()` if it should be available in Community tier
6. Add metric label handling if needed

## Hot Reload

`LicenseManager.Reload(token)` re-parses and caches new claims without service restart. The expiration watcher (60s ticker) automatically reverts to Community defaults on token expiry.
