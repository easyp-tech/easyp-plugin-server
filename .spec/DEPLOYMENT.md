<!-- generated: 2026-05-15, template: deployment.md -->
# Deployment

Deployment and infrastructure configuration for EasyP Service.

## Docker

### Service Dockerfile

`Dockerfile` — multi-stage build:
1. Build stage: `golang:alpine` → static binary
2. Runtime stage: minimal image

### Plugin Dockerfiles

Located in `registry/{group}/{name}/{version}/Dockerfile`:
- Multi-stage build (`golang:alpine` → `scratch`)
- Static binary with `upx` compression
- Runs as `nobody` (non-root)
- Runtime constraints: `--network=none --memory=128m --cpus=1.0`

## Docker Compose

`docker-compose.yml` provides a 14-service dev stack:

| Service | Port | Description |
|---------|------|-------------|
| service | 8080-8083 | EasyP gRPC/HTTP service |
| postgres | 5432 | PostgreSQL database |
| registry | 5005 | Docker registry (v2 API) |
| grafana | 3000 | Dashboards |
| loki | 3100 | Log aggregation |
| alloy | 4317-4318 | OpenTelemetry collector |
| tempo | 3200 | Distributed tracing |
| mimir | 9009 | Metrics storage |
| pyroscope | 4040 | Continuous profiling |
| ... | ... | Additional observability services |

### Minimal Stack

For local development:
```bash
task up-minimal  # postgres (port 5433) + docker registry (port 5005)
```

## GoReleaser

`.goreleaser.yaml` configures release builds:
- Cross-compilation targets
- Binary naming
- Archive formats
- Checksum generation

## Configuration

### Priority

CLI flags > environment variables > YAML config file.

### YAML Config

```yaml
server:
  host: 0.0.0.0
  port:
    grpc: "23410"
    metric: "23411"
    health: "23412"
    mcp: "23413"
db:
  driver: postgres
  postgres: "postgres://..."
  migrate_dir: migrate
registry:
  domain: "localhost:5005"
telemetry:
  otlp_endpoint: "localhost:4317"
  pyroscope_endpoint: "http://localhost:4040"
worker_pool:
  workers: 4
  queue_size: 16
  generation_timeout: 120s
  max_retries: 2
  shutdown_timeout: 30s
license:
  cache_ttl: 5m
rate_limit:
  requests_per_second: 10.0
  burst: 20
  cleanup_interval: 10m
```

### Environment Variables

All config fields have `env` tags. Prefix: section name (e.g., `SERVER_HOST`, `DB_POSTGRES_DSN`, `WORKER_POOL_WORKERS`).

## Requirements

- **Docker socket** — service mounts `/var/run/docker.sock` to execute plugin containers
- **PostgreSQL** — required for plugin metadata and audit logs
- **Docker Registry** — required for pulling plugin images
