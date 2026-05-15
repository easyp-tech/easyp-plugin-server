<!-- generated: 2026-05-15, template: development.md -->
# Tools

Reference of commands and project tools.

## 0. Dev Environment Setup

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | `brew install go` |
| Docker + Compose | latest | `brew install --cask docker` |
| Task | v3 | `brew install go-task` |
| golangci-lint | v2 | `brew install golangci-lint` |
| easyp | latest | `brew install easyp-tech/tap/easyp` |

### First Run

1. Clone: `git clone github.com/easyp-tech/service && cd service`
2. Copy env: `cp .env.example .env` — set `DB_POSTGRES_DSN`
3. Start infrastructure: `task up-minimal` (postgres + docker registry on port 5433)
4. Build and push required plugin images: `task local-push-required`
5. Run from source: `task run-local`
6. Verify: `curl http://localhost:23412/health`

## 1. Overview

All commands are run via `task` (Taskfile v3). For Go testing and linting, use standard Go tools directly.

## 2. Quick Reference

| Action | Command |
|--------|---------|
| Start full stack | `task up` |
| Start minimal stack | `task up-minimal` |
| Stop all | `task down` |
| Run from source | `task run-local` |
| Full cycle (down→up→push→logs) | `task run` |
| Push all plugins | `task local-push-registry` |
| Push required plugins only | `task local-push-required` |
| Run tests | `go test ./...` |
| Lint | `golangci-lint run ./...` |
| Generate protobuf | `easyp --cfg easyp.yaml generate` |
| Test MCP server | `task test-mcp` |
| Smoke test MCP | `task smoke-mcp` |

## 3. Detailed Command Groups

### Infrastructure

```bash
# Full 14-service dev stack (postgres, registry, grafana, loki, alloy, tempo, mimir, etc.)
task up

# Minimal: postgres (port 5433) + docker registry (port 5005) only
task up-minimal

# Stop everything, remove volumes
task down
```

### Running

```bash
# Full cycle: stop → start → push plugins → tail logs
task run

# Run from source against minimal stack (uses config.local.yml)
task run-local
# Equivalent to: go run ./cmd/main.go -cfg config.local.yml -log_level debug
```

### Plugin Images

```bash
# Build and push ALL plugin Dockerfiles from registry/
task local-push-registry
# Uses: ./push.sh localhost:5005 --push

# Build and push only required plugins (protocolbuffers/go:v1.36.10, grpc/go:v1.5.1)
task local-push-required
```

### Testing

```bash
# Unit tests
go test ./...

# With race detector
go test -race ./...

# Specific package
go test ./internal/core/...

# MCP server tests
task test-mcp
# Equivalent to: go test ./internal/mcpserver -run TestMCPServer -count=1

# MCP smoke test (requires running service)
task smoke-mcp
```

### Linting

```bash
# Run all linters
golangci-lint run ./...

# With auto-fix
golangci-lint run --fix ./...
```

## 4. Code Generation

### Protobuf

```bash
# Generate *.pb.go, *_grpc.pb.go, *.mcp.go
easyp --cfg easyp.yaml generate

# Lint proto files
easyp lint

# Validate config
easyp validate-config
```

Config: `easyp.yaml` — single file for lint rules, generation plugins, dependencies.

Generated files:
- `api/generator/v1/generator.pb.go` — protobuf types
- `api/generator/v1/generator_grpc.pb.go` — gRPC stubs
- `api/generator/v1/generator.mcp.go` — MCP tool bindings

## 5. CI/CD Cheatsheet

```bash
# Simulate CI locally:
golangci-lint run ./...          # Lint
go test -race ./...              # Tests with race detector
easyp lint                       # Proto lint
easyp --cfg easyp.yaml generate  # Ensure codegen is up to date
```

## 6. Tool Installation

| Tool | Install Command |
|------|----------------|
| Go | `brew install go` |
| Docker Desktop | `brew install --cask docker` |
| Task | `brew install go-task` |
| golangci-lint | `brew install golangci-lint` |
| easyp | `brew install easyp-tech/tap/easyp` |
| GoReleaser | `brew install goreleaser` |
