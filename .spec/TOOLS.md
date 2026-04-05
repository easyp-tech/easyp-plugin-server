<!-- generated: 2026-04-03, template: development.md -->
# Tools

## Dev Environment Setup

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | `brew install go` |
| Docker | 20+ | `brew install --cask docker` |
| Task | 3+ | `brew install go-task` |
| easyp | 0.16+ | [github.com/easyp-tech/easyp](https://github.com/easyp-tech/easyp) |

### First Run

1. Clone: `git clone https://github.com/easyp-tech/service.git`
2. Start infrastructure: `task up-minimal` (Postgres + Docker registry on port 5433)
3. Build + push required plugin images: `task local-push-required`
4. Run from source: `task run-local`
5. Verify: `curl http://localhost:23412/health`

All commands are run via `task` (Taskfile.yml).

## Quick Reference

| Action | Command |
|--------|---------|
| Full dev stack (14 services) | `task up` |
| Minimal stack (Postgres + registry) | `task up-minimal` |
| Stop + clean volumes | `task down` |
| Run from source | `task run-local` |
| Full cycle (down → up → push → logs) | `task run` |
| Build + push all plugin images | `task local-push-registry` |
| Build + push required plugins only | `task local-push-required` |
| Unit tests | `go test ./...` |
| MCP integration test | `task test-mcp` |
| MCP smoke test | `task smoke-mcp` |
| Protobuf codegen | `easyp --cfg easyp.yaml generate` |
| Build binary | `go build -o bin/server ./cmd/main.go` |

## Detailed Command Groups

### Infrastructure

```bash
# Full stack: Traefik, Grafana, Mimir, Loki, Tempo, Alloy, Pyroscope, Postgres, Registry, Service
task up

# Minimal: Postgres (port 5433) + Docker Registry (port 5005)
task up-minimal

# Stop everything and remove volumes
task down

# Enterprise mode: provide license credentials
LICENSE_PUBLIC_KEY=... LICENSE_KEY=... task up
```

### Plugin Images

```bash
# Build + push ALL plugin images to local registry (uses push.sh)
task local-push-registry

# Build + push only required plugins (protocolbuffers/go + grpc/go)
task local-push-required
```

Plugin images must exist in the local registry before the service can execute code generation.

### Running

```bash
# Run from source against minimal stack
task run-local
# Equivalent to: go run ./cmd/main.go -cfg config.local.yml -log_level debug

# Full cycle: restart everything, push images, follow logs
task run
```

### Testing

```bash
# All tests
go test ./...

# MCP integration test (uses httptest.Server)
task test-mcp
# Equivalent to: go test ./internal/mcpserver -run TestMCPServer -count=1

# MCP smoke test against running service
task smoke-mcp
# Equivalent to: go run ./cmd/mcp-smoke --endpoint http://localhost:8083/mcp
```

## Code Generation

```bash
# Download proto dependencies
easyp --cfg easyp.yaml mod download

# Generate Go stubs + MCP bindings (requires running service on :8080)
easyp --cfg easyp.yaml generate
```

Generates: `api/generator/v1/*.pb.go`, `*_grpc.pb.go`, `generator.mcp.go`

## CI/CD

### Workflows

| Workflow | File | Trigger | Steps |
|----------|------|---------|-------|
| EasyP | `.github/workflows/easyp.yml` | push/PR to master | Lint protobuf + breaking change detection |
| Release | `.github/workflows/release.yml` | version tag `v*.*.*` | Build + push Docker image via goreleaser to GHCR |

### Local CI Simulation

```bash
# Lint protobuf files (same as CI)
easyp --cfg easyp.yaml lint api/

# Run all tests (same as CI would)
go test ./...
```

## Tool Installation

| Tool | Install |
|------|---------|
| Task | `brew install go-task` or `go install github.com/go-task/task/v3/cmd/task@latest` |
| easyp | `go install github.com/easyp-tech/easyp/cmd/easyp@latest` |
| Docker | `brew install --cask docker` |
