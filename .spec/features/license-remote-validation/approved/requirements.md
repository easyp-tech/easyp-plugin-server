# Remote License Validation — Требования

**Статус:** Draft  
**Автор:** agent  
**Дата:** 2026-04-16

## Обзор

Сервис переходит с локальной валидации PASETO-токенов (Ed25519-подпись, публичный ключ через `-ldflags`) на архитектуру с внешним лицензионным сервером. PASETO и `-ldflags`-инжекция удаляются полностью, механика сборки упрощается.

**Текущий этап (Phase 1):** реальный лицензионный сервер ещё не существует. Вместо gRPC-клиента используется `MockLicenseClient` — хардкодная реализация интерфейса, всегда возвращающая Enterprise-лицензию. `MockLicenseClient` работает в production до появления реального сервера. `Manager` использует интерфейс `LicenseClient`, что позволит заменить мок на реальный gRPC-клиент без изменения `Manager`.

**Будущий этап (Phase 2, вне текущего scope):** реальный gRPC-клиент с TLS-соединением, передачей `license_key` / `installation_id` / `service_version` и обработкой ответа сервера.

---

## Глоссарий

| Термин | Определение | Code Artifact |
|--------|-------------|---------------|
| `LicenseClient` | Интерфейс клиента к лицензионному серверу; текущая реализация — мок, будущая — gRPC-клиент | `internal/license/client.go` (новый) |
| `MockLicenseClient` | Хардкодная реализация `LicenseClient`, всегда возвращающая Enterprise-Claims без сетевых вызовов. Используется в production до появления реального сервера | `internal/license/client.go` (новый) |
| `Manager` | Компонент кэширования и обновления Claims; работает через интерфейс `LicenseClient` | `internal/license/manager.go` |
| `Claims` | Структура данных лицензии: тир, фичи, лимиты | `internal/license/claims.go` |
| `CacheTTL` | Интервал периодического обновления Claims через `LicenseClient` | `internal/license/manager.go` |

---

## Пользовательские истории

- Как **оператор сервиса**, я хочу задавать лицензионный ключ через конфиг/env и URL лицензионного сервера, чтобы не нужно было пересобирать бинарник для смены лицензии.
- Как **разработчик**, я хочу запускать сервис без реального лицензионного сервера (мок), чтобы разрабатывать и тестировать в изоляции.
- Как **оператор**, я хочу, чтобы при недоступности лицензионного сервера сервис продолжал работать в Community mode, а не падал.

---

## Требования

### REQ-1: LicenseClient интерфейс и MockLicenseClient

**REQ-1.1** WHEN сервис инициализируется, the system SHALL создать `LicenseClient` через интерфейс, допускающий подмену реализации без изменения `Manager`.

**REQ-1.2** WHEN `LicenseClient` инициализируется на текущем этапе (Phase 1), the system SHALL использовать `MockLicenseClient` как единственную реализацию — и в production, и в тестах.

**REQ-1.3** WHEN `MockLicenseClient.ValidateLicense()` вызывается, the system SHALL вернуть хардкодные Claims с `Tier=enterprise` и максимальными лимитами (все фичи включены).

**REQ-1.4** WHEN в кодовой базе присутствует `MockLicenseClient`, the system SHALL содержать комментарий `// TODO: replace with real gRPC client when license server is available`, фиксирующий временный характер реализации.

### REQ-2: Будущий gRPC-клиент (Phase 2 — вне текущего scope)

> Требования REQ-2.x описывают поведение реального gRPC-клиента, который заменит `MockLicenseClient` в будущем. Реализация в текущем scope не выполняется.

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

**REQ-5.1** WHEN конфиг лицензии читается, the system SHALL принять поля: `cache_ttl` (duration, default=5m).

**REQ-5.2** WHEN конфиг лицензии читается, the system SHALL NOT принимать поля `key` (PASETO-токен) и `file` (путь к файлу токена) — они удаляются.

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
REQ-1.1 → REQ-1.2 → REQ-1.3 → REQ-1.4
Причина: интерфейс LicenseClient определяется первым, затем MockLicenseClient как его реализация.

REQ-1.1 → REQ-3.1 → REQ-3.2 → REQ-3.3 → REQ-3.4
Причина: Manager зависит от LicenseClient интерфейса для кэширования.

REQ-3.1 → REQ-4.1 → REQ-4.2
Причина: fallback — частный случай поведения Manager при ошибке.

REQ-5.1 → REQ-5.2 (независимо, конфиг упрощается)

REQ-6.1 → REQ-6.2 → REQ-6.3 (независимо от REQ-1..5, выполняется параллельно)

REQ-7.1 → REQ-7.2 → REQ-7.3 (верификация — последние)

REQ-2.x — Phase 2, вне текущего scope, не реализуются.
```

---

## Открытые вопросы для фазы Design

| Вопрос | Почему важно | Затрагивает REQ |
|--------|-------------|-----------------|
| Где определить интерфейс `LicenseClient` — в пакете `license` или `core`? | Если в `core`, FeatureGate и Manager могут зависеть от одного пакета без цикличности | REQ-1.1 |

---

## Команды верификации

| Действие | Команда | Источник |
|----------|---------|---------|
| Тест | `go test ./...` | `go.mod` + стандарт проекта |
| Сборка | `go build ./cmd/main.go` | `Dockerfile` |
| Lint | `golangci-lint run` | предположительно CI |
| Generate | `easyp --cfg easyp.yaml generate` | `easyp.yaml` (если потребуется proto для gRPC-клиента) |
