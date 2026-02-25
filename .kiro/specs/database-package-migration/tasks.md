# План реализации

- [x] 1. Написать exploration-тест условия дефекта
  - **Property 1: Fault Condition** — Пакет registry использует sqlx напрямую, минуя database.SQL
  - **CRITICAL**: Этот тест ДОЛЖЕН УПАСТЬ на неисправленном коде — падение подтверждает наличие дефекта
  - **НЕ ПЫТАЙТЕСЬ** исправлять тест или код, когда он падает
  - **NOTE**: Тест кодирует ожидаемое поведение — он подтвердит исправление, когда пройдёт после реализации
  - **GOAL**: Выявить контрпримеры, демонстрирующие архитектурный дефект
  - **Scoped PBT Approach**: Для детерминированного дефекта — проверить конкретные точки: тип поля `db` в `Registry`, возвращаемый тип `DB()`, наличие `runMigrations`/`extractUpSection`, отсутствие обёрток `NoTxContext`
  - Написать тест-файл `internal/adapters/registry/registry_migration_test.go`
  - Проверить что `Registry` хранит `*database.SQL` (а не `*sqlx.DB`) — на неисправленном коде не скомпилируется
  - Проверить что `DB()` возвращает `*database.SQL` — на неисправленном коде вернёт `*sqlx.DB`
  - Проверить что функции `runMigrations` и `extractUpSection` не экспортируются / удалены
  - Проверить что `Get`/`List` оборачивают запросы в `NoTxContext`
  - Запустить тест на НЕИСПРАВЛЕННОМ коде
  - **ОЖИДАЕМЫЙ РЕЗУЛЬТАТ**: Тест ПАДАЕТ (это корректно — доказывает наличие дефекта)
  - Задокументировать найденные контрпримеры для понимания корневой причины
  - Отметить задачу выполненной когда тест написан, запущен и падение задокументировано
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 2. Написать preservation property-тесты (ДО реализации исправления)
  - **Property 2: Preservation** — Бизнес-логика Get/List/Health/Close не изменена
  - **IMPORTANT**: Следовать observation-first методологии
  - Наблюдать: `Registry.Get` с корректными group/name/version возвращает плагин с распарсенной конфигурацией на неисправленном коде
  - Наблюдать: `Registry.Get` с version="latest" возвращает последнюю версию на неисправленном коде
  - Наблюдать: `Registry.Get` с несуществующим плагином возвращает `core.ErrNotFound` на неисправленном коде
  - Наблюдать: `Registry.List` с фильтрами (group, name, version, tags) возвращает отфильтрованный список на неисправленном коде
  - Наблюдать: `Registry.Health` проверяет доступность БД на неисправленном коде
  - Наблюдать: `Registry.Close` корректно закрывает соединение на неисправленном коде
  - Написать property-based тесты в `internal/adapters/registry/registry_preservation_test.go`, фиксирующие наблюдённое поведение
  - Property-based тестирование генерирует множество тест-кейсов для строгих гарантий сохранения поведения
  - Запустить тесты на НЕИСПРАВЛЕННОМ коде
  - **ОЖИДАЕМЫЙ РЕЗУЛЬТАТ**: Тесты ПРОХОДЯТ (подтверждают базовое поведение для сохранения)
  - Отметить задачу выполненной когда тесты написаны, запущены и проходят на неисправленном коде
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

- [x] 3. Исправление: миграция registry на пакет database

  - [x] 3.1 Обновить структуру `Registry` и удалить `Config`
    - Заменить поле `sql *sqlx.DB` на `db *database.SQL` в структуре `Registry`
    - Удалить структуру `Config` (заменяется параметрами в сигнатуре `New`)
    - Обновить импорты: добавить `internal/database`, убрать прямой `sqlx` где не нужен
    - _Bug_Condition: registryCode.usesCustomConfigStruct() AND registryCode.exportsSqlxDB()_
    - _Expected_Behavior: Registry хранит *database.SQL, Config удалён_
    - _Preservation: Все поля Registry доступны, domain парсится как прежде_
    - _Requirements: 2.4, 2.5_

  - [x] 3.2 Обновить функцию `New` — подключение и миграции
    - Изменить сигнатуру `New`: принимать `db *database.SQL`, `domain string`, `migrateDir string` вместо `Config`
    - Убрать вызов `sqlx.ConnectContext` — `*database.SQL` создаётся снаружи (в `main.go`)
    - Заменить `runMigrations` на `migrations.Parse(migrateDir)` + применение через `db.NoTxContext`
    - Удалить функции `runMigrations` и `extractUpSection`
    - _Bug_Condition: registryCode.usesDirectSqlxConnect() AND registryCode.hasCustomMigrationImplementation()_
    - _Expected_Behavior: New принимает готовый *database.SQL, миграции через migrations.Parse_
    - _Preservation: Миграции из migrate/ применяются в правильном порядке при старте_
    - _Requirements: 2.1, 2.2_

  - [x] 3.3 Обернуть SQL-запросы в `NoTxContext`
    - Обернуть `Get` — `r.db.NoTxContext(ctx, func(db *sqlx.DB) error { return db.GetContext(ctx, &dbFormat, query, args...) })`
    - Обернуть `List` — аналогично для `SelectContext`
    - Обернуть `Health` — для `PingContext`
    - Обновить `Close` — использовать `r.db.Close()`
    - Обновить `DB()` — возвращать `*database.SQL` вместо `*sqlx.DB`
    - _Bug_Condition: registryCode.callsSqlxMethodsWithoutWrapper()_
    - _Expected_Behavior: Все SQL-запросы проходят через NoTxContext с метриками и трейсингом_
    - _Preservation: Get/List/Health/Close возвращают идентичные результаты_
    - _Requirements: 2.3, 2.4_

  - [x] 3.4 Обновить `cmd/main.go` — создание `*database.SQL` и передача в `Registry`
    - Создать `*database.SQL` через `database.NewSQL(ctx, driver, sqlCfg, &connectors.Raw{Query: dsn})`
    - Передать готовый `*database.SQL` в `registry.New`
    - Обновить вызов `r.DB()` — потребители получают `*database.SQL`
    - Обновить `adapter_audit.New` — передавать `*database.SQL` или извлечь `*sqlx.DB` через обёртку
    - Обновить `adapter_metrics.NewDBCollector` — извлечь `*sql.DB` из `*database.SQL` (добавить метод или передать отдельно)
    - _Bug_Condition: main.go передаёт registry.Config с Postgres.Query_
    - _Expected_Behavior: main.go создаёт database.SQL через NewSQL с connectors.Raw_
    - _Preservation: Все потребители (audit, metrics) продолжают работать корректно_
    - _Requirements: 2.1, 2.5_

  - [x] 3.5 Обновить `internal/adapters/audit/audit.go`
    - Изменить тип поля `db` с `*sqlx.DB` на `*database.SQL`
    - Обновить `New` — принимать `*database.SQL`
    - Обернуть `Save` в `db.NoTxContext` для метрик и трейсинга
    - _Expected_Behavior: audit.Store работает через database.SQL_
    - _Preservation: Save записывает audit-логи идентично_
    - _Requirements: 2.4_

  - [x] 3.6 Обновить `internal/adapters/metrics/db_collector.go`
    - `NewDBCollector` принимает `*sql.DB` — нужно извлечь его из `*database.SQL`
    - Вариант: добавить метод `UnderlyingDB() *sql.DB` в `database.SQL`, или передавать `*sql.DB` отдельно из `main.go`
    - _Expected_Behavior: DBCollector получает *sql.DB для сбора метрик пула соединений_
    - _Preservation: Метрики пула соединений собираются как прежде_
    - _Requirements: 2.4_

  - [x] 3.7 Проверить что exploration-тест теперь проходит
    - **Property 1: Expected Behavior** — Все операции с БД проходят через database.SQL
    - **IMPORTANT**: Перезапустить ТОТ ЖЕ тест из задачи 1 — НЕ писать новый тест
    - Тест из задачи 1 кодирует ожидаемое поведение
    - Когда тест проходит — это подтверждает что ожидаемое поведение достигнуто
    - Запустить exploration-тест из шага 1
    - **ОЖИДАЕМЫЙ РЕЗУЛЬТАТ**: Тест ПРОХОДИТ (подтверждает исправление дефекта)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

  - [x] 3.8 Проверить что preservation-тесты всё ещё проходят
    - **Property 2: Preservation** — Бизнес-логика Get/List/Health/Close не изменена
    - **IMPORTANT**: Перезапустить ТЕ ЖЕ тесты из задачи 2 — НЕ писать новые тесты
    - Запустить preservation property-тесты из шага 2
    - **ОЖИДАЕМЫЙ РЕЗУЛЬТАТ**: Тесты ПРОХОДЯТ (подтверждают отсутствие регрессий)
    - Убедиться что все тесты проходят после исправления (нет регрессий)

- [x] 4. Checkpoint — Убедиться что все тесты проходят
  - Запустить полный набор тестов проекта
  - Убедиться что все тесты проходят, задать вопросы пользователю при необходимости
