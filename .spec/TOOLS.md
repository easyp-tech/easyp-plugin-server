<!-- generated: 2026-04-14, template: development.md -->
# Tools & Commands

## 0. Dev Environment Setup

**Prerequisites:**

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | `brew install go` |
| Docker | 20+ | `brew install --cask docker` |
| Task | 3+ | `brew install go-task` |
| EasyP | latest | `brew install easyp-tech/tap/easyp` |

**First Run:**

1. Clone: `git clone https://github.com/easyp-tech/service.git && cd service`
2. Start infrastructure: `task up-minimal`
3. Build + push required plugin images: `task local-push-required`
4. Run locally: `task run-local`
5. Verify: `curl http://localhost:23412/health`

## 1. Overview

All commands run via `task` (Taskfile.yml). Docker is required for plugin execution and infrastructure.

## 2. Quick Reference

| Action | Command |
|--------|---------|
| Start full stack | `task up` |
| Start minimal (Postgres + Registry) | `task up-minimal` |
| Stop and clean | `task down` |
| Run locally | `task run-local` |
| Full cycle | `task run` |
| Push plugin images | `task local-push-required` |
| Push all registry images | `task local-push-registry` |
| Unit tests | `go test ./...` |
| Protobuf codegen | `easyp --cfg easyp.yaml generate` |
| Protobuf lint | `easyp lint` |
| MCP smoke test | `task smoke-mcp` |
| MCP unit test | `task test-mcp` |

## 3. Detailed Command Groups

### Infrastructure

```bash
# Start all 14 services (Postgres, Registry, Grafana, Tempo, Loki, etc.)
task up

# Minimal stack: only Postgres (port 5433) + Docker Registry (port 5005)
task up-minimal

# Stop everything and remove volumes
task down

# Full cycle: down → up → push images → follow logs
task run
```

### Running Locally

```bash
# Run service from source against minimal stack
task run-local
# Equivalent to: go run ./cmd/main.go -cfg config.local.yml -log_level debug
```

### Plugin Images

```bash
# Build and push required plugins (protoc-gen-go, protoc-gen-go-grpc)
task local-push-required

# Build and push ALL registry plugins
task local-push-registry
```

### Testing

```bash
# All tests
go test ./...

# Specific package
go test ./internal/core/...

# With race detector
go test -race ./...

# MCP server tests
task test-mcp

# MCP smoke test (requires running service)
task smoke-mcp
```

## 4. Code Generation

### Protobuf

```bash
# Generate Go stubs + MCP bindings from generator.proto
easyp --cfg easyp.yaml generate
```

**Config:** `easyp.yaml`
**Source:** `api/generator/v1/generator.proto`
**Output:** `api/generator/v1/generator.pb.go`, `generator_grpc.pb.go`, `generator.mcp.go`

**Requirement:** Running service at localhost:8080 (easyp calls the service's own gRPC endpoint for codegen).

### Database Connectors

```bash
# Regenerate SSL enum stringers
go generate ./internal/database/connectors/...
```

## 5. CI/CD Cheatsheet

```bash
# Simulate full CI locally
go test -race -count=1 ./...
easyp lint
go vet ./...
```

## 6. Tool Installation

| Tool | Install Command |
|------|-----------------|
| Go | `brew install go` |
| Docker | `brew install --cask docker` |
| Task | `brew install go-task` |
| EasyP | `brew install easyp-tech/tap/easyp` or `go install github.com/easyp-tech/easyp/cmd/easyp@latest` |
