# EasyP API Service Configuration

## Configuration Priority

Values are applied in the following order (from highest to lowest priority):

1. **CLI flags**
2. **Configuration file** (`config.yml`)
3. **Environment variables**
4. **Default values**

## CLI Flags

| Flag | Description | Example |
|------|-------------|---------|
| `-cfg` | Path to the configuration file | `-cfg config.yml` |
| `-log_level` | Logging level: `debug`, `info`, `warn`, `error` | `-log_level debug` |

```bash
./easyp -cfg config.yml -log_level debug
```

## Configuration File (`config.yml`)

```yaml
server:
  host: "0.0.0.0"
  port:
    grpc: 8080        # gRPC API
    metric: 8081      # Prometheus metrics
    health: 8082      # Health checks
    gateway: 8083     # MCP server

db:
  migrate_dir: "migrate"
  driver: "postgres"
  postgres: "postgres://easyp_svc:easyp_pass@postgres:5432/easyp_db?sslmode=disable"

registry:
  domain: "localhost:5005"

telemetry:
  otlp_endpoint: "alloy:4317"
  pyroscope_endpoint: "http://pyroscope:4040"

worker_pool:
  workers: 4              # Number of parallel Docker workers
  queue_size: 16          # Task queue buffer size
  generation_timeout: 120s # Maximum generation time
  max_retries: 3          # Retries on transient errors
  shutdown_timeout: 30s   # Graceful shutdown timeout
```

## Environment Variables

All configuration parameters can be overridden via environment variables.

### Server

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_HOST` | Server bind address | `0.0.0.0` |
| `SERVER_PORT_GRPC` | gRPC API port | `8080` |
| `SERVER_PORT_METRIC` | Prometheus metrics port | `8081` |
| `SERVER_PORT_HEALTH` | Health check port | `8082` |
| `SERVER_PORT_MCP` | MCP server port | `8083` |

### Database

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_MIGRATE_DIR` | Migrations directory | `migrate` |
| `DB_DRIVER` | Database driver | `postgres` |
| `DB_POSTGRES_DSN` | PostgreSQL connection string | — |

### Registry

| Variable | Description | Default |
|----------|-------------|---------|
| `REGISTRY_DOMAIN` | Docker Registry domain | `localhost:5005` |

### Telemetry

| Variable | Description | Default |
|----------|-------------|---------|
| `TELEMETRY_OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | `alloy:4317` |
| `TELEMETRY_PYROSCOPE_ENDPOINT` | Pyroscope endpoint | `http://pyroscope:4040` |

### Worker Pool

| Variable | Description | Default |
|----------|-------------|---------|
| `WORKER_POOL_WORKERS` | Number of workers | `4` |
| `WORKER_POOL_QUEUE_SIZE` | Queue size | `16` |
| `WORKER_POOL_GENERATION_TIMEOUT` | Generation timeout | `120s` |
| `WORKER_POOL_MAX_RETRIES` | Maximum retries | `3` |
| `WORKER_POOL_SHUTDOWN_TIMEOUT` | Shutdown timeout | `30s` |

## Docker Compose Environment Variables

The following variables are used to customize ports in `docker-compose.yml`:

| Variable | Description | Default |
|----------|-------------|---------|
| `EASYP_TRAEFIK_PORT` | Traefik port (reverse proxy) | `80` |
| `EASYP_GRAFANA_PORT` | Grafana port | `3000` |
| `EASYP_REGISTRY_PORT` | Docker Registry port | `5005` |
| `EASYP_POSTGRES_PORT` | PostgreSQL port | `5432` |
| `EASYP_GRPC_PORT` | gRPC API port | `8080` |
| `EASYP_METRICS_PORT` | Metrics port | `8081` |
| `EASYP_HEALTH_PORT` | Health check port | `8082` |
| `EASYP_GATEWAY_PORT` | MCP server port | `8083` |

Example of running with custom ports:

```bash
EASYP_GRAFANA_PORT=3001 EASYP_GRPC_PORT=9090 docker compose up -d
```

## Examples

### Minimal Development Configuration

```yaml
server:
  host: "0.0.0.0"
  port:
    grpc: 8080
db:
  postgres: "postgres://easyp_svc:easyp_pass@localhost:5432/easyp_db?sslmode=disable"
registry:
  domain: "localhost:5005"
```

### Production Configuration

```yaml
server:
  host: "0.0.0.0"
  port:
    grpc: 8080
    metric: 8081
    health: 8082
    gateway: 8083
db:
  migrate_dir: "migrate"
  driver: "postgres"
  postgres: "postgres://user:password@db-host:5432/easyp_db?sslmode=require"
registry:
  domain: "registry.example.com"
telemetry:
  otlp_endpoint: "otel-collector:4317"
  pyroscope_endpoint: "http://pyroscope:4040"
worker_pool:
  workers: 8
  queue_size: 32
  generation_timeout: 120s
  max_retries: 3
  shutdown_timeout: 30s
```
