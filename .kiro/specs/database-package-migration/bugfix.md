# Документ требований к исправлению (Bugfix Requirements)

## Введение

Пакет `internal/adapters/registry` напрямую использует `sqlx.ConnectContext` для подключения к БД, реализует собственный механизм миграций и предоставляет сырой `*sqlx.DB` наружу. При этом в проекте уже существует внутренний пакет `internal/database`, который предоставляет абстракцию `database.SQL` с поддержкой метрик, трейсинга (OpenTelemetry), управления пулом соединений, коннекторов (`Raw`, `PostgresDB`, `CockroachDB`) и системы миграций. Прямое использование `sqlx` обходит все эти возможности, что приводит к отсутствию наблюдаемости (observability) на уровне DAL, дублированию кода миграций и несогласованности архитектуры.

## Анализ бага

### Текущее поведение (Дефект)

1.1 WHEN `registry.New` создаёт подключение к БД THEN система использует `sqlx.ConnectContext` напрямую, минуя `internal/database.NewSQL`, и не получает метрики, трейсинг и настройки пула соединений, предоставляемые абстракцией

1.2 WHEN `registry.New` выполняет миграции THEN система использует собственную реализацию `runMigrations` с ручным парсингом SQL-файлов вместо пакета `internal/database/migrations`

1.3 WHEN `Registry` выполняет SQL-запросы (Get, List, Save) THEN система обращается к `*sqlx.DB` напрямую без обёрток `NoTx`/`NoTxContext`/`Tx`, теряя автоматический сбор метрик и трейсинг на уровне каждого вызова

1.4 WHEN `Registry.DB()` возвращает сырой `*sqlx.DB` THEN внешние потребители (audit adapter, metrics collector) получают прямой доступ к соединению, обходя абстракцию `database.SQL`

1.5 WHEN `Registry` хранит конфигурацию подключения THEN система использует собственную структуру `Config` с полем `Postgres.Query` (сырая DSN-строка) вместо типизированных коннекторов из `internal/database/connectors`

### Ожидаемое поведение (Корректное)

2.1 WHEN `registry.New` создаёт подключение к БД THEN система SHALL использовать `database.NewSQL` с коннектором из `internal/database/connectors` (например, `connectors.Raw` для DSN-строки) для получения метрик, трейсинга и управления пулом соединений

2.2 WHEN `registry.New` выполняет миграции THEN система SHALL использовать пакет `internal/database/migrations` для парсинга и применения миграций, удалив дублирующую реализацию `runMigrations` и `extractUpSection`

2.3 WHEN `Registry` выполняет SQL-запросы THEN система SHALL использовать обёртки `database.SQL.NoTxContext` или `database.SQL.Tx` для автоматического сбора метрик и трейсинга каждого вызова DAL

2.4 WHEN внешним компонентам нужен доступ к БД THEN система SHALL предоставлять `*database.SQL` вместо сырого `*sqlx.DB`, чтобы все потребители работали через единую абстракцию

2.5 WHEN `Registry` принимает конфигурацию подключения THEN система SHALL принимать `database.Connector` (интерфейс) или использовать `connectors.Raw` для DSN-строки, обеспечивая совместимость с типизированными коннекторами

### Неизменное поведение (Предотвращение регрессий)

3.1 WHEN клиент вызывает `Registry.Get` с корректными group/name/version THEN система SHALL CONTINUE TO возвращать корректный плагин с распарсенной конфигурацией

3.2 WHEN клиент вызывает `Registry.Get` с version="latest" THEN система SHALL CONTINUE TO возвращать последнюю версию плагина

3.3 WHEN клиент вызывает `Registry.Get` с несуществующим плагином THEN система SHALL CONTINUE TO возвращать ошибку `core.ErrNotFound`

3.4 WHEN клиент вызывает `Registry.List` с фильтрами (group, name, version, tags) THEN система SHALL CONTINUE TO возвращать отфильтрованный список плагинов

3.5 WHEN клиент вызывает `Registry.Health` THEN система SHALL CONTINUE TO проверять доступность БД

3.6 WHEN клиент вызывает `Registry.Close` THEN система SHALL CONTINUE TO корректно закрывать соединение с БД

3.7 WHEN плагин вызывает `Generate` THEN система SHALL CONTINUE TO выполнять Docker-контейнер с корректными параметрами и возвращать результат генерации
