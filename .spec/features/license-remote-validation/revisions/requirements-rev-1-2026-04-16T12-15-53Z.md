# Remote License Validation — Требования

**Статус:** Draft  
**Автор:** agent  
**Дата:** 2026-04-16

## Обзор

Сервис переходит с локальной валидации PASETO-токенов (Ed25519-подпись, публичный ключ через `-ldflags`) на удалённую валидацию через gRPC-вызов к внешнему лицензионному серверу. Сервис отправляет лицензионный ключ, ID инсталляции и версию сервиса; сервер возвращает Claims (тир, фичи, лимиты). Claims кэшируются локально и периодически обновляются. При недоступности сервера используется мягкий fallback на Community mode. PASETO и `-ldflags`-инжекция удаляются полностью, механика сборки упрощается.

---

## Глоссарий

| Термин | Определение | Code Artifact |
|--------|-------------|---------------|
| `LicenseClient` | Интерфейс gRPC-клиента к лицензионному серверу | `internal/license/client.go` (новый) |
| `MockLicenseClient` | Мок-реализация `LicenseClient` для старта без реального сервера | `internal/license/client.go` (новый) |
| `Manager` | Компонент кэширования и обновления Claims | `internal/license/manager.go` |
| `Claims` | Структура данных лицензии: тир, фичи, лимиты | `internal/license/claims.go` |
| `CacheTTL` | Время жизни локального кэша Claims между обращениями к серверу | `internal/license/manager.go` |
| `InstallationID` | Уникальный идентификатор инсталляции сервиса (генерируется или задаётся конфигом) | `cmd/main.go` |
| `ServiceVersion` | Версия сервиса, передаваемая в запросе к лицензионному серверу | `cmd/main.go` |

---

## Пользовательские истории

- Как **оператор сервиса**, я хочу задавать лицензионный ключ через конфиг/env и URL лицензионного сервера, чтобы не нужно было пересобирать бинарник для смены лицензии.
- Как **разработчик**, я хочу запускать сервис без реального лицензионного сервера (мок), чтобы разрабатывать и тестировать в изоляции.
- Как **оператор**, я хочу, чтобы при недоступности лицензионного сервера сервис продолжал работать в Community mode, а не падал.

---

## Требования

### REQ-1: LicenseClient интерфейс и мок

**REQ-1.1** WHEN сервис инициализируется, the system SHALL создать `LicenseClient` через интерфейс, допускающий подмену реализации (gRPC-клиент или мок) без изменения `Manager`.

**REQ-1.2** WHEN `LicenseClient` не задан явно (URL не сконфигурирован), the system SHALL использовать `MockLicenseClient`, возвращающий `CommunityDefaults()` без сетевых вызовов.

**REQ-1.3** WHEN `MockLicenseClient.ValidateLicense()` вызывается, the system SHALL вернуть `Claims` с `Tier=community` и стандартными лимитами Community.

### REQ-2: gRPC-запрос к лицензионному серверу

**REQ-2.1** WHEN URL лицензионного сервера сконфигурирован и лицензионный ключ задан, the system SHALL отправить gRPC-запрос с полями: `license_key` (string), `installation_id` (string), `service_version` (string).

**REQ-2.2** WHEN gRPC-соединение устанавливается с лицензионным сервером, the system SHALL использовать TLS-шифрование без клиентской аутентификации (server-side TLS только).

**REQ-2.3** WHEN лицензионный сервер возвращает успешный ответ, the system SHALL принять Claims: `tier`, `features`, `max_workers`, `max_plugins`.

**REQ-2.4** WHEN лицензионный сервер возвращает ошибку или недоступен, the system SHALL сохранить текущие кэшированные Claims и залогировать предупреждение.

### REQ-3: Локальное кэширование и периодическое обновление

**REQ-3.1** WHEN `Manager` успешно получает Claims от `LicenseClient`, the system SHALL сохранить их в памяти (RWMutex-защищённое поле) и обновить Prometheus-метрики.

**REQ-3.2** WHEN запущен сервис, the system SHALL периодически (по тикеру с интервалом `CacheTTL`) вызывать `LicenseClient.ValidateLicense()` для обновления кэша.

**REQ-3.3** WHEN при периодическом обновлении `LicenseClient` возвращает ошибку, the system SHALL оставить в кэше последние успешно полученные Claims (или `CommunityDefaults()` если успешных вызовов ещё не было) и продолжить работу.

**REQ-3.4** WHEN при периодическом обновлении Claims успешно обновляются и тир изменился, the system SHALL залогировать информационное сообщение об изменении тира.

### REQ-4: Fallback на Community

**REQ-4.1** WHEN при старте сервиса `LicenseClient.ValidateLicense()` возвращает ошибку, the system SHALL запустить сервис с `CommunityDefaults()` и залогировать предупреждение.

**REQ-4.2** WHEN лицензионный ключ не задан в конфиге (пустая строка), the system SHALL использовать `CommunityDefaults()` без обращения к лицензионному серверу.

### REQ-5: Конфигурация

**REQ-5.1** WHEN конфиг лицензии читается, the system SHALL принять поля: `url` (string, URL лицензионного сервера), `license_key` (string), `installation_id` (string, опционально), `cache_ttl` (duration, default=5m).

**REQ-5.2** WHEN поле `installation_id` не задано в конфиге, the system SHALL использовать hostname машины в качестве `installation_id`.

**REQ-5.3** WHEN конфиг лицензии читается, the system SHALL NOT принимать поля `key` (PASETO-токен) и `file` (путь к файлу токена) — они удаляются.

### REQ-6: Удаление PASETO и механики сборки

**REQ-6.1** WHEN собирается бинарник сервиса, the system SHALL NOT требовать переменной `licensePublicKey` или аргумента `-ldflags "-X main.licensePublicKey=..."`.

**REQ-6.2** WHEN собирается Docker-образ, the system SHALL NOT принимать build-аргумент `LICENSE_PUBLIC_KEY` — он удаляется из `Dockerfile`.

**REQ-6.3** WHEN компилируется пакет `license`, the system SHALL NOT содержать импорта `aidanwoods.dev/go-paseto`.

### REQ-7: Метрики и совместимость интерфейсов

**REQ-7.1** WHEN `Manager` обновляет Claims, the system SHALL обновить метрики `license_valid` (1 если не Community, 0 если Community или ошибка) и `license_feature_denied_total`.

**REQ-7.2** WHEN `FeatureGate.Enabled()`, `MaxWorkers()`, `MaxPlugins()` вызываются, the system SHALL вернуть результат на основе кэшированных Claims без изменения сигнатуры методов интерфейса `core.FeatureGate`.

**REQ-7.3** WHEN вызывается `go test ./...`, the system SHALL пройти все существующие тесты, не использующие PASETO-специфичную логику.

---

## Топологический порядок

```
REQ-1.1 → REQ-1.2 → REQ-1.3
Причина: интерфейс LicenseClient должен быть определён до мока и Manager.

REQ-1.1 → REQ-2.1 → REQ-2.2 → REQ-2.3 → REQ-2.4
Причина: gRPC-клиент реализует LicenseClient.

REQ-1.1 → REQ-3.1 → REQ-3.2 → REQ-3.3 → REQ-3.4
Причина: Manager зависит от LicenseClient для кэширования.

REQ-3.1 → REQ-4.1 → REQ-4.2
Причина: fallback — частный случай поведения Manager.

REQ-5.1 → REQ-5.2 → REQ-5.3
Причина: конфиг должен быть определён до Manager.

REQ-6.1 → REQ-6.2 → REQ-6.3 (независимо от REQ-1..5, но выполняется одновременно)

REQ-7.1 → REQ-7.2 → REQ-7.3 (верификация на всех предыдущих)
```

---

## Открытые вопросы для фазы Design

| Вопрос | Почему важно | Затрагивает REQ |
|--------|-------------|-----------------|
| Схема gRPC API лицензионного сервера: proto-файл или JSON-over-gRPC? Нужно ли генерировать клиентский код из .proto? | Определяет, нужна ли codegen-фаза и как структурировать `LicenseClient` | REQ-2.1, REQ-2.3 |
| Где хранить `service_version` — в конфиге или вшивать через ldflags как `var serviceVersion string`? | Если ldflags остаётся только для версии — это допустимо, но надо явно зафиксировать | REQ-2.1, REQ-6.1 |
| `MockLicenseClient` — только для тестов или также для запуска в dev без сервера? | Определяет, нужна ли конфигурируемая mock-стратегия | REQ-1.2 |

---

## Команды верификации

| Действие | Команда | Источник |
|----------|---------|---------|
| Тест | `go test ./...` | `go.mod` + стандарт проекта |
| Сборка | `go build ./cmd/main.go` | `Dockerfile` |
| Lint | `golangci-lint run` | предположительно CI |
| Generate | `easyp --cfg easyp.yaml generate` | `easyp.yaml` (если потребуется proto для gRPC-клиента) |
