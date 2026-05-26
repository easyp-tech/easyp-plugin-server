# Code Review: goose-migration

## Verdict: NEEDS_CHANGES

Реализация корректно заменяет самописный мигратор на goose v3 с embed.FS. Все файлы изменены в соответствии с дизайном, проект компилируется, конфиги очищены. Однако REQ-2.4 (advisory lock для предотвращения параллельного выполнения миграций) **не реализован** — текущий код использует functional API (`goose.UpContext`), который не поддерживает advisory locking. Нужно переключиться на Provider API. Также godoc комментарий функции `Up` утверждает наличие advisory locking, что не соответствует коду.

## Change Set

| Файл | Статус | Notes |
|------|--------|-------|
| `internal/database/goosemigrate/goosemigrate.go` | ✅ Planned (NEW) | — |
| `internal/database/goosemigrate/migrations/00001_init.sql` | ✅ Planned (NEW) | — |
| `cmd/main.go` | ✅ Planned (MODIFIED) | Импорт, dbConfig, вызов миграций |
| `config.yml` | ✅ Planned (MODIFIED) | Удалён `migrate_dir` |
| `config.local.yml` | ✅ Planned (MODIFIED) | Удалён `migrate_dir` |
| `docker-compose.yml` | ✅ Planned (MODIFIED) | Удалён volume mount |
| `internal/database/migrations/` (6 файлов) | ✅ Planned (DELETED) | — |
| `migrate/` (5 файлов) | ✅ Planned (DELETED) | — |
| `go.mod`, `go.sum` | ⚠️ Unexpected | Обоснованно — добавлена зависимость goose v3 |

## Requirements Traceability

| Requirement | Test(s) | Code | CP | Verdict |
|-------------|---------|------|----|---------|
| REQ-1.1 | (без тестов) | `goosemigrate.go:13-14` embed.FS | CP-1 | ✅ |
| REQ-1.2 | (без тестов) | `cmd/main.go:71-74` нет `MigrateDir` | CP-2 | ✅ |
| REQ-2.1 | (без тестов) | `cmd/main.go:182` вызов до `NewSQL` | CP-3 | ✅ |
| REQ-2.2 | (без тестов) | `goosemigrate.go:36` goose.UpContext — идемпотентен | CP-4 | ✅ |
| REQ-2.3 | (без тестов) | `goosemigrate.go:36-38` error propagation | CP-5 | ✅ |
| REQ-2.4 | (без тестов) | **Не реализован** — advisory lock отсутствует | CP-6 | ❌ |
| REQ-3.1 | (без тестов) | `00001_init.sql:1` goose format | CP-7 | ✅ |
| REQ-3.2 | (без тестов) | goose default behavior — транзакция per migration | CP-8 | ✅ |
| REQ-4.1 | (без тестов) | `internal/database/migrations/` удалён | CP-9 | ✅ |
| REQ-4.2 | (без тестов) | grep подтверждает отсутствие импорта | CP-9 | ✅ |
| REQ-5.1 | (без тестов) | goose создаёт `goose_db_version` автоматически | CP-10 | ✅ |
| REQ-5.2 | (без тестов) | Старая таблица не используется | CP-10 | ✅ |

NOTE: тесты исключены из scope по решению пользователя.

## Design Conformance

### 3.1 Архитектурные границы
✅ Пакет `internal/database/goosemigrate/` создан в правильном слое. Зависимость однонаправленная: `cmd/main.go` → `goosemigrate`.

### 3.2 Модели данных
✅ SQL-схема в `00001_init.sql` полностью соответствует финальному состоянию из 5 оригинальных миграций. Seed-данные корректно удалены.

### 3.3 API Contracts
✅ Сигнатура `Up(ctx context.Context, dsn string) error` соответствует дизайну §2.3.

### 3.4 Error Handling
✅ Ошибки обёрнуты с контекстом через `fmt.Errorf("goosemigrate: ...: %w", err)`. Все 3 точки ошибок (Open, Ping, Up) покрыты.

### 3.5 Correctness Properties
- CP-1 (embed.FS equivalence): ✅
- CP-2 (absence MigrateDir): ✅
- CP-3 (propagation — миграции до pool): ✅
- CP-4 (idempotent Up): ✅
- CP-5 (error stops startup): ✅
- CP-6 (advisory lock): ❌ **Не реализован**
- CP-7 (goose file format): ✅
- CP-8 (transaction rollback): ✅
- CP-9 (no old code): ✅
- CP-10 (goose_db_version): ✅

### 3.6 Документация
Godoc комментарий функции `Up` заявляет "It uses PostgreSQL advisory locking" — это не соответствует реализации (F-1).

## Code Quality

### 4.1 Naming & Clarity
✅ Именование соответствует конвенциям проекта. Пакет `goosemigrate` — описательный.

### 4.2 Dead Code
✅ Нет мёртвого кода. `connectors` import остался в `cmd/main.go` — он по-прежнему используется для `database.NewSQL`.

### 4.3 Scope Creep
✅ Нет изменений вне scope.

### 4.4 Test Quality
N/A — тесты исключены из scope по решению пользователя.

## Security

Нет новых эндпоинтов, нет изменений в обработке пользовательского ввода. DSN передаётся из конфигурации — не от пользователя. SQL-миграции статически вкомпилированы через embed.FS — нет риска инъекции. Нет хардкоженных секретов.

**Замечание:** `goose.SetBaseFS()` и `goose.SetDialect()` модифицируют глобальное состояние пакета goose. Это безопасно при однократном вызове при старте, но может быть проблемой при параллельных тестах. Для текущего scope это не критично (F-2, minor).

## Verification Evidence

- **Build:**
```
$ go build -buildvcs=false ./...
(exit 0, no output)
```
- **Lint:**
```
$ go vet ./...
(exit 0, no output)
```

## Findings

| ID | Severity | File | Description | Requirement |
|----|----------|------|-------------|-------------|
| F-1 | major | `goosemigrate.go:17,30-34` | Advisory lock не реализован. Functional API (`goose.UpContext`) не поддерживает advisory locking. Нужно использовать Provider API с `goose.WithSessionLocker(lock.NewPostgresSessionLocker())`. Godoc также вводит в заблуждение. | REQ-2.4 |
| F-2 | minor | `goosemigrate.go:30-34` | `goose.SetBaseFS()` и `goose.SetDialect()` устанавливают глобальное состояние. Переход на Provider API (для F-1) решит эту проблему, т.к. Provider инкапсулирует конфигурацию. | — |

## Recommendations

1. **F-1 (major):** Переписать `goosemigrate.go` с использованием Provider API:
   ```go
   locker, err := lock.NewPostgresSessionLocker()
   provider, err := goose.NewProvider("postgres", db, migrationsFS,
       goose.WithSessionLocker(locker),
   )
   _, err = provider.Up(ctx)
   ```
   Это решит и F-1 (advisory lock), и F-2 (глобальное состояние).

2. **F-2 (minor):** Решается автоматически при фиксе F-1.
