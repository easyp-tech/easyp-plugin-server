# MCP Server (Model Context Protocol)

## Overview

EasyP provides an MCP server for integration with AI assistants. The server implements Streamable HTTP transport and provides tools for working with plugins and easyp configuration.

## Endpoint

```
http://localhost:8083/mcp
```

Transport: Streamable HTTP.

## Tools

### `plugins_list` — List of Plugins

Search and filter available plugins.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `group` | string | no | Filter by group |
| `name` | string | no | Filter by name |
| `version` | string | no | Filter by version |
| `tags` | string[] | no | Filter by tags |

**Response format:**

```json
{
    "total": 5,
    "plugins": [
        {
            "id": "550e8400-e29b-41d4-a716-446655440000",
            "group": "protocolbuffers",
            "name": "go",
            "version": "v1.36.10",
            "tags": ["stable", "official"],
            "created_at": "2024-01-01T00:00:00Z"
        }
    ]
}
```

**Call examples:**

All plugins:
```json
{}
```

Plugins in the `grpc` group:
```json
{
    "group": "grpc"
}
```

Plugins with the `stable` tag:
```json
{
    "tags": ["stable"]
}
```

### `easyp_config_describe` — easyp.yaml Schema Description

Helper for working with the easyp.yaml configuration file schema.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | no | Dot-path to a section (e.g., `generate.plugins[]`) |
| `include_schema` | bool | no | Include schema description |
| `include_fields` | bool | no | Include field descriptions |
| `include_examples` | bool | no | Include examples |
| `include_children` | bool | no | Include child elements |
| `examples_limit` | int | no | Examples limit (1-50) |

**Supported paths:**

| Path | Description |
|------|-------------|
| `` | Configuration root |
| `lint` | Linter settings |
| `deps` | Dependencies |
| `generate` | Generation settings |
| `generate.inputs[]` | Generation inputs |
| `generate.inputs[].directory` | Local directory |
| `generate.inputs[].git_repo` | Git repository |
| `generate.plugins[]` | Generation plugins |
| `generate.managed` | Managed settings |
| `breaking` | Backward compatibility checks |

**Response format:**

```json
{
    "schema_version": "easyp-config-v1",
    "selected_path": "generate.plugins[]",
    "schema": { ... },
    "fields": [ ... ],
    "examples": [ ... ],
    "notes": [ ... ]
}
```

## MCP Client Configuration

To connect an AI assistant to the EasyP MCP server, add the following to your client configuration:

```json
{
    "mcpServers": {
        "easyp": {
            "url": "http://localhost:8083/mcp"
        }
    }
}
```

### Examples for Various Clients

**VS Code / Kiro:**

File `.vscode/mcp.json` or `.kiro/settings/mcp.json`:
```json
{
    "mcpServers": {
        "easyp": {
            "url": "http://localhost:8083/mcp"
        }
    }
}
```

## Testing

### Unit/Integration Tests

```bash
go test ./internal/mcpserver -run TestMCPServer -count=1

# Or via Task
task test-mcp
```

### Smoke Test (Against a Running Server)

```bash
go run ./cmd/mcp-smoke --endpoint http://localhost:8083/mcp

# Or via Task
task smoke-mcp
```

The smoke test verifies:
- Connection to the MCP server
- Calling `plugins_list`
- Calling `easyp_config_describe`
- Response correctness
