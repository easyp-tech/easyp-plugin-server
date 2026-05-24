# Выполнение плагинов с диска — Task Plan

**Status:** Draft
**Date:** 2026-05-24

## Тип работы: Migration

Реструктуризация способа выполнения плагинов: Docker-in-Docker → exec с диска. Наблюдаемое поведение (gRPC API, stdin/stdout protobuf-протокол) не меняется.

---

**Test Style Source:** Tier 2
- Evidence: `internal/core/pool_test.go`, `internal/adapters/registry/registry_test.go`
- Key patterns: стандартный `go test`, table-driven tests, `testing.T`, моки через интерфейсы в test-файлах

**Commands:**

| Action   | Command                           | Source       |
|----------|-----------------------------------|--------------|
| Test     | `go test ./...`                   | Taskfile.yml |
| Build    | `go build -o main ./cmd/main.go`  | Taskfile.yml |
| Lint     | `golangci-lint run ./...`         | Taskfile.yml |
| Generate | `easyp --cfg easyp.yaml generate` | Taskfile.yml |

---

## Матрица покрытия

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1 | T-2, T-3 | CP-1 (Equivalence — протокол) |
| REQ-1.2 | T-3 | CP-1 |
| REQ-1.3 | T-1, T-3 | CP-5 (Exclusion — not found vs permission denied) |
| REQ-1.4 | T-3 | CP-1 |
| REQ-1.5 | T-1, T-3 | CP-15 (Propagation — env) |
| REQ-1.6 | T-1, T-3 | CP-16 (Propagation — timeout) |
| REQ-1.7 | T-1, T-3 | CP-2 (Absence — env isolation) |
| REQ-1.8 | T-1, T-3 | CP-3 (Absence — process group) |
| REQ-1.9 | T-1, T-3 | CP-4 (Absence — output limit) |
| REQ-1.10 | T-1, T-3 | CP-5 (Exclusion) |
| REQ-2.1 | T-1, T-2 | CP-14 (Round-trip — config) |
| REQ-2.2 | T-1, T-2 | CP-6 (Absence — empty command) |
| REQ-2.3 | T-2 | CP-9 (Propagation — migration) |
| REQ-2.4 | T-1, T-2 | CP-6, CP-7 (Absence — path traversal) |
| REQ-2.5 | T-1, T-2 | CP-6, CP-7 |
| REQ-2.6 | T-1, T-2 | CP-8 (Absence — old format) |
| REQ-3.1 | T-2 | CP-10 (Absence — domain) |
| REQ-3.2 | T-2 | CP-10 |
| REQ-4.1 | T-1, T-4 | CP-11 (Absence — Docker codes) |
| REQ-4.2 | T-1, T-4 | CP-12 (Equivalence — signal) |
| REQ-5.1 | T-5 | CP-13 (Propagation — telemetry) |
| REQ-6.1 | T-6 | — (инфраструктура) |
| REQ-6.2 | T-6 | — (инфраструктура) |
| REQ-6.3 | T-6 | — (инфраструктура) |
| REQ-7.1 | T-3 | CP-17 (Equivalence — hot reload) |

---

## T-1: Написать preservation-тесты для нового поведения (GREEN)

*_Requirements: REQ-1.1–REQ-1.10, REQ-2.1–REQ-2.6, REQ-4.1, REQ-4.2_*
*_Complexity: complex_*
*_Test_Style: `internal/core/pool_test.go`_*

GOAL: Зафиксировать ожидаемое поведение новой системы через тесты, которые будут проверять реализацию.

### T-1.1: Создать test helper — mock-бинарник плагина
- Файл: `internal/adapters/registry/testdata/mock_plugin.go`
- Написать Go-программу, которая компилируется в бинарник:
  - Читает stdin, пишет в stdout фиксированный protobuf `CodeGeneratorResponse`
  - При env `MOCK_EXIT_CODE=N` — завершается с кодом N
  - При env `MOCK_STDERR=text` — пишет text в stderr
  - При env `MOCK_OUTPUT_SIZE=N` — пишет N байт мусора в stdout
  - При env `MOCK_SLEEP=duration` — спит указанное время
  - При env `MOCK_FORK=1` — форкает дочерний процесс (sleep 1h)
  - При env `MOCK_PRINT_ENV=1` — пишет `os.Environ()` в stderr
- CRITICAL: Бинарник должен компилироваться в `TestMain()` теста и помещаться во временную директорию.

### T-1.2: Написать тесты на `PluginConfig` и `ValidateConfig`
- Файл: `internal/adapters/registry/registry_test.go`
- Table-driven tests:
  - `TestPluginConfig_RoundTrip`: Marshal → Unmarshal PluginConfig → равенство (CP-14)
  - `TestValidateConfig_EmptyCommand`: command = [] → error (CP-6)
  - `TestValidateConfig_PathTraversal`: command = ["../../bin/sh"] → error (CP-7)
  - `TestValidateConfig_PathTraversal_DotDot`: command = ["/plugins/../etc/passwd"] → error (CP-7)
  - `TestValidateConfig_ValidBinary`: command = ["/plugins/grpc/go/v1.5.1/plugin"] → nil (CP-6)
  - `TestValidateConfig_PythonInterpreter`: command = ["python3", "/plugins/.../plugin.py"] → nil (CP-7)
  - `TestValidateConfig_OldDockerFormat`: config = `{"docker":{}}` → error (CP-8)

### T-1.3: Написать тесты на `plugin.Generate()`
- Файл: `internal/adapters/registry/registry_test.go`
- Table-driven tests (все используют mock-бинарник из T-1.1):
  - `TestGenerate_Success`: Вызов с валидным mock-бинарником → корректный CodeGeneratorResponse (CP-1)
  - `TestGenerate_BinaryNotFound`: command[0] не существует → ошибка "not found" (CP-5)
  - `TestGenerate_PermissionDenied`: command[0] без +x → ошибка "permission denied" (CP-5)
  - `TestGenerate_NonZeroExit`: mock MOCK_EXIT_CODE=1 → ошибка с exit code и stderr (CP-1)
  - `TestGenerate_EnvIsolation`: MOCK_PRINT_ENV=1, проверка что DATABASE_URL отсутствует в выводе (CP-2)
  - `TestGenerate_CustomEnv`: config.env = {"MY_VAR":"val"}, MOCK_PRINT_ENV=1, проверка наличия MY_VAR (CP-15)
  - `TestGenerate_OutputSizeLimit`: MOCK_OUTPUT_SIZE=100MB при maxOutputSize=1MB → ошибка (CP-4)
  - `TestGenerate_Timeout`: config.timeout="100ms", MOCK_SLEEP=5s → ошибка таймаута (CP-16)
  - `TestGenerate_ProcessGroupKill`: MOCK_FORK=1, timeout → все процессы убиты (CP-3)

### T-1.4: Написать тесты на `isTransient()`
- Файл: `internal/core/pool_test.go`
- Table-driven tests:
  - `TestIsTransient_NoDockerCodes`: ExitError с кодами 125/126/127 → false (CP-11)
  - `TestIsTransient_Signal`: Процесс убит SIGKILL → false (CP-12)
  - `TestIsTransient_DeadlineExceeded`: context.DeadlineExceeded → false (существующее поведение)
- CRITICAL: Эти тесты ДОЛЖНЫ упасть на текущем коде (125/126/127 сейчас возвращают true).

---

## T-2: Реализовать модель данных и миграцию (CODE)

*_Requirements: REQ-2.1–REQ-2.6, REQ-3.1, REQ-3.2_*
*_Preservation: CP-14, CP-6, CP-7, CP-8, CP-9, CP-10_*
*_Complexity: standard_*

GOAL: Заменить DockerConfig на PluginConfig, обновить Registry.New(), создать SQL-миграцию.

### T-2.1: Заменить `DockerConfig`/`PluginConfig` на новую структуру
- Файл: `internal/adapters/registry/registry.go`
- Удалить тип `DockerConfig` (строки 31-41)
- Удалить текущий тип `PluginConfig` (строки 43-46)
- Добавить новый тип:
  ```go
  type PluginConfig struct {
      Command []string          `json:"command"`
      Env     map[string]string `json:"env,omitempty"`
      Timeout string            `json:"timeout,omitempty"`
  }
  ```
- Добавить функцию `ValidateConfig(config json.RawMessage, pluginsDir string) error`:
  - Проверяет что config можно десериализовать в PluginConfig
  - Проверяет len(Command) > 0
  - Проверяет что хотя бы один элемент Command — абсолютный путь внутри pluginsDir (после `filepath.Clean` + `strings.HasPrefix`)

### T-2.2: Обновить конструктор `Registry.New()` и структуру `Registry`
- Файл: `internal/adapters/registry/registry.go`
- Изменить struct `Registry`: убрать `domain *url.URL`, добавить `pluginsDir string`, `maxOutputSize int64`
- Изменить struct `plugin`: убрать `domain *url.URL`
- Изменить сигнатуру `New()`: `func New(_ context.Context, db *database.SQL, pluginsDir string, maxOutputSize int64) (*Registry, error)`
- Убрать `url.Parse(domain)`, вместо этого проверить что `pluginsDir != ""`
- В `Get()`: убрать строку `dbFormat.domain = r.domain`

### T-2.3: Добавить вызов `ValidateConfig` в `Create()` и `Update()`
- Файл: `internal/adapters/registry/registry.go`
- В `Create()`: перед `INSERT` вызвать `ValidateConfig(req.Config, r.pluginsDir)`, при ошибке — return error
- В `Update()`: перед `UPDATE` вызвать `ValidateConfig(req.Config, r.pluginsDir)`, при ошибке — return error

### T-2.4: Создать SQL-миграцию
- Файл: `migrate/5.disk_plugin_config.sql`
- Содержимое:
  ```sql
  -- up
  UPDATE plugins SET config = jsonb_build_object(
      'command', jsonb_build_array(
          '/plugins/' || group_name || '/' || name || '/' || version || '/plugin'
      )
  ) WHERE config ? 'docker';

  -- down
  -- Обратная миграция невозможна без оригинальных Docker-конфигов.
  ```

### T-2.5: Обновить `cmd/main.go`
- Файл: `cmd/main.go`
- Заменить `registryConfig.Domain` на `PluginsDir` и `MaxOutputSize`:
  ```go
  type registryConfig struct {
      PluginsDir    string `env:"PLUGINS_DIR, default=/plugins"     yaml:"plugins_dir"`
      MaxOutputSize int64  `env:"MAX_OUTPUT_SIZE, default=67108864" yaml:"max_output_size"`
  }
  ```
- Обновить вызов `registry.New(ctx, db, cfg.Registry.PluginsDir, cfg.Registry.MaxOutputSize)`
- Убрать import `"net/url"` если больше не используется

### T-2.6: Проверить сборку и тесты T-1.2
- Запустить `go build -o main ./cmd/main.go` — ДОЛЖЕН компилироваться
- Запустить `go test ./internal/adapters/registry/...` — тесты T-1.2 ДОЛЖНЫ проходить

---

## T-3: Реализовать выполнение плагина через exec (CODE)

*_Requirements: REQ-1.1–REQ-1.10, REQ-7.1_*
*_Preservation: CP-1, CP-2, CP-3, CP-4, CP-5, CP-15, CP-16, CP-17_*
*_Complexity: complex_*

GOAL: Заменить docker run на exec.CommandContext с изоляцией env, process group, output limit.

### T-3.1: Переписать `plugin.Generate()`
- Файл: `internal/adapters/registry/registry.go`
- Удалить весь текущий код `Generate()` (строки 196-280: docker args, imageName, docker run)
- Реализовать новую версию:
  1. `proto.Marshal(req)` → requestData (без изменений)
  2. Проверить `len(p.pluginConfig.Command) == 0` → вернуть ошибку
  3. `cmd := exec.CommandContext(ctx, p.pluginConfig.Command[0], p.pluginConfig.Command[1:]...)`
  4. `cmd.Stdin = bytes.NewReader(requestData)`
  5. `cmd.Env = []string{}` — чистое окружение
  6. Добавить env из `p.pluginConfig.Env`: `for k, v := range p.pluginConfig.Env { cmd.Env = append(cmd.Env, k+"="+v) }`
  7. `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`
  8. Создать `stdout` и `stderr` pipes с `io.LimitReader(pipe, maxOutputSize)`
  9. `cmd.Start()` — обработать `exec.ErrNotFound` (→ "not found") и `os.ErrPermission` (→ "permission denied")
  10. Читать stdout/stderr в goroutines
  11. `cmd.Wait()` — при ошибке: извлечь exit code из `*exec.ExitError`, приложить stderr
  12. `proto.Unmarshal(output, &response)` → вернуть response
- IMPORTANT: При context cancellation/timeout — kill process group: `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)`
- Добавить поле `maxOutputSize int64` в struct `plugin`, проставлять в `Get()` из `r.maxOutputSize`

### T-3.2: Обработать per-plugin timeout
- Файл: `internal/adapters/registry/registry.go`
- В начале `Generate()`: если `p.pluginConfig.Timeout != ""`:
  1. `d, err := time.ParseDuration(p.pluginConfig.Timeout)`
  2. `ctx, cancel := context.WithTimeout(ctx, d)` + `defer cancel()`
- NOTE: Если timeout не задан — используется timeout из WorkerPool (120s по умолчанию)

### T-3.3: Проверить тесты T-1.3
- Запустить `go test ./internal/adapters/registry/... -run TestGenerate` — все тесты ДОЛЖНЫ проходить
- Запустить `go build -o main ./cmd/main.go` — ДОЛЖЕН компилироваться

---

## T-4: Обновить `isTransient()` в WorkerPool (CODE)

*_Requirements: REQ-4.1, REQ-4.2_*
*_Preservation: CP-11, CP-12_*
*_Complexity: mechanical_*

GOAL: Убрать Docker-специфичную классификацию ошибок.

### T-4.1: Переписать `isTransient()`
- Файл: `internal/core/pool.go`
- Удалить проверку Docker exit codes (строки 319-326):
  ```go
  // Удалить:
  var exitErr *exec.ExitError
  if errors.As(err, &exitErr) {
      switch exitErr.ExitCode() {
      case dockerExitCodeError, dockerExitCodeNotFound, dockerExitCodePermission:
          return true
      }
  }
  ```
- Удалить проверку Docker-подстрок (строки 328-337):
  ```go
  // Удалить:
  case strings.Contains(msg, "connection refused"):
  case strings.Contains(msg, "daemon"):
  case strings.Contains(msg, "temporary failure"):
  ```
- Оставить только `context.DeadlineExceeded → false` и `default → false`
- Удалить константы `dockerExitCodeError`, `dockerExitCodeNotFound`, `dockerExitCodePermission` (если есть)
- Убрать неиспользуемые imports (`os/exec`, `strings`)

### T-4.2: Проверить тесты T-1.4
- Запустить `go test ./internal/core/... -run TestIsTransient` — все тесты ДОЛЖНЫ проходить
- Запустить `go test ./...` — все тесты ДОЛЖНЫ проходить

---

## T-5: Обновить `TracingPlugin` (CODE)

*_Requirements: REQ-5.1_*
*_Preservation: CP-13_*
*_Complexity: mechanical_*

GOAL: Заменить docker.exec спан на process.exec.

### T-5.1: Обновить спаны и атрибуты
- Файл: `internal/telemetry/tracing_plugin.go`
- Строка 52: `p.tracer.Start(ctx, "docker.exec"` → `p.tracer.Start(ctx, "process.exec"`
- Строки 53-56: заменить атрибуты:
  - `attribute.String("docker.image", imageName)` → `attribute.String("process.command", imageName)`
  - `attribute.String("docker.command", "docker run")` — удалить
- Строка 76: `dockerSpan.SetAttributes(attribute.Int("docker.exit_code", ...)` → `attribute.Int("process.exit_code", ...)`
- Переименовать переменную `dockerSpan` → `processSpan` (все использования)
- Обновить комментарии: строка 51 `// Create child span for Docker execution` → `// Create child span for process execution`
- Строка 60: `// Wrap Docker execution with Pyroscope labels` → `// Wrap process execution with Pyroscope labels`
- Строка 69: `// End docker span` → `// End process span`
- Убрать import `"os/exec"` если больше не используется

### T-5.2: Проверить сборку
- Запустить `go build -o main ./cmd/main.go`
- Запустить `go test ./internal/telemetry/...`

---

## T-6: Обновить Dockerfile (CODE)

*_Requirements: REQ-6.1, REQ-6.2, REQ-6.3_*
*_Preservation: —_*
*_Complexity: mechanical_*

GOAL: Минимальный debian-образ без Docker CLI, с VOLUME для плагинов.

### T-6.1: Переписать Dockerfile
- Файл: `Dockerfile`
- Builder stage: заменить `FROM golang:alpine3.22` → `FROM golang:1.24-bookworm`
- Builder stage: убрать `RUN apk update && apk add --no-cache ca-certificates` (в debian сертификаты уже есть)
- Runtime stage: заменить `FROM alpine:3.22` → `FROM debian:bookworm-slim`
- Runtime stage: убрать `RUN apk add --no-cache docker-cli ca-certificates`
- Runtime stage: добавить `RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*`
- Runtime stage: добавить `VOLUME ["/plugins"]`
- IMPORTANT: Убрать docker-cli — это ключевое изменение.

### T-6.2: Проверить сборку образа (ручная)
- NOTE: Ручная проверка — запустить `docker build .` локально чтобы убедиться что образ собирается.

---

## T-7: Финальная проверка (GATE)

*_Requirements: все REQ-1.x–REQ-7.x_*
*_Complexity: standard_*

### T-7.1: Запустить полный набор тестов
- `go test ./...` — все тесты ДОЛЖНЫ проходить

### T-7.2: Запустить линтер
- `golangci-lint run ./...` — ноль ошибок

### T-7.3: Проверить сборку
- `go build -o main ./cmd/main.go` — успешная сборка

### T-7.4: Трассировка требований
- Проверить что каждый REQ покрыт тестом из T-1
- Проверить что каждый CP подтверждён тестом
