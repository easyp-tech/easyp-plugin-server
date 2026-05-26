# Exploration: Переход на goose

## Намерение

Заменить самописную систему миграций (`internal/database/migrations/`) на библиотеку [pressly/goose](https://github.com/pressly/goose). Мотивация — текущий мигратор функционально ограничен: нет advisory locking, нет CLI, нет `embed.FS`, парсер теряет символы новой строки при конкатенации строк, нет dirty state tracking.

## Исследование

### Текущая система миграций

**Пакет**: `internal/database/migrations/` — 6 файлов, ~450 строк кода.

| Файл | Назначение |
|------|-----------|
| [migration.go](file:///Users/zergslaw/Projects/easyp/service/internal/database/migrations/migration.go) | Тип `Migration{Version, Name, Up, Down}` + `Parse()` / `FromFS()` |
| [parser.go](file:///Users/zergslaw/Projects/easyp/service/internal/database/migrations/parser.go) | Парсер `.sql` с маркерами `-- up` / `-- down`, формат имени `N.name.sql` |
| [commands.go](file:///Users/zergslaw/Projects/easyp/service/internal/database/migrations/commands.go) | `Run()`, `upAll()`, `rollback()`, `currentVersion()`, таблица `migration` |
| [errors.go](file:///Users/zergslaw/Projects/easyp/service/internal/database/migrations/errors.go) | `ErrInvalidMigrationExt`, `ErrInvalidMigrationName` |
| [command_string.go](file:///Users/zergslaw/Projects/easyp/service/internal/database/migrations/command_string.go) | Generated `stringer` для `Command` |
| [generate.go](file:///Users/zergslaw/Projects/easyp/service/internal/database/migrations/generate.go) | `//go:generate stringer` |

**Формат миграций**: `N.name.sql` — один файл с маркерами `-- up` / `-- down`. 5 файлов в `migrate/`:
- `1.init.sql` — создание таблицы `plugins`
- `2.example_plugins.sql` — seed-данные для плагинов
- `3.audit_log.sql` — создание таблицы `audit_log` + индексы
- `4.plugin_tags.sql` — добавление колонки `tags` + GIN-индекс
- `5.disk_plugin_config.sql` — UPDATE config для перехода с Docker на local binary

**Таблица состояния**: `migration(version integer PK, time timestamp)` — создаётся автоматически через `CREATE TABLE IF NOT EXISTS`.

**Место вызова**: единственное — [cmd/main.go#L183-L191](file:///Users/zergslaw/Projects/easyp/service/cmd/main.go#L183-L191):
```go
migrates, err := migrations.Parse(cfg.DB.MigrateDir)
err = migrations.Run(ctx, cfg.DB.Driver, &connectors.Raw{Query: cfg.DB.Postgres}, migrations.Up, migrates)
```

**Тестов на миграционный пакет нет.**

**Connector** используется `connectors.Raw` — просто возвращает DSN строку как есть.

### Ключевые проблемы текущего мигратора

1. **Парсер теряет `\n`** — в [parser.go#L60-L62](file:///Users/zergslaw/Projects/easyp/service/internal/database/migrations/parser.go#L60-L62) строки конкатенируются без `\n`, что ломает многострочный SQL (напр. `CREATE TABLE` без переводов строк между полями)
2. **Нет advisory lock** — при горизонтальном масштабировании два инстанса могут запустить миграции одновременно
3. **Нет `embed.FS`** — миграции читаются с диска, что усложняет деплой single-binary
4. **Нет CLI** — разработчикам приходится управлять миграциями только через запуск сервиса
5. **Нет dirty state** — если миграция упадёт посередине, нет механизма обнаружения inconsistent state
6. **`connect()` дублирует логику** из `database.NewSQL` — retry loop при ping

### goose API (v3)

goose v3 предоставляет:
- **Go API**: `goose.Up(db, dir)`, `goose.Down(db, dir)`, `goose.Status(db, dir)`
- **embed.FS**: `goose.SetBaseFS(embedFS)` — миграции компилируются в бинарник
- **Advisory lock**: `goose.WithSessionLocker(goose.NewPostgresSessionLocker(...))` — безопасен при параллельном запуске
- **Формат**: `NNNNNN_name.sql` с маркерами `-- +goose Up` / `-- +goose Down`
- **Транзакции**: каждая миграция в транзакции по умолчанию (можно отключить через `-- +goose NO TRANSACTION`)
- **Go-миграции**: можно писать миграции на Go (для data migrations)
- **CLI**: `goose create`, `goose status`, `goose up`, `goose down`, `goose redo`
- **Таблица**: `goose_db_version(id, version_id, is_applied, tstamp)` — свой формат

## Build Tooling

- **Orchestrator:** Taskfile v3 ([Taskfile.yml](file:///Users/zergslaw/Projects/easyp/service/Taskfile.yml))
- **Test:** `go test ./...`
- **Build:** `go build ./cmd/main.go`
- **Lint:** не указан в Taskfile (предположительно через CI)
- **Generate:** `easyp generate`, `go generate ./...`
- **Source:** `Taskfile.yml`

## Рассмотренные варианты

### Вариант A: Полная замена на goose (Go API + embed.FS)

Удалить `internal/database/migrations/`, заменить на вызов goose из `cmd/main.go`.

- **Как**: `goose.SetBaseFS(migrateFS)` + `goose.Up(db, ".")` + advisory lock
- **Миграции**: конвертировать формат (`-- up` → `-- +goose Up`) и нумерацию (`1.init.sql` → `00001_init.sql`)
- **State migration**: нужно перенести данные из таблицы `migration` в `goose_db_version`
- **Плюсы**: single-binary деплой, advisory lock, транзакции, CLI бесплатно
- **Минусы**: нужна одноразовая миграция state, новая зависимость
- **Сложность**: Средняя

### Вариант B: goose с сохранением disk-based чтения (без embed.FS)

Как вариант A, но без `embed.FS` — миграции остаются на диске.

- **Плюсы**: проще миграция, формат docker-compose не меняется
- **Минусы**: теряем преимущество single-binary; всё ещё нужен `migrate/` на диске
- **Сложность**: Низкая

### Вариант C: Минимальный фикс текущего мигратора

Починить парсер (`\n`), добавить advisory lock руками, добавить `embed.FS`.

- **Плюсы**: без новых зависимостей
- **Минусы**: велосипед, нет CLI, нет community-поддержки, нет dirty state
- **Сложность**: Средняя, но ongoing maintenance cost

## Ограничения и риски

1. **State migration**: таблица `migration` использует `version integer` (1, 2, 3...), goose использует `goose_db_version(version_id bigint, is_applied boolean)`. Нужна bootstrap-миграция для переноса state. На production-базе это одноразовая операция.

2. **Формат файлов**: все 5 миграций нужно переформатировать:
   - Имена: `1.init.sql` → `00001_init.sql`
   - Маркеры: `-- up` → `-- +goose Up`, `-- down` → `-- +goose Down`

3. **Docker-compose volume**: сейчас `migrate/` монтируется в контейнер. При embed.FS это больше не нужно, но нужно убедиться, что Dockerfile правильно копирует файлы на этапе сборки.

4. **`database.Connector` интерфейс**: goose принимает `*sql.DB` напрямую — не нужен `Connector`. Но connector продолжает использоваться для `database.NewSQL`, поэтому удалять его не нужно.

5. **Backwards-compatible deploy**: при rolling update старый инстанс может не понимать goose state table. Рекомендация: деплоить миграцию state в отдельном релизе.

## Рекомендованное направление

**Вариант A: Полная замена на goose с embed.FS.**

Причины:
- goose — зрелая библиотека с активной разработкой
- `embed.FS` делает бинарник самодостаточным
- Advisory lock критически важен для production
- Минимальная поверхность изменений в коде (только `cmd/main.go` + новый пакет + конвертация файлов)
- Удаление ~450 строк самописного кода

## Границы скоупа

### Must-have (v1)
- Конвертация 5 миграций в goose-формат
- Замена вызова в `cmd/main.go` на goose API
- `embed.FS` для миграций
- Advisory lock при запуске
- Bootstrap-миграция state (таблица `migration` → `goose_db_version`)
- Удаление пакета `internal/database/migrations/`

### Deferred (v2)
- CLI-обёртка для разработчиков (`task migrate-create`, `task migrate-status`)
- Go-миграции (на данный момент все миграции — SQL)
- Интеграция goose в CI для проверки миграций

### Needs spike
- Нет

## Предположения и открытые вопросы

[ASSUMPTION: production-база уже имеет все 5 миграций применёнными] — bootstrap-миграция будет вставлять 5 записей в `goose_db_version`.

[ASSUMPTION: формат нумерации `00001_name.sql` подходит] — goose поддерживает и timestamp-based нумерацию, и sequential. Sequential ближе к текущему формату.

[ASSUMPTION: `connectors` пакет остаётся без изменений] — goose работает напрямую с `*sql.DB`, но connector по-прежнему нужен для `database.NewSQL`.

[ASSUMPTION: embed.FS предпочтительнее disk-based чтения] — для single-binary деплоя.

**Открытые вопросы:**
1. Есть ли production-база с данными, которую нужно мигрировать? Или все среды пересоздаются с нуля (docker-compose down/up)?
2. Нужно ли сохранить старую таблицу `migration` после перехода или можно дропнуть?
