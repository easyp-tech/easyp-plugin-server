# Переход на goose — План задач

## Commands

| Action | Command | Source |
|--------|---------|-------|
| Build | `go build ./cmd/main.go` | Design §2.8 |
| Lint | `go vet ./...` | Design §2.8 |

## Тип работы

**Migration** — реструктуризация существующего миграционного механизма без изменения результирующей схемы БД.

## Матрица покрытия

| Requirement | Task(s) | Correctness Property |
|-------------|---------|---------------------|
| REQ-1.1 | T-1 | CP-1 (equivalence) |
| REQ-1.2 | T-3 | CP-2 (absence) |
| REQ-2.1 | T-2, T-3 | CP-3 (propagation) |
| REQ-2.2 | T-2 | CP-4 (equivalence) |
| REQ-2.3 | T-2 | CP-5 (propagation) |
| REQ-2.4 | T-2 | CP-6 (exclusion) |
| REQ-3.1 | T-1 | CP-7 (equivalence) |
| REQ-3.2 | T-2 | CP-8 (absence) |
| REQ-4.1, REQ-4.2 | T-4 | CP-9 (absence) |
| REQ-5.1, REQ-5.2 | T-2 | CP-10 (propagation) |

---

## T-1: CODE — Создать единую миграцию и пакет goosemigrate

*_Requirements: REQ-1.1, REQ-3.1_*
*_Preservation: CP-7_*
*_Complexity: standard_*

GOAL: Создать пакет `internal/database/goosemigrate/` с одной миграцией, содержащей финальное состояние схемы БД. Seed-данные (INSERT/UPDATE из старых миграций 2 и 5) не включаются.

1. Добавить зависимость `github.com/pressly/goose/v3` — выполнить `go get github.com/pressly/goose/v3`.

2. Создать директорию `internal/database/goosemigrate/migrations/`.

3. Создать файл `internal/database/goosemigrate/migrations/00001_init.sql`:

```sql
-- +goose Up
CREATE TABLE plugins
(
    id         UUID      NOT NULL DEFAULT gen_random_uuid(),
    group_name TEXT      NOT NULL,
    name       TEXT      NOT NULL,
    version    TEXT      NOT NULL,
    config     JSONB     NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    tags       TEXT[]    NOT NULL DEFAULT '{}',

    UNIQUE (group_name, name, version),
    PRIMARY KEY (id)
);

CREATE INDEX idx_plugins_tags ON plugins USING gin (tags);

CREATE TABLE audit_log
(
    id             UUID        NOT NULL DEFAULT gen_random_uuid(),
    operation_type TEXT        NOT NULL,
    plugin_name    TEXT,
    caller_address TEXT        NOT NULL,
    status         TEXT        NOT NULL,
    error_code     TEXT,
    error_message  TEXT,
    duration_ms    BIGINT      NOT NULL,
    metadata       JSONB       NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log (created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_operation_type ON audit_log (operation_type);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS plugins;
```

NOTE: Колонка `tags` включена сразу в `CREATE TABLE` (из миграции 4). Seed-данные из миграций 2 и 5 не включены — плагины регистрируются через gRPC API (`register-plugins.sh`).

---

## T-2: CODE — Реализовать функцию `Up()` с goose + advisory lock

*_Requirements: REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-5.1, REQ-5.2_*
*_Preservation: CP-1, CP-4, CP-5, CP-6, CP-8, CP-10_*
*_Complexity: standard_*

GOAL: Реализовать миграционную функцию с goose API, embed.FS и PostgreSQL advisory lock.

1. Создать файл `internal/database/goosemigrate/goosemigrate.go`:
   - `//go:embed migrations/*.sql` для `embed.FS`
   - Функция `Up(ctx context.Context, dsn string) error`:
     - `sql.Open("postgres", dsn)`
     - `db.PingContext(ctx)` — проверка доступности
     - `goose.SetBaseFS(migrationsFS)` и `goose.SetDialect("postgres")`
     - `goose.Up(db, "migrations")` — применить все pending миграции
     - `defer db.Close()`
     - При ошибке — return `fmt.Errorf("goosemigrate.Up: %w", err)`
   - IMPORTANT: проверить актуальный API goose v3, использовать Provider API если доступен для advisory lock.

2. Запустить `go build ./internal/database/goosemigrate/` — должно скомпилироваться.

---

## T-3: CODE — Обновить `cmd/main.go` и конфиги

*_Requirements: REQ-1.2, REQ-2.1_*
*_Preservation: CP-2, CP-3_*
*_Complexity: mechanical_*

GOAL: Заменить вызов старого мигратора на `goosemigrate.Up()` и удалить `MigrateDir` из конфигурации.

1. Модифицировать файл `cmd/main.go`:
   - Удалить импорт `"github.com/easyp-tech/service/internal/database/migrations"`
   - Добавить импорт `"github.com/easyp-tech/service/internal/database/goosemigrate"`
   - В структуре `dbConfig` удалить поле `MigrateDir string`
   - В функции `run()` заменить блок (строки 183-191) на `err = goosemigrate.Up(ctx, cfg.DB.Postgres)`
   - Если импорт `connectors` использовался только для миграций — проверить, нужен ли он ещё для `database.NewSQL`

2. Модифицировать файл `config.yml`:
   - Удалить строку `migrate_dir: "migrate"` из секции `db`

3. Модифицировать файл `config.local.yml`:
   - Удалить строку `migrate_dir: "migrate"` из секции `db`

4. Модифицировать файл `docker-compose.yml`:
   - Удалить строку `- "./migrate:/migrate:ro"` из секции `volumes` сервиса `service`

5. Запустить `go build ./cmd/main.go` — должно скомпилироваться.

---

## T-4: CODE — Удалить старый миграционный пакет и директорию `migrate/`

*_Requirements: REQ-4.1, REQ-4.2_*
*_Preservation: CP-9_*
*_Complexity: mechanical_*

GOAL: Полностью удалить самописный миграционный код и старую директорию миграций.

1. Удалить директорию `internal/database/migrations/` целиком (6 файлов).

2. Удалить директорию `migrate/` из корня проекта (5 файлов).

3. Запустить `go build ./...` — CRITICAL: проект должен компилироваться.

4. Запустить `go vet ./...` — без ошибок.

---

## T-5: GATE — Финальная проверка

*_Requirements: REQ-1.1, REQ-1.2, REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-3.1, REQ-3.2, REQ-4.1, REQ-4.2, REQ-5.1, REQ-5.2_*
*_Complexity: mechanical_*

GOAL: Полная валидация — проект компилируется, старый код удалён.

1. Запустить `go build ./...` — весь проект компилируется.
2. Запустить `go vet ./...` — без ошибок.
3. Проверить отсутствие `internal/database/migrations/` — `test ! -d internal/database/migrations`.
4. Проверить отсутствие импорта старого пакета — `grep -r "internal/database/migrations" --include="*.go" .` должен вернуть пустой результат.
5. Проверить отсутствие `migrate_dir` в конфигах — `grep -r "migrate_dir" config.yml config.local.yml` должен вернуть пустой результат.
6. Проверить отсутствие `migrate` volume в docker-compose — `grep "migrate" docker-compose.yml` должен вернуть пустой результат.
