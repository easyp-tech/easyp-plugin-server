<!-- generated: 2026-05-24, template: database.md -->
# Database

PostgreSQL access patterns for EasyP Service.

## Overview

The service uses PostgreSQL for:
- Plugin metadata storage (registry)
- Audit log persistence

All database access goes through the `database.SQL` wrapper — never raw `sqlx.DB`.

## Connection

```go
db, err := database.NewSQL(ctx, cfg.DB.Driver, sqlCfg, &connectors.Raw{Query: cfg.DB.Postgres})
```

Config:
```yaml
db:
  driver: postgres
  postgres: "postgres://user:pass@localhost:5432/easyp?sslmode=disable"
  migrate_dir: migrate
```

Environment: `DB_DRIVER`, `DB_POSTGRES_DSN`, `DB_MIGRATE_DIR`

## database.SQL Wrapper

`internal/database/sql.go` wraps `*sqlx.DB` with:
- Automatic **metrics** collection per query
- Automatic **tracing** (OpenTelemetry spans)
- Connection pool management

Usage: always use `database.SQL` methods instead of raw `sqlx.DB`.

## Migrations

Location: `migrate/` directory with numbered SQL files.

Format: `NNN_description.sql` with `-- up` and `-- down` markers.

```sql
-- up
CREATE TABLE plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_name VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    version VARCHAR NOT NULL,
    ...
);

-- down
DROP TABLE IF EXISTS plugins;
```

Running:
```go
migrates, err := migrations.Parse(cfg.DB.MigrateDir)
err = migrations.Run(ctx, cfg.DB.Driver, connector, migrations.Up, migrates)
```

**Order matters** — files are sorted by numeric prefix; never reorder.

Migrations run automatically at service startup before opening the connection pool.

## Metrics

### DB Pool Collector (`internal/adapters/metrics/db_collector.go`)

Exposes `*sql.DB` pool stats as Prometheus metrics:
- `db_max_open_connections` — configured pool size
- `db_open_connections` — current open connections
- `db_in_use_connections` — connections in use
- `db_idle_connections` — idle connections
- `db_wait_count_total` — total waits for a connection
- `db_wait_duration_seconds_total` — total wait time

### Business Metrics Collector (`internal/adapters/metrics/business_collector.go`)

Queries the database for business-level metrics:
- Plugin count by group
- Total registered plugins
