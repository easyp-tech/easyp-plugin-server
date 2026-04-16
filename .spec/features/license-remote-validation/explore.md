# Исследование: Remote License Validation

## Намерение

Сейчас лицензия валидируется локально: сервис держит PASETO v4.public токен, проверяет Ed25519-подпись встроенным публичным ключом и кэширует `Claims` в памяти. Пользователь хочет перейти на внешнюю валидацию — вместо локальной криптографии сервис будет ходить по API в отдельный лицензионный сервер, который подтверждает валидность лицензии и возвращает Claims.

## Исследование

### Текущая архитектура (brownfield)

```
LICENSE_KEY/FILE (PASETO v4.public token)
  → license.Manager.applyToken()
      → paseto.ParseV4Public() [Ed25519, встроенный publicKey через ldflags]
          → Claims { Tier, Features, MaxWorkers, MaxPlugins, ExpiresAt }
              → license.FeatureGate.Enabled(feature)   [кэш, RWMutex]
```

Ключевые файлы:
- `internal/license/manager.go` — парсинг, кэш claims, 60-секундный тикер истечения
- `internal/license/claims.go` — структура Claims, CommunityDefaults()
- `internal/license/gate.go` — FeatureGate читает claims из Manager
- `internal/license/features.go` — enum feature, IsEnterprise()
- `internal/license/metrics.go` — Prometheus: license_valid, expiry_timestamp, feature_denied_total
- `internal/api/license_interceptor.go` — gRPC interceptor, method→Feature маппинг
- `cmd/main.go:237-265` — инициализация Manager, FeatureGate, LicenseInterceptor

Текущий конфиг:
```yaml
license:
  key: ""   # inline PASETO token
  file: ""  # path to file with token
```
Публичный ключ — `var licensePublicKey string` в `cmd/main.go`, инжектируется через `-ldflags`.

### Что должно сохраниться (preservation constraints)

- Интерфейс `core.FeatureGate` (методы `Enabled`, `MaxWorkers`, `MaxPlugins`) — используется в `core.New`, `ratelimiter`, `api.LicenseInterceptor`
- Структура `license.Claims` — используется в `FeatureGate`, тикере истечения
- Community defaults (`CommunityDefaults()`) — fallback при недоступном сервере
- Prometheus-метрики — должны продолжать работать
- Все существующие тесты: `internal/license/claims_test.go`, `features_test.go`

## Build Tooling

- **Orchestrator:** Task (`Taskfile.yml`)
- **Test:** `go test ./...`
- **Build:** `go build ./cmd/main.go`
- **Lint:** не задан явно в Taskfile (предположительно `golangci-lint run`)
- **Generate:** `easyp --cfg easyp.yaml generate` (proto codegen)
- **Source:** `Taskfile.yml`, `go.mod`

## Рассматриваемые варианты

### Вариант A: Полная замена — только удалённая валидация

Убрать PASETO и ldflags-публичный ключ. Сервис при старте и периодически посылает HTTP/gRPC запрос на license server с лицензионным ключом (простой string: UUID или API-key), получает Claims в ответ.

**Плюсы:**
- Централизованный контроль: сервер может отозвать лицензию мгновенно
- Не нужен build-time ldflags, проще деплой
- Нет зависимости от криптографической библиотеки PASETO

**Минусы:**
- Жёсткая зависимость от доступности лицензионного сервера
- Сетевые задержки при валидации
- Нужна стратегия fallback при недоступности

**Сложность:** Средняя

---

### Вариант B: Гибрид — удалённая валидация с локальным кэшем и fallback

Сервис отправляет лицензионный ключ на remote API при старте и по тикеру (например, каждые 5 минут). Ответ (Claims) кэшируется. При недоступности сервера — используется кэш до истечения TTL, потом Community.

**Плюсы:**
- Устойчивость к временным сбоям сети
- Централизованный контроль лицензиями
- Можно реализовать без PASETO (или оставить PASETO как опцию)

**Минусы:**
- Сложнее: нужна логика кэша + fallback + TTL
- Revocation не мгновенный (до следующего обновления кэша)

**Сложность:** Средняя-высокая

---

### Вариант C: PASETO остаётся + дополнительная проверка отзыва через API

Локальная валидация PASETO сохраняется, но добавляется вызов к remote API для проверки "не отозван ли токен" (revocation check). Remote сервер — только для revocation list.

**Плюсы:**
- Минимальные изменения в коде
- Offline-режим работает без изменений
- Мгновенная локальная проверка + возможность отзыва

**Минусы:**
- Сохраняется сложность PASETO + ldflags
- Нужны два механизма (local crypto + remote HTTP)
- Избыточно, если цель — полностью убрать локальную криптографию

**Сложность:** Низкая-средняя

---

## Ограничения и риски

- **Доступность сервера:** если лицензионный сервер недоступен, нужна чёткая стратегия (Community fallback, кэш, hard fail)
- **Безопасность транспорта:** HTTP вызов должен быть по TLS (HTTPS/gRPC+TLS)
- **Аутентификация запроса:** как сервис идентифицирует себя перед лицензионным сервером? (API key, mTLS, JWT)
- **Нет лицензионного API:** протокол и контракт внешнего сервера пока не определён — нужен spike или договорённость
- **Breaking change конфига:** поле `license.key` (PASETO token) заменяется на другой идентификатор
- **Тесты:** `claims_test.go` и `features_test.go` тестируют PASETO-логику — при полном переходе потребуется переписать

## Рекомендуемое направление

**Вариант B** — удалённая валидация с локальным кэшем и graceful fallback на Community.

Обоснование: даёт централизованный контроль (основная цель), при этом устойчив к временным сетевым сбоям. Лицензионный ключ становится простым string (UUID/API-key), который передаётся в запросе к license server. Manager заменяет PASETO-логику на HTTP/gRPC клиент.

[ASSUMPTION: Лицензионный сервер предоставляет HTTP REST или gRPC API — протокол пока не определён]  
[ASSUMPTION: Лицензионный ключ становится простым string (не PASETO-токен), передаваемым в Authorization или теле запроса]  
[ASSUMPTION: При недоступности сервера сервис падает в Community mode, а не в hard fail]  
[ASSUMPTION: Интерфейс `core.FeatureGate` и структура `Claims` остаются без изменений]

## Границы объёма

- **Must-have (v1):**
  - Замена `license.Manager` на remote-валидацию через HTTP/gRPC клиент
  - Локальное кэширование Claims с TTL
  - Graceful fallback на Community при ошибке сети
  - Периодическое обновление (refresh тикер)
  - Сохранение Prometheus-метрик
  - Обновление конфига: убрать `key`/`file` (PASETO), добавить `url` и `license_key`

- **Deferred (v2):**
  - mTLS между сервисом и лицензионным сервером
  - Логика retry с exponential backoff при ошибке
  - Webhook от лицензионного сервера для push-инвалидации

- **Needs spike:**
  - Протокол и схема ответа лицензионного сервера (HTTP REST vs gRPC, формат Claims в ответе)
  - Аутентификация сервиса перед лицензионным сервером

## Допущения и открытые вопросы

1. **Какой протокол использует лицензионный сервер?** HTTP REST (JSON) или gRPC? Есть ли уже готовый контракт / proto-схема?
2. **Что передаётся в запросе?** Только лицензионный ключ (string)? Или также ID инсталляции, версия сервиса?
3. **Стратегия недоступности:** Community mode (мягко) или запрет всех Enterprise-запросов (жёстко)?
4. **Аутентификация запроса к license server:** API key в заголовке? mTLS? Без аутентификации?
5. **Нужно ли сохранить поддержку PASETO** как резервный режим (offline), или полностью убрать?
