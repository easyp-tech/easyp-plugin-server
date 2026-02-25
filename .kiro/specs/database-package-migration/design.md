# Миграция на пакет database — Дизайн исправления

## Обзор

Пакет `internal/adapters/registry` напрямую использует `sqlx` для подключения к БД, реализует собственный механизм миграций и экспортирует сырой `*sqlx.DB`. В проекте уже есть внутренний пакет `internal/database`, предоставляющий абстракцию `database.SQL` с метриками, трейсингом (OpenTelemetry), управлением пулом соединений и системой миграций. Исправление заключается в замене прямого использования `sqlx` на `database.NewSQL`, замене кастомных миграций на `database/migrations.Parse`, оборачивании SQL-запросов в `NoTxContext`/`Tx` и изменении публичного API для экспорта `*database.SQL` вместо `*sqlx.DB`.

## Глоссарий

- **Bug_Condition (C)**: Условие дефекта — использование прямого `sqlx.ConnectContext`, кастомных миграций, необёрнутых SQL-запросов и экспорт сырого `*sqlx.DB` в пакете `registry`
- **Property (P)**: Желаемое поведение — все операции с БД проходят через абстракцию `database.SQL`, обеспечивая метрики, трейсинг и единообразие
- **Preservation**: Существующая бизнес-логика (Get, List, Health, Close, Generate) должна работать идентично после рефакторинга
- **`database.SQL`**: Обёртка над `*sqlx.DB` в `internal/database/sql.go`, предоставляющая `NoTx`, `NoTxContext`, `Tx` с автоматическими метриками и трейсингом
- **`database.Connector`**: Интерфейс в `internal/database/sql.go` с методом `DSN() (string, error)` для получения строки подключения
- **`connectors.Raw`**: Реализация `Connector` в `internal/database/connectors/raw.go`, принимающая готовую DSN-строку
- **`migrations.Parse`**: Функция в `internal/database/migrations/migration.go`, парсящая SQL-файлы с маркерами `-- up` / `-- down`

## Детали бага

### Fault Condition

Баг проявляется при любом использовании пакета `registry` — все операции с БД обходят абстракцию `database.SQL`. Это не runtime-ошибка, а архитектурный дефект: отсутствие метрик, трейсинга и единообразия на уровне DAL.

**Формальная спецификация:**
```
FUNCTION isBugCondition(registryCode)
  INPUT: registryCode — исходный код пакета registry
  OUTPUT: boolean

  RETURN registryCode.usesDirectSqlxConnect()
         OR registryCode.hasCustomMigrationImplementation()
         OR registryCode.callsSqlxMethodsWithoutWrapper()
         OR registryCode.exportsSqlxDB()
         OR registryCode.usesCustomConfigStruct()
END FUNCTION
```

### Примеры

- `registry.New` вызывает `sqlx.ConnectContext(ctx, cfg.Driver, cfg.Postgres.Query)` — ожидается `database.NewSQL(ctx, cfg.Driver, sqlCfg, connector)`
- `Registry.Get` вызывает `r.sql.GetContext(ctx, &dbFormat, query, args...)` напрямую — ожидается `r.db.NoTxContext(ctx, func(db *sqlx.DB) error { return db.GetContext(...) })`
- `Registry.DB()` возвращает `*sqlx.DB` — ожидается возврат `*database.SQL`
- `runMigrations` вручную читает и парсит SQL-файлы — ожидается `migrations.Parse(dir)` с последующим применением через `db.ExecContext`
- `main.go` передаёт `registry.Config{Postgres: struct{Query string}{...}}` — ожидается передача `database.Connector` (например, `&connectors.Raw{Query: dsn}`)

## Ожидаемое поведение

### Требования к сохранению (Preservation Requirements)

**Неизменное поведение:**
- `Registry.Get` с корректными group/name/version возвращает плагин с распарсенной конфигурацией
- `Registry.Get` с version="latest" возвращает последнюю версию
- `Registry.Get` с несуществующим плагином возвращает `core.ErrNotFound`
- `Registry.List` с фильтрами возвращает отфильтрованный список
- `Registry.Health` проверяет доступность БД
- `Registry.Close` корректно закрывает соединение
- `plugin.Generate` выполняет Docker-контейнер и возвращает результат
- Миграции из директории `migrate/` применяются в правильном порядке при старте

**Область:**
Все SQL-запросы, бизнес-логика и внешнее поведение API не должны измениться. Изменяется только инфраструктурный слой: способ подключения, оборачивание запросов и экспортируемые типы.

## Гипотеза о корневой причине

Дефект является архитектурным — пакет `registry` был написан до появления (или без учёта) внутреннего пакета `internal/database`. Основные причины:

1. **Прямое подключение через sqlx**: `registry.New` использует `sqlx.ConnectContext` вместо `database.NewSQL`, что обходит настройку пула соединений (`SetConnMaxLifetime`, `SetMaxOpenConns` и т.д.), метрики и трейсинг

2. **Кастомная реализация миграций**: Функции `runMigrations` и `extractUpSection` дублируют логику `internal/database/migrations`, но менее надёжно (нет версионирования, нет поддержки down-миграций)

3. **Отсутствие обёрток DAL**: Методы `Get`, `List`, `Health` обращаются к `*sqlx.DB` напрямую, минуя `NoTxContext`/`Tx`, которые обеспечивают автоматический сбор метрик по имени метода и трейсинг через OpenTelemetry

4. **Экспорт сырого типа**: `DB() *sqlx.DB` заставляет потребителей (`audit.Store`, `metrics.DBCollector`) работать с низкоуровневым типом, что делает невозможным использование обёрток `database.SQL` в этих адаптерах

5. **Собственная структура Config**: `registry.Config` с полем `Postgres.Query` не использует типизированные коннекторы, что затрудняет переключение между типами БД (PostgreSQL, CockroachDB)

## Correctness Properties

Property 1: Fault Condition — Все операции с БД проходят через database.SQL

_For any_ вызов `registry.New`, результирующий `Registry` SHALL использовать `*database.SQL` для всех операций с БД, включая подключение через `database.NewSQL`, миграции через `database/migrations`, и оборачивание запросов в `NoTxContext`/`Tx`.

**Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5**

Property 2: Preservation — Бизнес-логика запросов не изменена

_For any_ вызов `Registry.Get`, `Registry.List`, `Registry.Health`, `Registry.Close` с теми же входными данными, исправленная версия SHALL возвращать идентичные результаты (те же данные, те же ошибки), что и оригинальная версия, сохраняя всю существующую функциональность.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7**

## Реализация исправления

### Необходимые изменения

Исходя из анализа корневой причины:

**Файл**: `internal/adapters/registry/registry.go`

**Изменения структуры `Registry`:**
1. **Заменить поле `sql *sqlx.DB` на `db *database.SQL`** — центральное изменение, от которого зависят все остальные
2. **Удалить структуру `Config`** — заменить параметрами `database.Connector` и `string` (domain) в сигнатуре `New`

**Изменения функции `New`:**
3. **Заменить `sqlx.ConnectContext` на `database.NewSQL`** — передать `driver`, `database.SQLConfig` и `connector` (например, `connectors.Raw`)
4. **Заменить `runMigrations` на `migrations.Parse`** — распарсить миграции через `migrations.Parse(migrateDir)`, затем применить через `db.NoTxContext` или напрямую через `sqlx.DB` внутри обёртки
5. **Обновить сигнатуру `New`** — принимать `*database.SQL` (уже созданный снаружи) или `database.Connector` + `database.SQLConfig`

**Изменения методов:**
6. **Обернуть `Get` в `NoTxContext`** — `r.db.NoTxContext(ctx, func(db *sqlx.DB) error { return db.GetContext(ctx, &dbFormat, query, args...) })`
7. **Обернуть `List` в `NoTxContext`** — аналогично для `SelectContext`
8. **Обернуть `Health` в `NoTxContext`** — для `PingContext`
9. **Заменить `DB() *sqlx.DB` на `DB() *database.SQL`** — изменить возвращаемый тип

**Удалить:**
10. **Удалить `runMigrations`** — кастомная реализация больше не нужна
11. **Удалить `extractUpSection`** — вспомогательная функция для кастомных миграций

**Файл**: `cmd/main.go`

12. **Обновить создание `Registry`** — передавать `database.Connector` (например, `&connectors.Raw{Query: cfg.DB.Postgres}`) вместо `registry.Config`
13. **Создавать `*database.SQL` в `main.go`** — вызывать `database.NewSQL` до создания `Registry`, передавать готовый `*database.SQL` в `registry.New`
14. **Обновить вызов `r.DB()`** — потребители (`audit.New`, `metrics.NewDBCollector`) должны принимать `*database.SQL` или извлекать `*sql.DB` из него

**Файл**: `internal/adapters/audit/audit.go`

15. **Обновить тип поля `db`** — с `*sqlx.DB` на `*database.SQL` (или оставить `*sqlx.DB`, если audit будет получать его через обёртку)

**Файл**: `internal/adapters/metrics/db_collector.go`

16. **Обновить получение `*sql.DB`** — `metrics.NewDBCollector` принимает `*sql.DB`, нужно извлечь его из `*database.SQL`

## Стратегия тестирования

### Подход к валидации

Стратегия тестирования следует двухфазному подходу: сначала выявить контрпримеры, демонстрирующие дефект на неисправленном коде, затем проверить корректность исправления и сохранение существующего поведения.

### Exploratory Fault Condition Checking

**Цель**: Выявить контрпримеры, демонстрирующие дефект ДО реализации исправления. Подтвердить или опровергнуть анализ корневой причины.

**План тестирования**: Написать тесты, проверяющие что `Registry` использует `database.SQL` и его обёртки. Запустить на неисправленном коде для наблюдения ошибок.

**Тест-кейсы**:
1. **Проверка типа подключения**: Убедиться, что `Registry` хранит `*database.SQL`, а не `*sqlx.DB` (не скомпилируется на неисправленном коде)
2. **Проверка обёрток**: Убедиться, что `Get`/`List` вызывают `NoTxContext` (не будет вызываться на неисправленном коде)
3. **Проверка экспорта**: Убедиться, что `DB()` возвращает `*database.SQL` (вернёт `*sqlx.DB` на неисправленном коде)
4. **Проверка миграций**: Убедиться, что `runMigrations` и `extractUpSection` удалены (существуют на неисправленном коде)

**Ожидаемые контрпримеры**:
- Код не компилируется с новыми типами на неисправленной версии
- Обёртки `NoTxContext` не вызываются при SQL-запросах

### Fix Checking

**Цель**: Проверить, что для всех входных данных, где условие дефекта выполняется, исправленная функция работает корректно.

**Псевдокод:**
```
FOR ALL input WHERE isBugCondition(registryCode) DO
  result := registryFixed.Get(input) // или List, Health, Close
  ASSERT result == registryOriginal.Get(input) // идентичный результат
  ASSERT metricsCollected(result)               // метрики собраны
  ASSERT traceSpanCreated(result)               // трейс создан
END FOR
```

### Preservation Checking

**Цель**: Проверить, что для всех входных данных, где условие дефекта НЕ выполняется, исправленная функция возвращает тот же результат.

**Псевдокод:**
```
FOR ALL input WHERE NOT isBugCondition(input) DO
  ASSERT registryOriginal.Get(input) == registryFixed.Get(input)
  ASSERT registryOriginal.List(input) == registryFixed.List(input)
  ASSERT registryOriginal.Health() == registryFixed.Health()
END FOR
```

**Подход к тестированию**: Property-based тестирование рекомендуется для preservation checking, так как:
- Автоматически генерирует множество тест-кейсов по всему домену входных данных
- Ловит граничные случаи, которые ручные unit-тесты могут пропустить
- Даёт строгие гарантии неизменности поведения для всех не-дефектных входов

**План тестирования**: Наблюдать поведение на неисправленном коде для всех SQL-запросов, затем написать property-based тесты, фиксирующие это поведение.

**Тест-кейсы**:
1. **Preservation Get**: Проверить, что `Get` возвращает те же данные и ошибки после рефакторинга
2. **Preservation List**: Проверить, что `List` с фильтрами возвращает идентичные результаты
3. **Preservation Health**: Проверить, что `Health` корректно проверяет доступность БД
4. **Preservation Close**: Проверить, что `Close` корректно закрывает соединение

### Unit Tests

- Проверка что `registry.New` создаёт `Registry` с `*database.SQL` внутри
- Проверка что `Get` оборачивает запрос в `NoTxContext`
- Проверка что `List` оборачивает запрос в `NoTxContext`
- Проверка что `DB()` возвращает `*database.SQL`
- Проверка что `runMigrations` и `extractUpSection` удалены из кода
- Проверка что `Config` struct удалена

### Property-Based Tests

- Генерация случайных group/name/version и проверка что `Get` возвращает корректные результаты через `database.SQL`
- Генерация случайных фильтров и проверка что `List` возвращает идентичные результаты
- Генерация случайных конфигураций плагинов и проверка сохранения парсинга JSON-конфигурации

### Integration Tests

- Полный цикл: создание `Registry` через `database.NewSQL` → применение миграций → вставка плагина → получение через `Get` → проверка результата
- Проверка что метрики собираются при вызовах `Get`/`List` через `NoTxContext`
- Проверка что потребители (`audit.Store`, `metrics.DBCollector`) корректно работают с новым типом `DB()`
