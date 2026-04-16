# Remote License Validation — План реализации

## Вводная информация

**Test Style Source:** Tier 2
- Evidence: `internal/license/claims_test.go`, `internal/license/features_test.go`
- Паттерны: стандартный пакет `testing`, `testify/assert`, table-driven tests (`tests := []struct{...}{...}`), тесты в том же пакете (`package license` / `package core`), именование `TestXxx_Scenario`

**Commands:**

| Action | Command | Source |
|--------|---------|--------|
| Test   | `go test ./...` | `go.mod` |
| Build  | `go build ./cmd/main.go` | `Dockerfile` |
| Lint   | `golangci-lint run` | CI |

---

## Матрица покрытия

| Требование | Задачи | Свойство корректности |
|-----------|--------|----------------------|
| REQ-1.1 | T-2, T-3 | CP-1, CP-6 |
| REQ-1.2 | T-3 | CP-1 |
| REQ-1.3 | T-6 | CP-1 |
| REQ-1.4 | T-3 | CP-1 |
| REQ-3.1 | T-4, T-6 | CP-2 |
| REQ-3.2 | T-4 | CP-2 |
| REQ-3.3 | T-6 | CP-3 |
| REQ-3.4 | T-4 | CP-2 |
| REQ-4.1 | T-4, T-6 | CP-4 |
| REQ-4.2 | T-4 | CP-4 |
| REQ-5.1 | T-4 | CP-8 |
| REQ-5.2 | T-4 | CP-8 |
| REQ-6.1 | T-7 | CP-8 |
| REQ-6.2 | T-7 | CP-8 |
| REQ-6.3 | T-5 | CP-9 |
| REQ-7.1 | T-4, T-5 | CP-7 |
| REQ-7.2 | T-1, T-5 | CP-6 |
| REQ-7.3 | T-8 | CP-6 |

---

## Тип работы: Migration

Перестройка внутренней реализации `license.Manager` (PASETO → `core.LicenseClient`) без изменения наблюдаемого поведения (интерфейс `core.FeatureGate` и Prometheus-метрики сохраняются). Дополнительно: добавление новых доменных типов в `core` (pure feature).

**Порядок задач:**
```
T-1: GREEN (preservation) → T-2..T-5: CODE (bottom-up) → T-6: GREEN (новые тесты) → T-7: CODE (сборка) → T-8: GATE
```

---

## T-1 · GREEN — Написать preservation-тесты для `FeatureGate`

*_Requirements: REQ-7.2_*
*_Test_Style: `internal/license/features_test.go`_*

GOAL: Зафиксировать текущее наблюдаемое поведение `FeatureGate` до начала рефакторинга. Тесты должны проходить до, во время и после всех изменений.

IMPORTANT: Эти тесты должны проходить на **немодифицированной** кодовой базе.
IMPORTANT: Следуй паттерну Tier 2 — `testify/assert`, table-driven, `package license`.
DO NOT: Изменять production-код в этой задаче.

Инструкции:
1. Создать файл `internal/license/gate_preservation_test.go`.
2. Написать тест `TestFeatureGate_CommunityDefaults_Enabled` — проверяет, что для Community Claims все нон-Enterprise фичи включены, все Enterprise фичи выключены.
3. Написать тест `TestFeatureGate_MaxWorkers_Returns4ForCommunity` — проверяет `MaxWorkers()` = 4 при Community Claims.
4. Написать тест `TestFeatureGate_MaxPlugins_Returns10ForCommunity` — проверяет `MaxPlugins()` = 10 при Community Claims.
5. Запустить: `go test ./internal/license/...`
6. Все тесты должны быть GREEN.

---

## T-2 · CODE — Добавить `core.LicenseClaims`, `core.LicenseClient`, `core.CommunityLicenseClaims()`

*_Requirements: REQ-1.1, REQ-7.2_*
*_Preservation: CP-6 (интерфейс FeatureGate не меняется)_*

GOAL: Добавить новые доменные типы в слой бизнес-логики согласно соглашению проекта (все интерфейсы в `core/domain.go`).

CRITICAL: Изменять только `internal/core/domain.go`. Один файл в этой задаче.
DO NOT: Удалять `license.Claims` или `license.CommunityDefaults()` в этой задаче — они ещё нужны.

Подзадачи:
- [ ] 1. В `internal/core/domain.go` добавить тип `LicenseClaims` (поля: `Tier string`, `Features []Feature`, `MaxWorkers int`, `MaxPlugins int`) — `go build ./...`
- [ ] 2. В `internal/core/domain.go` добавить интерфейс `LicenseClient` с методом `ValidateLicense(ctx context.Context) (LicenseClaims, error)` — `go build ./...`
- [ ] 3. В `internal/core/domain.go` добавить функцию `CommunityLicenseClaims() LicenseClaims` (возвращает community-тир, 4 воркера, 10 плагинов, все нон-Enterprise фичи) — `go test ./internal/core/...`

После всех подзадач: `go build ./cmd/main.go` и `golangci-lint run ./internal/core/...`

---

## T-3 · CODE — Реализовать `license.MockLicenseClient`

*_Requirements: REQ-1.1, REQ-1.2, REQ-1.4_*
*_Preservation: CP-1 (мок всегда Enterprise), CP-6_*

GOAL: Создать временную реализацию `core.LicenseClient`, которая всегда возвращает Enterprise-Claims. Добавить TODO-комментарий о замене на реальный gRPC-клиент.

CRITICAL: Создать только `internal/license/client.go`. Один файл в этой задаче.

Подзадачи:
- [ ] 1. Создать `internal/license/client.go` с типом `MockLicenseClient struct{}` и конструктором `NewMockLicenseClient() *MockLicenseClient`; добавить TODO-комментарий `// TODO: replace with real gRPC client when license server is available` — `go build ./internal/license/...`
- [ ] 2. Реализовать метод `ValidateLicense(ctx context.Context) (core.LicenseClaims, error)` — возвращает `core.LicenseClaims{Tier: "enterprise", MaxWorkers: -1, MaxPlugins: -1, Features: все core.Feature}` и `nil` ошибку — `go test ./internal/license/...`

После всех подзадач: `go build ./cmd/main.go`

---

## T-4 · CODE — Переписать `license.Manager` (убрать PASETO, принять `core.LicenseClient`)

*_Requirements: REQ-3.1, REQ-3.2, REQ-3.3, REQ-3.4, REQ-4.1, REQ-4.2, REQ-5.1, REQ-5.2, REQ-7.1_*
*_Preservation: CP-2, CP-3, CP-4, CP-6, CP-7, CP-8_*

GOAL: Заменить всю PASETO-логику в Manager на зависимость от `core.LicenseClient`. Manager кэширует `core.LicenseClaims` и периодически обновляет их через тикер.

CRITICAL: Изменять только `internal/license/manager.go`. Один файл в этой задаче.
DO NOT: Трогать `gate.go`, `claims.go`, `errors.go` — это следующая задача.

Подзадачи:
- [ ] 1. Удалить поля `publicKey`, `hasKey` из struct `Manager`; добавить поле `client core.LicenseClient`; заменить поле `claims Claims` на `claims core.LicenseClaims` — `go build ./internal/license/...` (ожидаются ошибки компиляции в gate.go — пока игнорируем, задача одного файла)
- [ ] 2. Переписать конструктор `NewManager`: принять `client core.LicenseClient` вместо `publicKeyHex string, cfg Config`; добавить проверку `client == nil → panic`; если `cfg.CacheTTL <= 0 → использовать 5m`; убрать `loadToken`/`applyToken` — `go build ./internal/license/...`
- [ ] 3. Добавить в `NewManager` первичный вызов `client.ValidateLicense(ctx)` — при ошибке → `core.CommunityLicenseClaims()` + лог `Warn`; при успехе → кэшировать Claims + лог `Info` с тиром — `go build ./internal/license/...`
- [ ] 4. Переписать `StartExpirationWatcher` → переименовать в `StartRefreshWatcher`; внутри: тикер `cfg.CacheTTL`; при ошибке → сохранять кэш + лог `Warn`; при смене тира → лог `Info` — `go build ./internal/license/...`
- [ ] 5. Удалить методы `ParseToken`, `FormatToken`, `extractClaims`, `applyToken`, `loadToken`, `checkExpiration` из `manager.go` — `go build ./internal/license/...`
- [ ] 6. Обновить `updateMetrics()`: `license_valid = 1` если `claims.Tier == "enterprise"`, иначе 0; убрать `expiryTS` (нет `ExpiresAt` в `core.LicenseClaims`) — `go build ./internal/license/...`
- [ ] 7. Обновить структуру `Config`: убрать `Key string` и `File string`; добавить `CacheTTL time.Duration \`env:"CACHE_TTL,default=5m" yaml:"cache_ttl"\`` — `go test ./internal/license/...`

После всех подзадач: `go build ./cmd/main.go` и `golangci-lint run ./internal/license/...`

---

## T-5 · CODE — Обновить `gate.go`, `claims.go`, `errors.go` (перейти на `core.LicenseClaims`)

*_Requirements: REQ-6.3, REQ-7.1, REQ-7.2_*
*_Preservation: CP-6, CP-7, CP-9_*

GOAL: Привести вспомогательные файлы пакета `license` в соответствие с новыми типами из `core`. Удалить PASETO-ошибки. Убрать зависимость на `aidanwoods.dev/go-paseto`.

Подзадачи:
- [ ] 1. В `internal/license/gate.go`: заменить тип `claims Claims` → `claims core.LicenseClaims`; обновить `Enabled()` — `Tier == "enterprise"` (string); обновить `MaxWorkers()` и `MaxPlugins()` — читают из `core.LicenseClaims` — `go test ./internal/license/...`
- [ ] 2. В `internal/license/claims.go`: удалить тип `Claims`, `Tier`, константы `TierCommunity`/`TierEnterprise`, функцию `CommunityDefaults()`, `communityFeatures()`; оставить файл как stub или удалить (если все экспорты ушли в `core`) — `go build ./internal/license/...`
- [ ] 3. В `internal/license/errors.go`: удалить `ErrInvalidToken`, `ErrSignatureInvalid`, `ErrTokenExpired`, `ErrInvalidClaims`, `ErrFileNotFound`; оставить только `ErrNoClient = errors.New("license: client must not be nil")` — `go build ./internal/license/...`
- [ ] 4. Запустить `go mod tidy` для удаления `aidanwoods.dev/go-paseto` из `go.mod`/`go.sum` — `go build ./...`

После всех подзадач: `go test ./...` и `golangci-lint run ./internal/license/...`

---

## T-6 · GREEN — Написать тесты для `MockLicenseClient` и `Manager`; перенести `claims_test.go`

*_Requirements: REQ-1.3, REQ-3.1, REQ-3.3, REQ-4.1_*
*_Test_Style: `internal/license/features_test.go`_*

GOAL: Покрыть тестами новые компоненты (MockLicenseClient, Manager). Перенести `claims_test.go` в `core`.

IMPORTANT: Следуй паттерну Tier 2 — `testify/assert`, `package license` / `package core`.

Подзадачи:
- [ ] 1. Создать `internal/license/client_test.go` с тестами `TestMockLicenseClient_ValidateLicense` (Tier=enterprise, error=nil) и `TestMockLicenseClient_AllFeaturesEnabled` (все фичи включая Enterprise) — `go test ./internal/license/...`
- [ ] 2. Создать `internal/license/manager_test.go` с тестами: `TestManager_InitSuccess` (первый вызов успешен → Claims в кэше), `TestManager_InitError_FallbackCommunity` (первый вызов ошибка → CommunityLicenseClaims), `TestManager_Refresh_ErrorPreservesCache` (ошибка тикера → кэш не меняется), `TestManager_NilClient_Panics` — `go test ./internal/license/...`
- [ ] 3. Переместить `internal/license/claims_test.go` → `internal/core/license_claims_test.go`; обновить `package license` → `package core`; заменить вызовы `CommunityDefaults()` → `CommunityLicenseClaims()`, тип `Claims` → `LicenseClaims` — `go test ./internal/core/...`

После всех подзадач: `go test ./...`

---

## T-7 · CODE — Обновить `cmd/main.go`, `Dockerfile`, `go.mod`

*_Requirements: REQ-6.1, REQ-6.2_*
*_Preservation: CP-8_*

GOAL: Убрать остатки PASETO-инфраструктуры из entry point и конфигурации сборки.

Подзадачи:
- [ ] 1. В `cmd/main.go`: удалить `var licensePublicKey string` и все ссылки на него; удалить `licenseConfig.Key`/`licenseConfig.File` из конфигурационной структуры; создавать `license.NewMockLicenseClient()` и передавать в `license.NewManager(client, cfg.License, log, reg, namespace)`; переименовать все вызовы `lm.StartExpirationWatcher` → `lm.StartRefreshWatcher` — `go build ./cmd/main.go`
- [ ] 2. В `Dockerfile`: удалить строку `ARG LICENSE_PUBLIC_KEY=""`; заменить `RUN go build -ldflags "-X main.licensePublicKey=${LICENSE_PUBLIC_KEY}" -o easyp ./cmd/main.go` на `RUN go build -o easyp ./cmd/main.go` — `go build ./cmd/main.go` (локально)

После всех подзадач: `go build ./cmd/main.go` и `go test ./...`

---

## T-8 · GATE — Финальная проверка

*_Requirements: REQ-7.3_*

GOAL: Убедиться, что все требования выполнены, все тесты проходят, сборка чистая.

Инструкции:
1. Запустить полный набор тестов: `go test ./...` — все тесты GREEN.
2. Запустить сборку: `go build ./cmd/main.go` — компилируется без `-ldflags`.
3. Запустить линтер: `golangci-lint run` — нет новых ошибок.
4. Проверить отсутствие PASETO: `grep -r "go-paseto" go.mod go.sum` — пусто.
5. Проверить отсутствие `licensePublicKey` в коде: `grep -r "licensePublicKey\|LICENSE_PUBLIC_KEY" cmd/ Dockerfile` — пусто.
6. Убедиться, что `core.LicenseClient` объявлен в `internal/core/domain.go`: `grep "LicenseClient" internal/core/domain.go`.
7. Убедиться, что preservation-тесты T-1 проходят: `go test ./internal/license/... -run TestFeatureGate`.
