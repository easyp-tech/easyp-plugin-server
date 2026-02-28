# EasyP API Service Development

## Requirements

- **Go** 1.26+
- **Docker** and **Docker Compose**
- **Task** (optional, for convenient commands) — [taskfile.dev](https://taskfile.dev)

## Quick Start

```bash
# 1. Start the infrastructure
task up

# 2. Build and push plugins to the local registry
task local-push-registry

# The service starts automatically in docker-compose
```

After startup:
- gRPC API: `localhost:8080`
- Metrics: `localhost:8081/metrics`
- Health check: `localhost:8082/health`
- MCP server: `localhost:8083/mcp`
- Grafana: `localhost:3000` (admin/admin)

## Task Commands

| Command | Description |
|---------|-------------|
| `task up` | Start all services |
| `task down` | Stop and clean up (including volumes) |
| `task run` | Full cycle: down → up → push plugins → logs |
| `task local-push-registry` | Build and push plugin images |
| `task test-mcp` | Run MCP integration tests |
| `task smoke-mcp` | Smoke tests for MCP against a running server |

## Building

```bash
# Build the binary
go build -o bin/server ./cmd/main.go

# Run with configuration
./bin/server -cfg config.yml -log_level debug
```

## Project Structure

```
.
├── cmd/
│   ├── main.go              # Service entry point
│   └── mcp-smoke/           # MCP server smoke test
├── internal/
│   ├── api/                 # gRPC transport layer (server, interceptors)
│   ├── core/                # Business logic (Core, WorkerPool)
│   │   └── pool.go          # Worker pool with backpressure and retry
│   ├── adapters/
│   │   ├── registry/        # Plugin registry adapter (PostgreSQL + Docker)
│   │   ├── audit/           # Asynchronous audit system
│   │   └── metrics/         # Prometheus metrics
│   ├── database/            # SQL wrapper over sqlx (tracing, pool, transactions)
│   ├── mcpserver/           # MCP server (Streamable HTTP)
│   └── telemetry/           # Telemetry (OTEL, Pyroscope, tracing decorators)
├── api/
│   └── generator/v1/        # Protobuf definitions and generated Go code
├── docs/
│   └── api/                 # Auto-generated API documentation
├── migrate/                 # SQL migrations
├── registry/                # Plugin Dockerfiles
│   └── {group}/{name}/{version}/
├── configs/
│   ├── alloy/               # Alloy configuration (telemetry collector)
│   ├── grafana/             # Grafana configuration (datasources, dashboards)
│   ├── mimir/               # Mimir configuration
│   ├── loki/                # Loki configuration
│   ├── tempo/               # Tempo configuration
│   ├── pyroscope/           # Pyroscope configuration
│   └── traefik/             # Traefik configuration
├── config.yml               # Service configuration
├── docker-compose.yml       # Docker Compose for development
├── Dockerfile               # Multi-stage service build
├── easyp.yaml               # easyp configuration (code generation)
├── push.sh                  # Plugin build and push script
├── Taskfile.yml             # Task commands
└── .goreleaser.yaml         # GoReleaser configuration
```

## Adding a New Plugin

### 1. Create the plugin directory

```bash
mkdir -p registry/{group}/{name}/{version}
```

For example:
```bash
mkdir -p registry/myorg/mycodegen/v1.0.0
```

### 2. Create a Dockerfile

The plugin must:
- Read protobuf data from **stdin**
- Write the result to **stdout**
- Run as a **non-privileged user**

```dockerfile
FROM alpine:3.22
# Install plugin dependencies
COPY mycodegen /usr/local/bin/mycodegen
USER nobody
ENTRYPOINT ["mycodegen"]
```

### 3. Add a migration

Create an SQL migration in `migrate/`:

```sql
INSERT INTO plugins (group_name, name, version, config, tags)
VALUES (
    'myorg',
    'mycodegen',
    'v1.0.0',
    '{"docker": {"network": "none", "memory": "128m", "cpus": "1.0", "user": "nobody"}}',
    ARRAY['stable']
);
```

### 4. Build and push

```bash
./push.sh localhost:5005 --push
# Or
task local-push-registry
```

## Testing

### Unit Tests

```bash
go test ./internal/core/...
```

### MCP Integration Tests

```bash
go test ./internal/mcpserver -run TestMCPServer -count=1
# Or
task test-mcp
```

### MCP Smoke Tests

Require a running server:

```bash
go run ./cmd/mcp-smoke --endpoint http://localhost:8083/mcp
# Or
task smoke-mcp
```

## Code Generation (Protobuf)

```bash
easyp generate
```

Uses the `easyp.yaml` configuration with remote plugins on `localhost:8080`.

## Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| gRPC | v1.79.1 | RPC framework |
| protobuf | v1.36.11 | Serialization |
| OpenTelemetry | v1.40.0 | Tracing and metrics |
| sqlx | v1.4.0 | SQL driver |
| Prometheus client | v1.23.2 | Metrics export |
| MCP SDK | v1.3.1 | Model Context Protocol |
| Pyroscope | v1.2.7 | Continuous profiling |

## Release

Releases are built via GoReleaser:

- Multi-architecture Docker images (amd64, arm64)
- Published to `ghcr.io/easyp-tech/service`
- Tags: `{version}-amd64`, `{version}-arm64`, `{version}` (manifest), `latest`

Configuration in `.goreleaser.yaml`.
