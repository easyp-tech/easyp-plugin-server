# epctl CLI — Task Plan

## Преамбула

**Work Type:** Pure feature + Migration (переезд сервера + новые команды CLI)

**Test Style Source:** Deferred (v2)
- Тесты не входят в текущий скоуп по решению пользователя

**Commands:**

| Action | Command | Source |
|--------|---------|--------|
| Build (server) | `go build -o easyp ./cmd/easyp/` | design.md §2.6 |
| Build (CLI) | `go build -o epctl ./cmd/epctl/` | design.md §2.6 |
| Lint | `golangci-lint run ./...` | design.md §2.6 |

## Coverage Matrix

| Requirement | Task(s) | Описание |
|-------------|---------|----------|
| REQ-1.1 | T-2 | Серверный бинарник из `cmd/easyp/` |
| REQ-1.2 | T-4 | CLI-бинарник из `cmd/epctl/` |
| REQ-1.3 | T-2 | Namespace метрик = "easyp" |
| REQ-1.4 | T-2 | Удаление `cmd/main.go` |
| REQ-2.1 | T-3, T-5 | Сборка плагинов из registry/ |
| REQ-2.2 | T-3, T-5 | Path-filter для build |
| REQ-2.3 | T-3 | Кэш-попадание |
| REQ-2.4 | T-3 | Fail-fast по умолчанию |
| REQ-2.5 | T-3, T-5 | --continue-on-error |
| REQ-2.6 | T-3, T-5 | Суммарный отчёт |
| REQ-2.7 | T-3 | build_args |
| REQ-2.8 | T-3 | Переименование бинарника |
| REQ-2.9 | T-3 | Пустой registry |
| REQ-3.1 | T-6 | Регистрация через gRPC |
| REQ-3.2 | T-6 | Path-filter для register |
| REQ-3.3 | T-6 | AlreadyExists → skip |
| REQ-3.4 | T-6 | Иная ошибка → stop |
| REQ-3.5 | T-6 | Формирование config.command |
| REQ-3.6 | T-6 | Default addr localhost:8080 |
| REQ-3.7 | T-6 | --plugins-prefix |
| REQ-3.8 | T-6 | Суммарный отчёт register |
| REQ-4.1 | T-6 | List через gRPC |
| REQ-4.2 | T-6 | Флаги фильтрации list |
| REQ-4.3 | T-6 | Пустой список → exit 0 |
| REQ-4.4 | T-6 | Connection failure |
| REQ-4.5 | T-6 | Default addr list |
| REQ-5.1 | T-1, T-7 | Валидация YAML |
| REQ-5.2 | T-7 | Файл не существует |
| REQ-5.3 | T-1, T-7 | Unknown fields warning |
| REQ-5.4 | T-1, T-7 | Валидный YAML → exit 0 |
| REQ-5.5 | T-7 | Нет аргумента → usage |
| REQ-6.1 | T-1 | --output json |
| REQ-6.2 | T-1 | --output text (default) |
| REQ-6.3 | T-1 | JSON error |
| REQ-7.1 | T-8 | Taskfile → epctl plugins build |
| REQ-7.2 | T-8 | Taskfile → epctl plugins register |
| REQ-7.3 | T-8 | Dockerfile → cmd/easyp/ |
| REQ-7.4 | T-8 | Удаление bash-скриптов |

---

## T-1: Создать shared-пакеты `internal/config/` и `internal/output/`

*_Requirements: REQ-5.1, REQ-5.3, REQ-5.4, REQ-6.1, REQ-6.2, REQ-6.3_*
*_Complexity: standard_*

**GOAL:** Создать переиспользуемые пакеты для конфигурации и вывода, доступные и серверу, и CLI.

### T-1.1 Создать `internal/config/config.go`

Создать файл `internal/config/config.go` с package `config`.
- Перенести типы из `cmd/main.go` (строки 51-98): `Config`, `Server`, `Ports`, `DBConfig`, `RegistryConfig`, `TelemetryConfig`, `WorkerPoolConfig`, `LicenseConfig`, `RateLimitConfig`.
- Экспортировать все типы (заглавные буквы уже есть, подтипы переименовать: `config` → `Config`, `server` → `Server`, `ports` → `Ports`, `dbConfig` → `DBConfig`, `registryConfig` → `RegistryConfig`, `telemetryConfig` → `TelemetryConfig`, `workerPoolConfig` → `WorkerPoolConfig`, `licenseConfig` → `LicenseConfig`, `rateLimitConfig` → `RateLimitConfig`).
- Сохранить все `env` и `yaml` теги.
- Добавить метод `Validate() error` — проверяет обязательные поля (GRPC port, DB driver).
- Добавить функцию `LoadAndValidate(path string) (*Config, []string, error)` — читает YAML с `yaml.NewDecoder` + `KnownFields(true)`, возвращает config, warnings (unknown fields), ошибку.
- Импорты: `time`, `gopkg.in/yaml.v3`, `os`.

### T-1.2 Создать `internal/output/printer.go`

Создать файл `internal/output/printer.go` с package `output`.
- Определить тип `Printer` с полями `format string` и `w io.Writer`.
- Реализовать `NewPrinter(format string, w io.Writer) *Printer` — валидирует format ("text"/"json"), default "text".
- Реализовать `Table(headers []string, rows [][]string) error`:
  - При format "text" — вывод `text/tabwriter` с табуляцией.
  - При format "json" — вывод `[]map[string]string` где ключи = headers.
- Реализовать `JSON(v any) error` — `json.NewEncoder(w).Encode(v)`.
- Реализовать `Message(msg string) error`:
  - При format "text" — `fmt.Fprintln(w, msg)`.
  - При format "json" — `{"message": msg}`.
- Реализовать `Error(err error) error`:
  - При format "json" — `{"error": err.Error()}`.
  - При format "text" — `fmt.Fprintf(w, "Error: %s\n", err)`.
- Импорты: `encoding/json`, `fmt`, `io`, `text/tabwriter`.

### T-1.3 Верификация

Запустить `go build ./internal/config/ && go build ./internal/output/`.
CRITICAL: оба пакета должны компилироваться независимо.

---

## T-2: Перенести серверный entry point в `cmd/easyp/main.go`

*_Requirements: REQ-1.1, REQ-1.3, REQ-1.4_*
*_Complexity: standard_*

**GOAL:** Перенести `cmd/main.go` → `cmd/easyp/main.go`, зафиксировать namespace метрик, добавить валидацию конфига при старте.

### T-2.1 Создать `cmd/easyp/main.go`

Скопировать `cmd/main.go` в `cmd/easyp/main.go`.
- Заменить все ссылки на локальные типы конфигурации (`config`, `server`, `ports` и т.д.) на импорты из `internal/config`:
  - `cfg := config{}` → `cfg := config.Config{}`
  - Все поля доступны через `cfg.Server.Port.GRPC` и т.д.
- Добавить `const serviceNamespace = "easyp"`.
- Заменить строку 121 `appName := filepath.Base(os.Args[0])` на `appName := serviceNamespace`.
- Удалить `"path/filepath"` из импортов (если больше нигде не используется).
- Добавить вызов `cfg.Validate()` в функцию `start()` сразу после десериализации конфига (после строки envconfig, ~строка 155), перед инициализацией компонентов:
  ```
  if err := cfg.Validate(); err != nil {
      return fmt.Errorf("config validation: %w", err)
  }
  ```
- Добавить `"github.com/easyp-tech/service/internal/config"` в импорты.

### T-2.2 Удалить `cmd/main.go`

Удалить файл `cmd/main.go`.

### T-2.3 Верификация

Запустить `go build -o easyp ./cmd/easyp/`.
CRITICAL: бинарник должен собраться без ошибок.

---

## T-3: Реализовать `PluginBuilder` и `PathFilter`

*_Requirements: REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-2.5, REQ-2.6, REQ-2.7, REQ-2.8, REQ-2.9_*
*_Complexity: complex_*

**GOAL:** Портировать логику сборки плагинов из legacy builder в переиспользуемый пакет.

### T-3.1 Создать `internal/epctl/path_filter.go`

Создать файл с package `epctl`.
- Определить `PathFilter` struct: `Group string`, `Name string`.
- `ParsePathFilter(arg string) (PathFilter, error)`:
  - Пустая строка → пустой filter (без ограничений).
  - "group" → `PathFilter{Group: "group"}`.
  - "group/name" → `PathFilter{Group: "group", Name: "name"}`.
  - Более 1 слеша → `fmt.Errorf("invalid filter: %q, expected 'group' or 'group/name'", arg)`.
- `Match(group, name string) bool`:
  - Если Group == "" → true (пустой фильтр).
  - Если Group != group → false.
  - Если Name == "" → true (group match).
  - Если Name != name → false.
  - Иначе → true.

### T-3.2 Создать `internal/epctl/builder.go`

Создать файл с package `epctl`.
- Определить типы: `PluginConfig`, `VersionEntry`, `BuildJob`, `BuildResult`, `BuildSummary` (как в design §2.3).
- Реализовать `UnmarshalYAML` для `VersionEntry` — поддержка строкового ("v1.0.0") и map-формата (`version: "v1.0.0"`).
- `NewPluginBuilder(registryDir, outputDir string, parallelism int, continueOnError bool) *PluginBuilder`.
- `DiscoverJobs(filter *PathFilter) ([]BuildJob, error)`:
  - `filepath.WalkDir(registryDir)` ищет `plugin.yaml` файлы.
  - Парсит каждый YAML в `PluginConfig`.
  - Извлекает group/name из пути (parent dirs).
  - Для каждой версии создаёт `BuildJob` с `OutputDir = outputDir/group/name/version/`.
  - Применяет `filter.Match(group, name)`.
  - Если 0 jobs — возвращает пустой slice, nil (не ошибка).
- `needsBuild(job BuildJob) bool`:
  - Проверяет `os.Stat(job.OutputDir + "/plugin")`.
  - Если файл существует → false (cached).
  - Иначе → true.
- `buildDockerArgs(job BuildJob) []string`:
  - Формирует: `["build", "--output", "type=local,dest=" + tmpDir, "--build-arg", "VERSION=" + job.Version, "--build-arg", "BINARY_NAME=" + job.Binary]`.
  - Для каждого `key, value` в `job.BuildArgs` добавляет `"--build-arg", key + "=" + value`.
  - Добавляет `job.PluginDir` (контекст сборки).
- `Build(ctx context.Context, jobs []BuildJob) (*BuildSummary, error)`:
  - Использует `errgroup.Group` с `SetLimit(parallelism)`.
  - Для каждого job: если `needsBuild` = false → cached; иначе `exec.CommandContext(ctx, "docker", buildDockerArgs(job)...)`.
  - После Docker build: `os.Rename(tmpDir/binaryName, job.OutputDir/plugin)`, `os.Chmod(0o755)`.
  - При ошибке: если `continueOnError` = true → записать в results, продолжить; иначе → `cancel()` контекста и вернуть ошибку.
  - Собрать `BuildSummary` с подсчётом Total/Built/Failed/Cached.
- Импорты: `context`, `fmt`, `os`, `os/exec`, `path/filepath`, `sync`, `time`, `golang.org/x/sync/errgroup`, `gopkg.in/yaml.v3`.

### T-3.3 Верификация

Запустить `go build ./internal/epctl/`.
CRITICAL: пакет должен компилироваться без ошибок.

---

## T-4: Создать CLI-каркас `cmd/epctl/` и cobra root

*_Requirements: REQ-1.2_*
*_Complexity: mechanical_*

**GOAL:** Создать entry point для epctl и cobra root command с глобальным `--output` флагом.

### T-4.1 Создать `internal/epctl/root.go`

Создать файл с package `epctl`.
- Определить `func NewRootCmd() *cobra.Command`:
  - `Use: "epctl"`, `Short: "EasyP Service control utility"`.
  - Persistent flag `--output` (string, default "text", valid: "text", "json").
  - Добавить subcommands: `newPluginsCmd()`, `newConfigCmd()` (пока заглушки, будут реализованы в T-5, T-6, T-7).
- Определить `func Execute()`:
  - `cmd := NewRootCmd()`, `if err := cmd.Execute(); err != nil { os.Exit(1) }`.
- Определить `func getPrinter(cmd *cobra.Command) *output.Printer`:
  - Получить значение `--output` из persistent flags.
  - Вернуть `output.NewPrinter(format, os.Stdout)`.
- Импорты: `os`, `github.com/spf13/cobra`, `github.com/easyp-tech/service/internal/output`.

### T-4.2 Создать `cmd/epctl/main.go`

Создать файл с package `main`.
- Импортировать `github.com/easyp-tech/service/internal/epctl`.
- `func main() { epctl.Execute() }`.

### T-4.3 Добавить зависимость cobra

Запустить `go get github.com/spf13/cobra`.

### T-4.4 Верификация

Запустить `go build -o epctl ./cmd/epctl/`.
Запустить `./epctl --help`.
CRITICAL: должен вывести help с подкомандами `plugins` и `config`.

---

## T-5: Реализовать команду `plugins build`

*_Requirements: REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-2.5, REQ-2.6_*
*_Complexity: standard_*

**GOAL:** Подключить PluginBuilder к cobra command.

### T-5.1 Создать `internal/epctl/plugins_build.go`

Создать файл с package `epctl`.
- `func newPluginsBuildCmd() *cobra.Command`:
  - `Use: "build [filter]"`, `Short: "Build plugins from registry"`.
  - `Args: cobra.MaximumNArgs(1)`.
  - Flags: `--registry-dir` (string, default "registry"), `--output-dir` (string, default "plugins"), `--parallelism` (int, default 3), `--continue-on-error` (bool, default false).
  - `RunE`: 
    1. Получить `printer` через `getPrinter(cmd)`.
    2. Парсить filter: если args[0] есть → `ParsePathFilter(args[0])`.
    3. Создать `NewPluginBuilder(registryDir, outputDir, parallelism, continueOnError)`.
    4. `jobs, err := builder.DiscoverJobs(&filter)`.
    5. Если 0 jobs → `printer.Message("No plugins found in registry")`, return nil.
    6. `summary, err := builder.Build(cmd.Context(), jobs)`.
    7. Вывести summary через printer: Table или JSON в зависимости от формата.
    8. Если `summary.Failed > 0 && !continueOnError` → return error.

### T-5.2 Подключить к plugins subcommand в `root.go`

В `root.go` добавить `newPluginsCmd()` функцию:
- `Use: "plugins"`, `Short: "Plugin management commands"`.
- `AddCommand(newPluginsBuildCmd())`.

NOTE: `register` и `list` будут добавлены в T-6.

### T-5.3 Верификация

Запустить `go build -o epctl ./cmd/epctl/ && ./epctl plugins build --help`.
CRITICAL: должен показать help с флагами `--registry-dir`, `--output-dir`, `--parallelism`, `--continue-on-error`.

---

## T-6: Реализовать команды `plugins register` и `plugins list`

*_Requirements: REQ-3.1, REQ-3.2, REQ-3.3, REQ-3.4, REQ-3.5, REQ-3.6, REQ-3.7, REQ-3.8, REQ-4.1, REQ-4.2, REQ-4.3, REQ-4.4, REQ-4.5_*
*_Complexity: standard_*

**GOAL:** Реализовать gRPC-взаимодействие с сервером через SDK.

### T-6.1 Добавить `CreatePlugin` в SDK

В `sdk/client.go` добавить метод:
```go
func (c *Client) CreatePlugin(
    ctx context.Context,
    group, name, version string,
    pluginConfig map[string]any,
    tags []string,
) (*generator.PluginInfo, error)
```
- Внутри: конвертировать `pluginConfig` в `*structpb.Struct` через `structpb.NewStruct(pluginConfig)`.
- Вызвать `c.genClient.CreatePlugin(ctx, &generator.CreatePluginRequest{...})`.
- Вернуть `resp.GetPlugin(), nil`.
- Использовать `c.withTimeout(ctx, c.cfg.createPluginTimeout)` — добавить `createPluginTimeout` в SDK config с default 30s.

### T-6.2 Создать `internal/epctl/plugins_register.go`

Создать файл с package `epctl`.
- `func newPluginsRegisterCmd() *cobra.Command`:
  - `Use: "register [filter]"`, `Short: "Register plugins in EasyP service"`.
  - `Args: cobra.MaximumNArgs(1)`.
  - Flags: `--addr` (string, default "localhost:8080"), `--plugins-dir` (string, default "plugins"), `--plugins-prefix` (string, default "/plugins").
  - `RunE`:
    1. Парсить filter.
    2. Сканировать `plugins-dir`: найти все директории вида `{group}/{name}/{version}/plugin`.
    3. Применить filter.
    4. Создать `sdk.NewClient(addr, sdk.WithInsecure())`.
    5. Для каждого плагина:
       - Сформировать `config = map[string]any{"command": pluginsPrefix + "/" + group + "/" + name + "/" + version + "/plugin"}`.
       - Вызвать `client.CreatePlugin(ctx, group, name, version, config, nil)`.
       - Если `status.Code(err) == codes.AlreadyExists` → warning, skipped++, continue.
       - Если другая ошибка → вернуть ошибку с exit code 1.
       - Иначе → registered++.
    6. Вывести отчёт через printer: registered, skipped, total.

### T-6.3 Создать `internal/epctl/plugins_list.go`

Создать файл с package `epctl`.
- `func newPluginsListCmd() *cobra.Command`:
  - `Use: "list"`, `Short: "List registered plugins"`.
  - Flags: `--addr` (string, default "localhost:8080"), `--group`, `--name`, `--version`, `--tags` (string slice).
  - `RunE`:
    1. Создать `sdk.NewClient(addr, sdk.WithInsecure())`.
    2. Сформировать `sdk.PluginFilter{Group, Name, Version, Tags}`.
    3. `plugins, err := client.ListPlugins(ctx, filter)`.
    4. Если connection error → `fmt.Errorf("cannot connect to %s: %w", addr, err)`.
    5. Если 0 плагинов → `printer.Message("No plugins found")`, exit 0.
    6. Вывести таблицу через printer: group, name, version, tags, created_at.

### T-6.4 Подключить к plugins subcommand

В `root.go` обновить `newPluginsCmd()`: добавить `newPluginsRegisterCmd()` и `newPluginsListCmd()`.

### T-6.5 Верификация

Запустить `go build -o epctl ./cmd/epctl/`.
Запустить `./epctl plugins register --help` и `./epctl plugins list --help`.
CRITICAL: обе команды должны показывать корректный help.

---

## T-7: Реализовать команду `config validate`

*_Requirements: REQ-5.1, REQ-5.2, REQ-5.3, REQ-5.4, REQ-5.5_*
*_Complexity: mechanical_*

**GOAL:** Подключить валидацию конфига из shared-пакета к cobra command.

### T-7.1 Создать `internal/epctl/config_validate.go`

Создать файл с package `epctl`.
- `func newConfigValidateCmd() *cobra.Command`:
  - `Use: "validate <path>"`, `Short: "Validate service config YAML"`.
  - `Args: cobra.ExactArgs(1)`.
  - `RunE`:
    1. Получить printer.
    2. `cfg, warnings, err := config.LoadAndValidate(args[0])`.
    3. Если err != nil → `printer.Error(err)`, return err.
    4. Если len(warnings) > 0 → вывести каждый warning.
    5. Если `cfg.Validate()` != nil → вывести ошибки.
    6. Иначе → `printer.Message("Config is valid")`.

### T-7.2 Подключить к config subcommand

В `root.go` добавить `newConfigCmd()`:
- `Use: "config"`, `Short: "Configuration management"`.
- `AddCommand(newConfigValidateCmd())`.

### T-7.3 Верификация

Запустить `go build -o epctl ./cmd/epctl/ && ./epctl config validate --help`.
CRITICAL: должен показать help с обязательным аргументом `<path>`.

---

## T-8: Обновить инфраструктуру (Dockerfile, Taskfile, удалить скрипты)

*_Requirements: REQ-7.1, REQ-7.2, REQ-7.3, REQ-7.4_*
*_Complexity: mechanical_*

**GOAL:** Обновить сборочную инфраструктуру и удалить замещённые bash-скрипты.

### T-8.1 Обновить `Dockerfile`

В `Dockerfile` заменить:
- `go build -o /easyp ./cmd/` → `go build -o /easyp ./cmd/easyp/`.
- Убедиться что `COPY` и `ENTRYPOINT` указывают на `/easyp`.

### T-8.2 Обновить `Taskfile.yml`

- Таска `build-plugins`: заменить `./build-plugins.sh` на `go run ./cmd/epctl plugins build`.
- Таска `register-plugins`: заменить `./register-plugins.sh` на `go run ./cmd/epctl plugins register`.

### T-8.3 Удалить bash-скрипты

Удалить файлы:
- `build-plugins.sh`
- `register-plugins.sh`

### T-8.4 Верификация (GATE)

Запустить:
1. `go build -o easyp ./cmd/easyp/` — серверный бинарник.
2. `go build -o epctl ./cmd/epctl/` — CLI бинарник.
3. `./epctl --help` — help выводится.
4. `./epctl plugins build --help` — help выводится.
5. `./epctl plugins register --help` — help выводится.
6. `./epctl plugins list --help` — help выводится.
7. `./epctl config validate --help` — help выводится.

CRITICAL: все 7 проверок должны пройти без ошибок.
