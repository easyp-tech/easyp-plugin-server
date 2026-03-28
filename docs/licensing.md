# EasyP Licensing

## Overview

EasyP API Service uses a two-tier licensing model to control access to features and resource limits:

- **Community** (free, default) — includes core features: code generation, plugin listing, MCP server tools, rate limiting, and plugin CRUD. Limited to 4 workers and 10 plugins.
- **Enterprise** (paid) — unlocks all features including multi-tenancy, response caching, and audit, with higher resource limits (16 workers, unlimited plugins).

License tokens use the [PASETO v4.public](https://paseto.io/) format with Ed25519 asymmetric signatures. Tokens are signed with a private key (held by the license issuer) and verified with a public key embedded in the service binary at build time. All validation is performed offline — no network calls are required.

## Feature Tiers

| # | Feature Enum | String ID | Tier | Implemented | Description |
|---|-------------|-----------|------|-------------|-------------|
| 0 | `FeatureCodeGeneration` | `code_generation` | Community | ✓ | Code generation from protobuf via Docker plugins |
| 1 | `FeaturePluginListing` | `plugin_listing` | Community | ✓ | Listing available plugins |
| 2 | `FeatureMCPServerTools` | `mcp_server_tools` | Community | ✓ | MCP server tools (plugins_list, easyp_config_describe) |
| 3 | `FeatureRateLimiting` | `rate_limiting` | Community | ✓ | Per-client rate limiting (token bucket, by IP) |
| 4 | `FeaturePluginCRUD` | `plugin_crud` | Community | ✗ | Plugin CRUD operations (add/remove/update) |
| 5 | `FeatureMultiTenancy` | `multi_tenancy` | Enterprise | ✗ | Multi-tenancy (data isolation between clients) |
| 6 | `FeatureResponseCaching` | `response_caching` | Enterprise | ✗ | Generation response caching |
| 7 | `FeatureAudit` | `audit` | Enterprise | ✓ | Operation audit logging |

Feature enums are defined in `internal/license/features.go` as typed `int` constants using `iota`. The `IsEnterprise()` method returns `true` for features 5–7.

## Limits

| Limit | Community | Enterprise |
|-------|-----------|------------|
| Max workers | 4 | 16 |
| Max plugins | 10 | Unlimited |

License limits override the corresponding values from `config.yml`. When the service operates in Community mode, the limits above are enforced regardless of the configuration file settings.

## Configuration

The license token can be provided in two ways:

| Parameter | Env Variable | Description |
|-----------|-------------|-------------|
| `license.key` | `LICENSE_KEY` | Inline PASETO v4.public token string |
| `license.file` | `LICENSE_FILE` | Path to a file containing the PASETO token |

### Priority Rules

1. If `license.key` is set, it is used regardless of `license.file`. A warning is logged when both are specified.
2. If only `license.file` is set, the token is read from the specified file path.
3. If neither is set, the service starts in Community mode.

### Example

```yaml
# Enterprise license via inline token
license:
  key: "v4.public.eyJ..."

# Or via file path
license:
  file: "/etc/easyp/license.key"
```

Environment variables work the same way:

```bash
export LICENSE_KEY="v4.public.eyJ..."
# or
export LICENSE_FILE="/etc/easyp/license.key"
```

## Embedding the Public Key

PASETO v4.public uses asymmetric cryptography (Ed25519). The license issuer signs tokens with a **private key** (kept secret). The service verifies tokens using the corresponding **public key**, which is safe to distribute.

The public key is embedded into the service binary at build time via Go linker flags:

```bash
go build -ldflags "-X main.licensePublicKey=<hex-encoded-ed25519-public-key>" ./cmd/
```

This sets the `licensePublicKey` variable in the `main` package. At startup, the `LicenseManager` decodes this hex string into an Ed25519 public key and uses it to verify token signatures.

If no public key is embedded (the variable is empty), the service operates in Community mode regardless of any token configuration. This ensures the service always starts successfully.

## Graceful Degradation

The licensing system is designed to never cause service failures. All license-related errors result in a fallback to Community mode.

| Scenario | Behavior |
|----------|----------|
| No license configured | Community mode, all core features available |
| Expired license token | Community mode, warning logged with expiration timestamp |
| Invalid token (malformed) | Community mode, error logged with parsing failure reason |
| Invalid signature (tampered) | Community mode, error logged with verification failure |
| License expires at runtime | Transition to Community mode within 60 seconds, in-progress operations complete normally |
| No public key embedded | Community mode, no error |
| Both `key` and `file` set | `key` is used, warning logged about ambiguous configuration |
| License file not found | Community mode, error logged |

When the license expires during runtime:

1. The expiration watcher (60-second ticker) detects the expiration
2. The service transitions to Community mode automatically
3. In-progress Enterprise operations complete normally
4. Subsequent Enterprise requests are denied with gRPC `PERMISSION_DENIED`
5. All Community features continue without interruption
6. No restart is required

## Metrics and Monitoring

The licensing system exposes three Prometheus metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `easyp_license_valid` | Gauge | — | `1` when the license is valid, `0` when invalid or absent |
| `easyp_license_expiry_timestamp_seconds` | Gauge | — | Unix timestamp of the license expiration |
| `easyp_license_feature_denied_total` | Counter | `feature` | Number of denied feature access requests |

### PromQL Examples

License status:

```promql
easyp_license_valid
```

Time remaining until expiration (in seconds):

```promql
easyp_license_expiry_timestamp_seconds - time()
```

Feature denial rate over the last 5 minutes:

```promql
rate(easyp_license_feature_denied_total[5m])
```

Top denied features:

```promql
topk(5, sum by (feature) (rate(easyp_license_feature_denied_total[5m])))
```

### Alerting Recommendations

- Alert when the license becomes invalid: `easyp_license_valid == 0`
- Alert when the license expires within 7 days: `easyp_license_expiry_timestamp_seconds - time() < 7 * 24 * 3600`
- Alert on sustained feature denials: `rate(easyp_license_feature_denied_total[5m]) > 0`

## FAQ

**What happens when the license expires?**

The service transitions to Community mode automatically. Core features (code generation, plugin listing, MCP tools, rate limiting, plugin CRUD) continue working. Enterprise-only features (multi-tenancy, response caching, audit) are disabled. Community limits apply (`max_workers=4`, `max_plugins=10`). No restart is needed — the transition happens within 60 seconds.

**Can I run without a license?**

Yes. Without a license, the service operates in Community mode with all core features enabled. Community mode is the default.

**How do I upgrade from Community to Enterprise?**

Obtain a PASETO v4.public license token from the license issuer. Add it to your configuration via `license.key` (inline) or `license.file` (file path), or set the `LICENSE_KEY` / `LICENSE_FILE` environment variable. Restart the service or wait for the configuration to be reloaded.

**Is an internet connection required for license validation?**

No. License validation is performed entirely offline using the Ed25519 public key embedded in the binary. The token optionally contains a `refresh_url` claim for automatic license renewal, but this is not required — if the refresh request fails, the current token remains valid until its expiration date.

**What if both `license.key` and `license.file` are set?**

The `license.key` value takes priority. The token from `license.file` is ignored, and a warning is logged about the ambiguous configuration.
