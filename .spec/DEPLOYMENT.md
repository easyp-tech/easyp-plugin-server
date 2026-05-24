<!-- generated: 2026-05-24, template: deployment.md -->
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
- Used for building binaries only (via `docker build --output`)

### Plugin Build Process

Plugins are built as local binaries, not pushed to a Docker registry:

```bash
# build-plugins.sh extracts binaries from Docker multi-stage builds
docker build --output=plugins/{group}/{name}/{version}/ registry/{group}/{name}/{version}/
```

Result: `plugins/{group}/{name}/{version}/plugin` — a static Linux binary.

At runtime, the service executes these binaries directly via stdin/stdout (configured via `registry.plugins_dir`).

### Plugin Registration

Plugins must be registered in PostgreSQL before use:

```bash
# Requires grpcurl and a running service
./register-plugins.sh [host:port]

# Registers via gRPC CreatePlugin API with config containing command path
grpcurl -plaintext -d '{"group":"grpc","name":"go","version":"v1.5.1","config":{"command":["/plugins/grpc/go/v1.5.1/plugin"]}}' \
  localhost:8080 api.generator.v1.ServiceAPI/CreatePlugin
```

## Docker Compose

`docker-compose.yml` provides a full dev stack:

| Service | Port | Description |
|---------|------|-------------|
| service | 8080-8083 | EasyP gRPC/HTTP service |
| postgres | 5432 | PostgreSQL database |
| traefik | 80 | Reverse proxy |
| rustfs | 9000-9001 | S3-compatible storage (for observability backends) |
| grafana | 3000 | Dashboards |
| loki | — | Log aggregation |
| alloy | 12345 | OpenTelemetry collector (replaces Prometheus scraper) |
| tempo | — | Distributed tracing |
| mimir | — | Metrics storage |
| pyroscope | — | Continuous profiling |
| init-buckets | — | One-shot: creates S3 buckets for observability |

### Minimal Stack

For local development:
```bash
task up-minimal  # postgres only (port 5433)
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

### Config Files

| File | Purpose |
|------|---------|
| `config.yml` | Docker-compose service config (internal hostnames) |
| `config.local.yml` | Local development config (localhost, port 5433) |

### YAML Config Structure

```yaml
server:
  host: 0.0.0.0
  port:
    grpc: 8080
    metric: 8081
    health: 8082
    mcp: 8083
db:
  driver: postgres
  postgres: "postgres://easyp_svc:easyp_pass@localhost:5433/easyp_db?sslmode=disable"
  migrate_dir: migrate
registry:
  plugins_dir: "./plugins"        # Directory with built plugin binaries
  max_output_size: 67108864       # 64MB max plugin output
telemetry:
  otlp_endpoint: "localhost:4317"
  pyroscope_endpoint: "http://localhost:4040"
worker_pool:
  workers: 4
  queue_size: 16
  generation_timeout: 120s
  max_retries: 3
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

### Customizable Ports (docker-compose)

| Variable | Default | Description |
|----------|---------|-------------|
| `EASYP_POSTGRES_PORT` | 5432 | PostgreSQL host port |
| `EASYP_GRPC_PORT` | 8080 | gRPC API host port |
| `EASYP_METRICS_PORT` | 8081 | Metrics host port |
| `EASYP_HEALTH_PORT` | 8082 | Health host port |
| `EASYP_GATEWAY_PORT` | 8083 | MCP/Gateway host port |
| `EASYP_GRAFANA_PORT` | 3000 | Grafana host port |
| `EASYP_TRAEFIK_PORT` | 80 | Traefik host port |

## Requirements

- **PostgreSQL** — required for plugin metadata and audit logs
- **Plugin binaries** — must be built via `task build-plugins` before service can generate code
- **grpcurl** — required for plugin registration via `register-plugins.sh`
