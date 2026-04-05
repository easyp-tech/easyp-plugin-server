<!-- generated: 2026-04-03, template: auth.md -->
# Authentication & Licensing

## Overview

EasyP uses **PASETO v4.public** tokens for license management with a two-tier model. There is no user authentication — the license system controls feature access and resource limits.

## Tiers

| Tier | How activated | Features | Limits |
|------|---------------|----------|--------|
| `community` | No public key or no token | 5 non-enterprise features | MaxWorkers=4, MaxPlugins=10 |
| `enterprise` | Valid PASETO token + public key | All 8 features | Per-token limits (-1 = unlimited) |

## Features

```go
// internal/license/features.go
const (
    FeatureCodeGeneration  Feature = iota // Community
    FeaturePluginListing                  // Community
    FeatureMCPServerTools                 // Community
    FeatureRateLimiting                   // Community
    FeaturePluginCRUD                     // Community
    FeatureMultiTenancy                   // Enterprise-only
    FeatureResponseCaching                // Enterprise-only
    FeatureAudit                          // Enterprise-only
)
```

`Feature.IsEnterprise()` returns `true` for the last three.

## Token Structure (Claims)

```go
// internal/license/claims.go
type Claims struct {
    Tier       Tier      `json:"tier"`        // "community" | "enterprise"
    Features   []Feature `json:"features"`    // Enabled features
    MaxWorkers int       `json:"max_workers"` // Worker pool limit
    MaxPlugins int       `json:"max_plugins"` // -1 = unlimited
    ExpiresAt  time.Time `json:"exp"`
    IssuedAt   time.Time `json:"iat"`
    Issuer     string    `json:"iss"`
    Subject    string    `json:"sub"`
    RefreshURL string    `json:"refresh_url,omitempty"`
}
```

`CommunityDefaults()` returns claims with all non-enterprise features, MaxWorkers=4, MaxPlugins=10.

## Component Chain

```
┌──────────────┐    ┌─────────────────┐    ┌─────────────┐    ┌──────────┐
│ LicenseManager│───→│  FeatureGate    │───→│ Interceptor │───→│ gRPC API │
│ (PASETO parse)│    │ (Enabled/Max*)  │    │ (per-method)│    │          │
└──────────────┘    └─────────────────┘    └─────────────┘    └──────────┘
                                                │
                                           ┌────▼────┐
                                           │RateLimiter│
                                           └─────────┘
```

### LicenseManager (`internal/license/manager.go`)

- Parses PASETO v4.public tokens using Ed25519 public key
- Public key injected at build time: `go build -ldflags "-X main.licensePublicKey=..."`
- Empty public key → Community mode (no token verification)
- Thread-safe claims cache (`sync.RWMutex`)
- `StartExpirationWatcher()` checks expiry every 60s
- `Reload(token)` hot-reloads a new token without restart
- Config priority: `license.key` env > `license.file` file path

### FeatureGate (`internal/license/gate.go`)

Algorithm for `Enabled(feature)`:
1. Invalid feature → `false`
2. Get claims from LicenseManager (thread-safe)
3. Tier == Enterprise → `true` (all features)
4. `feature.IsEnterprise()` → increment `featureDenied` metric, `false`
5. Check if feature in `claims.Features` list → result

Also exposes `MaxWorkers()` and `MaxPlugins()` from current claims.

### LicenseInterceptor (`internal/api/license_interceptor.go`)

- Maps gRPC method → `license.Feature` via `methodFeatures` map
- Methods not in map are treated as Community (pass-through)
- CRUD methods are registered at startup via `RegisterMethodFeature()`:
  - `ServiceAPI_CreatePlugin_FullMethodName` → `FeaturePluginCRUD`
  - `ServiceAPI_UpdatePlugin_FullMethodName` → `FeaturePluginCRUD`
  - `ServiceAPI_DeletePlugin_FullMethodName` → `FeaturePluginCRUD`
- `RegisterMethodFeature()` adds new Enterprise method checks at startup
- Returns `codes.PermissionDenied` for denied features

### Core FeatureGate bridge (`cmd/main.go`)

```go
// featureGateAdapter bridges license.Feature (typed) → core.FeatureGate (int)
type featureGateAdapter struct{ gate *license.FeatureGate }
func (a *featureGateAdapter) Enabled(feature int) bool { return a.gate.Enabled(license.Feature(feature)) }
func (a *featureGateAdapter) MaxWorkers() int           { return a.gate.MaxWorkers() }
func (a *featureGateAdapter) MaxPlugins() int           { return a.gate.MaxPlugins() }
```

This adapter prevents circular dependency between `core` and `license` packages.

## Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `license_tier` | Gauge | `tier` | Current license tier |
| `license_valid` | Gauge | — | 1 if license valid, 0 otherwise |
| `license_expiry_seconds` | Gauge | — | Seconds until expiry |
| `license_feature_denied_total` | Counter | `feature` | Enterprise feature denials |

## Adding a New Enterprise Feature

1. Add constant to `internal/license/features.go` (before `featureCount`)
2. Add string name to `featureNames` array
3. If enterprise-only, add to `IsEnterprise()` check
4. Register method in `LicenseInterceptor` via `RegisterMethodFeature()`
5. Check feature in business logic via `FeatureGate.Enabled()`
