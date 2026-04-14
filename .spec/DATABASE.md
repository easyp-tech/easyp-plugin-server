<!-- generated: 2026-04-14, template: database.md -->
# Database

## 1. Overview

PostgreSQL via `sqlx` (Go SQL extensions). No ORM — raw SQL with parameterized queries.

- **Driver**: `github.com/lib/pq`
- **Wrapper**: `github.com/jmoiron/sqlx`
- **Connection**: `internal/database/sql.go` (`database.SQL`)
- **Migrations**: Custom parser in `internal/database/migrations/`
- **Connection string**: `postgres://easyp_svc:easyp_pass@host:port/easyp_db?sslmode=disable`

## 2. Schema Overview

```
┌───────────────────┐
│     plugins       │
│───────────────────│
│ id (PK, uuid)     │
│ group_name (text)  │
│ name (text)        │
│ version (text)     │
│ config (jsonb)     │
│ tags (text[])      │
│ created_at (ts)    │
│                    │
│ UNIQUE(group_name, │
│   name, version)   │
└───────────────────┘

┌────────────────────┐
│    audit_log       │
│────────────────────│
│ id (PK, uuid)      │
│ operation_type     │
│ plugin_name        │
│ caller_address     │
│ status             │
│ error_code         │
│ error_message      │
│ duration_ms        │
│ metadata (jsonb)   │
│ created_at (tstz)  │
└────────────────────┘
```

**Table Reference:**

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `plugins` | Registered code generation plugins | `id`, `group_name`, `name`, `version`, `config` (JSONB Docker settings), `tags` (text array) |
| `audit_log` | Audit trail of all API operations | `id`, `operation_type`, `plugin_name`, `caller_address`, `status`, `duration_ms`, `metadata` |

## 3. Migration Strategy

- **Tool**: Custom Go parser (`internal/database/migrations/`)
- **Directory**: `migrate/`
- **Naming**: `{N}.{description}.sql` (sequential numbers)
- **Convention**: `-- up` / `-- down` markers in each file
- **Execution**: Automatic on service startup (`migrations.Run()` in `cmd/main.go`)
- **Rollback**: Supported via `-- down` sections

**Current migrations:**

| File | Description |
|------|-------------|
| `1.init.sql` | Create `plugins` table (uuid PK, unique group/name/version) |
| `2.example_plugins.sql` | Seed data: protocolbuffers/go, grpc/go, community/pseudomuto-doc, grpc-ecosystem/openapiv2, grpc-ecosystem/gateway |
| `3.audit_log.sql` | Create `audit_log` table with indexes on `created_at` and `operation_type` |
| `4.plugin_tags.sql` | Add `tags text[]` column to `plugins`, GIN index for array queries |

**Creating a new migration:**
1. Create `migrate/{N+1}.{description}.sql`
2. Add `-- up` section with SQL
3. Add `-- down` section with rollback SQL
4. Restart service (migrations run automatically)

## 4. Connection Management

```go
// internal/database/sql.go
type SQLConfig struct {
    Metrics *Metrics
}
```

**Pool settings:**
| Setting | Value |
|---------|-------|
| MaxLifetime | 60s |
| MaxIdleTime | 10s |
| MaxOpenConns | 50 |
| MaxIdleConns | 50 |

- Connection created via `database.NewSQL()` with retry logic
- Passed to adapters as `*database.SQL`
- Health check: `r.Health()` via `hellofresh/health-go`

**Metrics (pool):**
| Metric | Description |
|--------|-------------|
| `db_open_connections` | Active connections |
| `db_idle_connections` | Idle connections |
| `db_wait_count_total` | Total connection waits |
| `db_wait_duration_seconds_total` | Total wait time |

## 5. Query Patterns

### Repository Pattern

`internal/adapters/registry/registry.go` implements `core.Registry`:

```go
// Get plugin by group/name/version
func (r *Registry) Get(ctx context.Context, group, name, version string) (Plugin, error)

// List with filter using parameterized queries
func (r *Registry) List(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)

// Create with unique constraint check
func (r *Registry) Create(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error)
```

### Transaction Management

```go
// internal/database/sql.go
func (s *SQL) Tx(ctx context.Context, fn func(*sqlx.Tx) error) error {
    tx, err := s.db.BeginTxx(ctx, nil)
    // ... defer rollback on panic
    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }
    return tx.Commit()
}
```

### Tag Filtering

PostgreSQL array containment operator for tag filtering:
```sql
SELECT ... FROM plugins WHERE tags @> $1
```

## 6. Seed Data

Example plugins seeded via `migrate/2.example_plugins.sql`:

| Group | Name | Version | Docker Config |
|-------|------|---------|--------------|
| protocolbuffers | go | v1.36.10 | network=none, memory=128m, cpus=1.0, user=nobody |
| grpc | go | v1.5.1 | network=none, memory=128m, cpus=1.0, user=nobody |
| community | pseudomuto-doc | v1.5.1 | network=none, memory=256m, cpus=1.0, user=nobody |
| grpc-ecosystem | openapiv2 | v2.27.3 | network=none, memory=128m, cpus=1.0, user=nobody |
| grpc-ecosystem | gateway | v2.27.3 | network=none, memory=128m, cpus=1.0, user=nobody |

## 7. Indexes

| Table | Index | Columns | Type |
|-------|-------|---------|------|
| `plugins` | PK | `id` | B-tree |
| `plugins` | UNIQUE | `(group_name, name, version)` | B-tree |
| `plugins` | `idx_plugins_tags` | `tags` | GIN |
| `audit_log` | PK | `id` | B-tree |
| `audit_log` | `idx_audit_log_created_at` | `created_at` | B-tree |
| `audit_log` | `idx_audit_log_operation_type` | `operation_type` | B-tree |
