<!-- generated: 2026-04-04, template: deployment.md -->
# Deployment

## Docker Image

Multi-stage build (`Dockerfile`):

```
Stage 1: golang:alpine3.22 (builder)
  - go build -ldflags "-X main.licensePublicKey=${LICENSE_PUBLIC_KEY}" -o easyp ./cmd/main.go

Stage 2: alpine:3.22 (runtime)
  - apk add docker-cli ca-certificates
  - ENTRYPOINT ["/easyp"]
```

**Critical:** `docker-cli` is required in runtime — the service executes plugins as Docker containers.

Build arg `LICENSE_PUBLIC_KEY` embeds the Ed25519 public key for PASETO license verification. Empty = Community mode.

## Docker Compose Stack

14 services in `docker-compose.yml`:

| Service | Image | Purpose | Port |
|---------|-------|---------|------|
| service | local build | EasyP API | 8080-8083 |
| postgres | postgres:17.7 | Database | 5432 |
| registry | registry:3.0.0 | Docker v2 registry | 5005 |
| alloy | grafana/alloy:v1.9.1 | Telemetry collector | 12345 |
| grafana | grafana/grafana:12.3.0 | Dashboards | 3000 |
| mimir | grafana/mimir:2.16.0 | Metrics backend | — |
| loki | grafana/loki:3.5.0 | Log backend | — |
| tempo | grafana/tempo:2.7.2 | Trace backend | — |
| pyroscope | grafana/pyroscope:1.13.5 | Profiling backend | — |
| traefik | traefik:v3.6 | Reverse proxy | 80 |
| rustfs | rustfs/rustfs | S3-compatible storage | 9000-9001 |
| init-buckets | minio/mc | Creates S3 buckets | — |

### Service dependencies

```
service → postgres (healthy) + alloy (started)
alloy → loki + mimir + tempo
mimir, loki, tempo, pyroscope → init-buckets (completed)
init-buckets → rustfs
```

### Volume mounts for service

```yaml
- "./config.yml:/config.yml:ro"
- "./migrate:/migrate:ro"
- "/var/run/docker.sock:/var/run/docker.sock"  # Required for plugin execution
```

## Environment Variables

Key overrides via `${VAR:-default}`:

| Variable | Default | Purpose |
|----------|---------|---------|
| `EASYP_GRPC_PORT` | 8080 | gRPC port |
| `EASYP_METRICS_PORT` | 8081 | Metrics port |
| `EASYP_HEALTH_PORT` | 8082 | Health port |
| `EASYP_GATEWAY_PORT` | 8083 | MCP port |
| `EASYP_POSTGRES_PORT` | 5432 | Postgres port (use 5433 if local PG running) |
| `EASYP_REGISTRY_PORT` | 5005 | Docker registry port |
| `EASYP_GRAFANA_PORT` | 3000 | Grafana port |
| `LICENSE_PUBLIC_KEY` | "" | Build-time PASETO public key |
| `LICENSE_KEY` | "" | Runtime license token |

## CI/CD

### EasyP Lint (`.github/workflows/easyp.yml`)

Triggers: push/PR to `master`

- **Lint**: `easyp-tech/actions/lint@v1.1.1` — proto lint on `api/` directory
- **Breaking**: `easyp-tech/actions/breaking@v1.1.1` — breaking change detection against `master`

### Release (`.github/workflows/release.yml`)

Triggers: tags matching `v[0-9]+.[0-9]+.[0-9]+`

1. Setup QEMU + Docker Buildx
2. Login to GHCR (`ghcr.io`)
3. Validate semver tag
4. `goreleaser release --clean --timeout=90m`

## Quick Commands

```bash
task up              # Full 14-service stack
task up-minimal      # Postgres + registry only (port 5433)
task down            # Stop + remove volumes
task run             # down → up → push → logs
task run-local       # go run against minimal stack
task local-push-required  # Build + push essential plugin images
```
