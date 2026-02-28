# EasyP API Service Database

## Overview

EasyP uses PostgreSQL to store the plugin registry and audit logs. Migrations are stored in the `migrate/` directory and are applied automatically when the service starts.

## Connection Pool

| Parameter | Value |
|----------|---------|
| Max open connections | 50 |
| Max idle connections | 50 |
| Max connection lifetime | 60s |
| Max idle time | 10s |

Pool metrics are available via Prometheus (see [Monitoring](monitoring.md)).

## Migrations

Migrations are located in the `migrate/` directory and are applied in numerical order when the service starts.

### 1. `1.init.sql` — Main Schema

Plugins table:

```sql
CREATE TABLE plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_name TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE(group_name, name, version)
);
```

Each plugin is identified by a combination of `group_name/name:version` (e.g., `protocolbuffers/go:v1.36.6`).

### 2. `2.example_plugins.sql` — Initial Data

Seed data with 5 plugins:

| Group | Name | Description |
|--------|-----|----------|
| `protocolbuffers` | `go` | Go code generation from protobuf |
| `grpc` | `go` | gRPC Go code generation |
| `community` | `pseudomuto-doc` | Documentation generation |
| `grpc-ecosystem` | `openapiv2` | OpenAPI v2 spec generation |
| `grpc-ecosystem` | `gateway` | gRPC-Gateway code generation |

### 3. `3.audit_log.sql` — Audit

Audit log table:

```sql
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_type TEXT NOT NULL,
    plugin_name TEXT,
    caller_address TEXT NOT NULL,
    status TEXT NOT NULL,
    error_code TEXT,
    error_message TEXT,
    duration_ms BIGINT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX idx_audit_log_operation_type ON audit_log(operation_type);
```

### 4. `4.plugin_tags.sql` — Plugin Tags

Adding tags to plugins:

```sql
ALTER TABLE plugins ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX idx_plugins_tags ON plugins USING GIN(tags);
```

The GIN index enables efficient tag-based searching.

## Plugin Configuration JSONB Structure

The `config` field in the `plugins` table contains the plugin's Docker configuration:

```json
{
    "docker": {
        "network": "none",
        "memory": "128m",
        "cpus": "1.0",
        "user": "nobody",
        "env": {},
        "working_dir": "",
        "read_only": false,
        "tmpfs": {}
    }
}
```

| Field | Description | Default |
|------|----------|-------------|
| `network` | Docker network mode | `none` (no network) |
| `memory` | Memory limit | `128m` |
| `cpus` | CPU limit | `1.0` |
| `user` | Container user | `nobody` |
| `env` | Environment variables | `{}` |
| `working_dir` | Working directory | `""` |
| `read_only` | Read-only filesystem | `false` |
| `tmpfs` | tmpfs mount | `{}` |

## Useful Queries

### List All Plugins

```sql
SELECT group_name, name, version, tags, created_at
FROM plugins
ORDER BY group_name, name, version;
```

### Search Plugins by Tag

```sql
SELECT group_name, name, version
FROM plugins
WHERE tags @> ARRAY['stable'];
```

### Audit Statistics for the Last 24 Hours

```sql
SELECT
    operation_type,
    status,
    COUNT(*) as count,
    AVG(duration_ms) as avg_duration_ms,
    MAX(duration_ms) as max_duration_ms
FROM audit_log
WHERE created_at > now() - INTERVAL '24 hours'
GROUP BY operation_type, status
ORDER BY count DESC;
```

### Slowest Generations

```sql
SELECT
    plugin_name,
    caller_address,
    duration_ms,
    status,
    error_message,
    created_at
FROM audit_log
WHERE operation_type = 'GENERATE_CODE'
ORDER BY duration_ms DESC
LIMIT 20;
```

### Generation Errors

```sql
SELECT
    plugin_name,
    error_code,
    error_message,
    COUNT(*) as count
FROM audit_log
WHERE status = 'ERROR'
GROUP BY plugin_name, error_code, error_message
ORDER BY count DESC;
```

### Plugin Count by Group

```sql
SELECT group_name, COUNT(*) as count
FROM plugins
GROUP BY group_name
ORDER BY count DESC;
```

### Plugin Docker Configuration

```sql
SELECT
    group_name || '/' || name || ':' || version AS plugin,
    config->'docker'->>'memory' AS memory,
    config->'docker'->>'cpus' AS cpus,
    config->'docker'->>'network' AS network
FROM plugins;
```
