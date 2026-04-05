<!-- generated: 2026-04-04, template: database.md -->
# Database

## Overview

PostgreSQL 17.7 via `sqlx` + `lib/pq`. All access goes through the `database.SQL` wrapper which provides metrics, tracing, and transaction management.

## Connection Pool Defaults

| Parameter | Default | Config field |
|-----------|---------|-------------|
| MaxOpenConns | 50 | `SetMaxOpenConnections` |
| MaxIdleConns | 50 | `SetMaxIdleConnections` |
| ConnMaxLifetime | 60s | `SetConnMaxLifetime` |
| ConnMaxIdleTime | 10s | `SetConnMaxIdleTime` |

DSN: `postgres://easyp_svc:easyp_pass@postgres:5432/easyp_db?sslmode=disable`

## Schema

### `plugins` (migration 1 + 4)

```sql
CREATE TABLE plugins (
    id         UUID      NOT NULL DEFAULT gen_random_uuid(),
    group_name TEXT      NOT NULL,
    name       TEXT      NOT NULL,
    version    TEXT      NOT NULL,
    config     JSONB     NOT NULL DEFAULT '{}',
    tags       TEXT[]    NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (group_name, name, version),
    PRIMARY KEY (id)
);
CREATE INDEX idx_plugins_tags ON plugins USING gin (tags);
```

The `config` JSONB stores Docker execution parameters:
```json
{"docker": {"network": "none", "memory": "128m", "cpus": "1.0", "user": "nobody"}}
```

### `audit_log` (migration 3)

```sql
CREATE TABLE audit_log (
    id              UUID        NOT NULL DEFAULT gen_random_uuid(),
    operation_type  TEXT        NOT NULL,
    plugin_name     TEXT,
    caller_address  TEXT        NOT NULL,
    status          TEXT        NOT NULL,  -- 'success' | 'error'
    error_code      TEXT,
    error_message   TEXT,
    duration_ms     BIGINT      NOT NULL,
    metadata        JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);
CREATE INDEX idx_audit_log_created_at ON audit_log (created_at);
CREATE INDEX idx_audit_log_operation_type ON audit_log (operation_type);
```

## Migrations

Files in `migrate/` directory, sorted by numeric prefix:

| # | File | Description |
|---|------|-------------|
| 1 | `1.init.sql` | Creates `plugins` table |
| 2 | `2.example_plugins.sql` | Inserts seed plugins (protocolbuffers/go, grpc/go, etc.) |
| 3 | `3.audit_log.sql` | Creates `audit_log` table with indexes |
| 4 | `4.plugin_tags.sql` | Adds `tags TEXT[]` column + GIN index to `plugins` |

Format: `-- up` / `-- down` sections. Never reorder or renumber existing files.

## SQL Wrapper (`internal/database/sql.go`)

Three access patterns:

```go
// No transaction, no tracing
db.NoTx(func(db *sqlx.DB) error { ... })

// Transaction with tracing
db.Tx(ctx, nil, func(tx *sqlx.Tx) error { ... })

// No transaction, with tracing
db.NoTxContext(ctx, func(db *sqlx.DB) error { ... })
```

All patterns automatically:
- Wrap errors with caller method name
- Collect duration and error metrics via `MetricCollector`
- Create OTel spans (for `Tx` and `NoTxContext`)
- Handle panic recovery in transactions (`Tx` rolls back on panic)

## DAL Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `{namespace}_{subsystem}_errors_total` | Counter | `func` |
| `{namespace}_{subsystem}_call_duration_seconds` | Histogram | `func` |

Registered via `database.NewMetrics()`. Method names extracted automatically from DAL struct.

## Adding a New Table

1. Create `migrate/{next_number}.description.sql` with `-- up` and `-- down` sections
2. Write adapter in `internal/adapters/` implementing a core interface
3. Use `db.Tx()` or `db.NoTxContext()` for data access
4. Register DAL metrics via `database.NewMetrics()`
