<!-- generated: 2026-04-14, template: deployment.md -->
# Deployment

## 1. Overview

Docker-based deployment with docker-compose for local/dev. Production target: Docker containers with external orchestration.

```
Source → go build (multi-stage Dockerfile) → Docker image → docker-compose up
```

## 2. Environments

| Environment | Access | Purpose | Config |
|-------------|--------|---------|--------|
| Local (source) | `localhost:23410-23413` | Development from source | `config.local.yml` |
| Local (Docker) | `localhost:8080-8083` | Full stack development | `config.yml` |

## 3. Docker

### Dockerfile (multi-stage)

```dockerfile
# Stage 1: Build
FROM golang:alpine3.22 AS builder
ARG LICENSE_PUBLIC_KEY=""
COPY go.mod go.mod
RUN go mod download
COPY . /app
WORKDIR /app
RUN go build -ldflags "-X main.licensePublicKey=${LICENSE_PUBLIC_KEY}" -o easyp ./cmd/main.go

# Stage 2: Runtime
FROM alpine:3.22
RUN apk add --no-cache docker-cli ca-certificates
COPY --from=builder /app/easyp /easyp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/easyp"]
```

Key points:
- `docker-cli` required in runtime for plugin container execution
- `LICENSE_PUBLIC_KEY` build arg for Enterprise features
- Minimal Alpine runtime image

### Docker Compose (14 services)

| Service | Image | Port | Purpose |
|---------|-------|------|---------|
| service | (built) | 8080-8083 | EasyP API service |
| postgres | postgres:17 | 5432/5433 | Database |
| registry | registry:2 | 5005 | Docker plugin registry |
| grafana | grafana/grafana | 3000 | Dashboards |
| tempo | grafana/tempo | — | Distributed tracing |
| loki | grafana/loki | — | Log aggregation |
| mimir | grafana/mimir | — | Metrics storage |
| alloy | grafana/alloy | 4317 | OTEL collector |
| pyroscope | grafana/pyroscope | 4040 | Profiling |
| traefik | traefik | 80 | Reverse proxy |
| rustfs | — | 9000-9001 | S3-compatible storage |
| init-buckets | — | — | Create S3 buckets |

```bash
# Start everything
task up

# Start minimal (Postgres + Registry only)
task up-minimal

# Stop and clean volumes
task down

# Follow service logs
docker compose logs -f service
```

## 4. Health Checks

| Endpoint | Type | Port | Checks |
|----------|------|------|--------|
| `/health` | HTTP | 8082/23412 | PostgreSQL connectivity |

Implemented via `hellofresh/health-go` with Postgres health check.

## 5. Secrets Management

| Secret | Purpose | Where Set |
|--------|---------|-----------|
| `DB_POSTGRES_DSN` | Database connection string | Config file / env var |
| `LICENSE_KEY` | PASETO license token | Config file / env var |
| `LICENSE_FILE` | Path to license file | Config file / env var |
| `LICENSE_PUBLIC_KEY` | Ed25519 public key | Build-time `-ldflags` |

Config priority: CLI flags > env vars > YAML file.

## 6. Plugin Docker Images

Plugin containers are executed by the service via Docker socket:

```bash
# Build and push required plugins
task local-push-required

# Build and push all plugins
task local-push-registry
```

**Plugin image layout** (multi-stage):
1. Build: `golang:alpine` → compile protoc plugin
2. Compress with `upx`
3. Runtime: `scratch` image, run as `nobody`
4. Execution flags: `--network=none --memory=128m --cpus=1.0`

**Requirement**: Docker socket mounted at `/var/run/docker.sock`

## 7. Enterprise Deployment

```bash
# Build with license public key
docker build --build-arg LICENSE_PUBLIC_KEY="<base64-key>" -t easyp-service .

# Run with license token
LICENSE_KEY="<paseto-token>" task up
```
