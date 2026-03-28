# Deploying EasyP API Service

## Requirements

- **Docker** and **Docker Compose** — for running the service and plugins
- **Docker socket** (`/var/run/docker.sock`) — the service runs plugins as Docker containers
- **PostgreSQL** — storing the plugin registry and audit logs
- **Docker Registry** — storing plugin images

## Docker Compose (Development)

### Quick Start

```bash
# Start the entire stack
docker compose up --build --remove-orphans --detach

# Or using Task
task up

# Build and push plugins to the registry
./push.sh localhost:5005 --push
# Or
task local-push-registry
```

### Full Cycle

```bash
# Stop → start → push plugins → logs
task run
```

### Service Components

| Service | Version | Purpose |
|---------|---------|---------|
| PostgreSQL | 17.7 | Storing plugins and audit logs |
| Docker Registry | 3.0 | Storing plugin images |
| Grafana | 12.3 | Visualizing metrics, logs, traces |
| Mimir | 2.16 | Metrics storage (Prometheus-compatible) |
| Loki | 3.5 | Log storage |
| Tempo | 2.7.2 | Trace storage |
| Pyroscope | 1.13.5 | Continuous profiling |
| Alloy | v1.9.1 | Telemetry collector (replacement for Grafana Agent) |
| Traefik | 3.6 | Reverse proxy |
| RustFS | — | S3-compatible storage for Mimir/Loki/Tempo backends |

The service container mounts the Docker socket to execute plugins:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

## Docker Image

### Building

Multi-stage build: Go builder (Alpine) → Alpine runtime with docker-cli.

```dockerfile
FROM golang:alpine3.22 AS builder
# ... build the /easyp binary

FROM alpine:3.22
RUN apk add --no-cache docker-cli ca-certificates
COPY --from=builder /app/easyp /easyp
ENTRYPOINT ["/easyp"]
```

### Publishing

Images are published to `ghcr.io/easyp-tech/service` via GoReleaser.

**Multi-architecture**: amd64, arm64

**Tags**:
- `{version}-amd64` — image for amd64
- `{version}-arm64` — image for arm64
- `{version}` — multi-architecture manifest
- `latest` — latest version

### Running

```bash
docker run -d \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./config.yml:/config.yml \
  -p 8080:8080 \
  -p 8081:8081 \
  -p 8082:8082 \
  -p 8083:8083 \
  ghcr.io/easyp-tech/service:latest \
  -cfg /config.yml
```

## Health Check

```
GET http://host:8082/health
```

Checks the connection to PostgreSQL. Returns HTTP 200 on a successful connection.

## Graceful Shutdown

The service handles termination signals:

- `SIGHUP`
- `SIGINT`
- `SIGQUIT`
- `SIGABRT`
- `SIGTERM`

Shutdown order:

1. Graceful stop of the gRPC server (stop accepting new requests, complete in-flight ones)
2. Flush telemetry (send remaining traces and metrics)
3. Drain worker pool (wait for current code-generation tasks to finish)
4. Drain audit queue (write remaining audit events)
5. Close database connections

Forced termination after 15 seconds if graceful shutdown has not completed.

## Loading Plugins

Plugins are Docker images stored in a Docker Registry.

```bash
# Build and push all plugins
./push.sh localhost:5005 --push

# Or using Task
task local-push-registry
```

The `push.sh` script builds images from the `registry/` directory and pushes them to the specified registry.

## License Configuration

EasyP supports two license tiers: **Community** (free, default) and **Enterprise**. The license is a PASETO v4.public token verified against a public key embedded at build time.

### Building with a Public Key

Embed the Ed25519 public key into the binary at build time:

```bash
go build -ldflags "-X main.licensePublicKey=<hex-encoded-ed25519-public-key>" ./cmd/
```

If no public key is embedded, the service runs in Community mode.

### Providing the License Token

The license token can be supplied via the config file or environment variables:

```yaml
# config.yml
license:
  key: "v4.public.xxx..."       # inline token (takes priority)
  file: "/path/to/license.key"  # or path to a file containing the token
```

Environment variables: `LICENSE_KEY`, `LICENSE_FILE`. If both `key` and `file` are set, `key` takes priority.

### Docker Example

Mount a license file as a volume:

```bash
docker run -d \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./config.yml:/config.yml \
  -v ./license.key:/license.key \
  -p 8080:8080 -p 8081:8081 -p 8082:8082 -p 8083:8083 \
  ghcr.io/easyp-tech/service:latest \
  -cfg /config.yml
```

Or pass the token directly via an environment variable:

```bash
docker run -d \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./config.yml:/config.yml \
  -e LICENSE_KEY="v4.public.xxx..." \
  -p 8080:8080 -p 8081:8081 -p 8082:8082 -p 8083:8083 \
  ghcr.io/easyp-tech/service:latest \
  -cfg /config.yml
```

### Graceful Degradation

When a license expires or is absent, the service transitions to Community mode without interruption:

- All Community features remain fully operational (code generation, plugin listing, MCP tools, rate limiting, plugin CRUD)
- Enterprise-only features (multi-tenancy, response caching, audit) are disabled
- Community limits apply: `max_workers=4`, `max_plugins=10`
- In-progress operations complete normally; only subsequent Enterprise requests are denied
- The expiration watcher checks every 60 seconds and logs a warning on transition

No restart is required — the service continues serving requests throughout the transition.

### Monitoring License Status

Watch these Prometheus metrics to stay ahead of license issues:

| Metric | Type | Description |
|--------|------|-------------|
| `easyp_license_valid` | Gauge | `1` when the license is valid, `0` otherwise |
| `easyp_license_expiry_timestamp_seconds` | Gauge | Unix timestamp of the license expiration |
| `easyp_license_feature_denied_total` | Counter | Number of denied feature requests (label: `feature`) |

Useful PromQL queries:

- License status: `easyp_license_valid`
- Time until expiration: `easyp_license_expiry_timestamp_seconds - time()`
- Feature denial rate: `rate(easyp_license_feature_denied_total[5m])`

## Production Deployment Checklist

1. Configure PostgreSQL with strong credentials and SSL (`sslmode=require`)
2. Deploy a Docker Registry (or use an existing one)
3. Push plugin images to the registry
4. Mount the Docker socket into the service container
5. Set up monitoring (Prometheus scraping from the metrics port)
6. Configure a health check for the orchestrator
7. Set resource limits for the service container
8. Configure the worker pool according to the expected load
9. Configure the license: embed the public key at build time, provide the token via `license.key`, `license.file`, or `LICENSE_KEY` env var
10. Set up alerts on `easyp_license_valid == 0` and `easyp_license_expiry_timestamp_seconds - time() < 7*24*3600` (expiring within 7 days)
