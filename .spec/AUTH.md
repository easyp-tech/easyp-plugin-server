<!-- generated: 2026-04-14, template: auth.md -->
# Auth & Licensing

This project uses **PASETO v4.public** tokens for license-based feature gating, not traditional user authentication. There is no user login, session management, or OAuth.

## 1. Overview

| Aspect | Detail |
|--------|--------|
| Mechanism | PASETO v4.public token (asymmetric, Ed25519) |
| Tiers | Community (default) / Enterprise |
| Token source | Build-time (`-ldflags`), config file, or environment variable |
| Feature gating | `FeatureGate` interface checks per-request feature availability |

## 2. Architecture

```
License Token (PASETO v4.public)
  → LicenseManager.ParseToken()  [verify Ed25519 signature]
    → Claims { Tier, Features, MaxWorkers, MaxPlugins, ExpiresAt }
      → FeatureGate.Enabled(feature)  [called by interceptors and core]
```

## 3. License Tiers

### Community (default — no license required)
- 4 workers, 10 plugins max
- Features: CodeGeneration, PluginListing, MCPServerTools, RateLimiting, PluginCRUD

### Enterprise (valid license required)
- Configurable workers and plugins (-1 = unlimited)
- All Community features + MultiTenancy, ResponseCaching, Audit

## 4. Token Lifecycle

1. **Issuance**: External license server generates PASETO v4.public token
2. **Distribution**: Token set via `LICENSE_KEY` env var, `license.key` config, or `license.file` path
3. **Verification**: `LicenseManager.ParseToken()` verifies Ed25519 signature using public key
4. **Expiration**: 60-second watcher reverts to Community mode on expiry
5. **Reload**: `LicenseManager.Reload()` supports dynamic token replacement

### Public Key Injection
```bash
go build -ldflags "-X main.licensePublicKey=<base64-encoded-public-key>" -o easyp ./cmd/main.go
```

## 5. Claims Structure

```go
// internal/license/claims.go
type Claims struct {
    Tier       string    // "community" or "enterprise"
    Features   []string  // Enabled feature names
    MaxWorkers int       // Worker pool limit
    MaxPlugins int       // Plugin registry limit (-1 = unlimited)
    ExpiresAt  time.Time
    IssuedAt   time.Time
    Issuer     string
    Subject    string
    RefreshURL string    // Optional refresh endpoint
}
```

Community defaults:
```go
Claims{
    Tier:       "community",
    MaxWorkers: 4,
    MaxPlugins: 10,
    Features:   communityFeatures,
}
```

## 6. FeatureGate

```go
// internal/license/gate.go
type FeatureGate struct {
    manager *LicenseManager
}

func (g *FeatureGate) Enabled(feature Feature) bool
func (g *FeatureGate) MaxWorkers() int
func (g *FeatureGate) MaxPlugins() int
```

Logic:
- Enterprise tier → all features enabled
- Community tier → only community features and those explicitly in claims.Features
- Invalid feature → always false

## 7. License Interceptor

`internal/api/license_interceptor.go`:
- gRPC `UnaryServerInterceptor` + `StreamServerInterceptor`
- Maps gRPC methods to required features
- Returns `codes.PermissionDenied` (`core.ErrFeatureDenied`) if feature disabled
- Supports `RegisterMethodFeature()` for dynamic mapping

## 8. Configuration

```yaml
# config.yml
license:
  key: ""    # Inline PASETO v4.public token
  file: ""   # Path to license file containing the token
```

Environment variables:
```bash
LICENSE_KEY=<paseto-token>     # Inline token
LICENSE_FILE=/path/to/license  # File path
```

Priority: `license.key` > `license.file`

## 9. Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `license_valid` | Gauge | 1 if license is valid, 0 otherwise |
| `license_expiry_timestamp_seconds` | Gauge | Unix timestamp of license expiration |

## 10. License Errors

| Error | Description |
|-------|-------------|
| `ErrInvalidToken` | Token format is not valid PASETO |
| `ErrSignatureInvalid` | Ed25519 signature verification failed |
| `ErrTokenExpired` | Token past expiration date |
| `ErrInvalidClaims` | Claims payload is malformed |
| `ErrFileNotFound` | License file path does not exist |

On any error, service continues in **Community mode** (logged as warning).
