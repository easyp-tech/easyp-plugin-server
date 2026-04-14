# Исследование: Перенос аудита в бизнес-логику

## Намерение

Текущая механика аудит-логов реализована как gRPC interceptor (`AuditInterceptor`), который перехватывает все RPC-вызовы и формирует `AuditEntry` на уровне транспорта. Проблема: interceptor не имеет полного бизнес-контекста, вынужден парсить protobuf-ответы для извлечения метаданных, и не покрывает MCP-эндпоинт. Цель — перенести логику формирования и отправки аудит-записей в core layer, чтобы аудит стал частью бизнес-логики.

## Исследование

### Текущая архитектура аудита

**Поток данных (as-is):**
```
gRPC Request → interceptor chain → [AuditInterceptor] → handler → response
                                          │
                                          ▼
                                    chan AuditEntry (cap 1000)
                                          │
                                          ▼
                                    Worker.Run() → Store.Save() → PostgreSQL
```

**Файлы:**

| Файл | Роль |
|------|------|
| `internal/api/audit_interceptor.go` | gRPC interceptor — формирует `AuditEntry`, отправляет в канал |
| `internal/adapters/audit/worker.go` | Фоновый воркер — читает из канала, пишет в БД с трейсингом |
| `internal/adapters/audit/audit.go` | `Store` — реализует `core.AuditLog` (INSERT в PostgreSQL) |
| `internal/core/domain.go` | Доменные типы: `AuditEntry`, `AuditLog` интерфейс, константы операций |
| `migrate/3.audit_log.sql` | Схема таблицы `audit_log` |

### Проблемы текущего подхода

1. **`extractMetadata` — code smell.** Interceptor делает type assertion на protobuf-ответ (`*generator.GenerateCodeResponse`, `*generator.PluginsResponse`) чтобы достать `file_count` / `plugin_count`. Это бизнес-данные, которых у transport layer быть не должно.

2. **MCP не покрыт.** MCP-эндпоинт (`/mcp`, порт 8083) обрабатывается через HTTP, минуя gRPC interceptor chain. Инструменты `plugins_list`, `generate_code`, `easyp_config_describe` не аудитируются.

3. **Plugin name extraction дублирует бизнес-логику.** Interceptor разбирает request для извлечения `plugin_name` через type switch по protobuf-типам — это знание о структуре запросов, которое принадлежит core.

4. **Нет гранулярности.** Interceptor видит только "запрос начался / завершился". Нельзя аудитировать частичные операции, retry, или промежуточные шаги.

### Core layer — что есть сейчас

`core.Core` содержит бизнес-методы (`Generate`, `ListPlugins`, `CreatePlugin`, `UpdatePlugin`, `DeletePlugin`). Ни один из них не знает про аудит. Core зависит от:
- `Metrics` — метрики (уже вызывается из core)  
- `Registry` — доступ к плагинам
- `FeatureGate` — лицензионные проверки

`AuditLog` интерфейс определён в core, но используется только адаптером напрямую.

### Caller IP

Сейчас interceptor извлекает IP через `peer.FromContext(ctx)`. В цепочке interceptor'ов уже есть `realip` (первый в цепочке), который кладёт реальный IP в контекст. При переносе в core нужно прокидывать caller IP через context — стандартный паттерн. [ASSUMPTION: realip interceptor уже записывает IP в context, и core сможет его прочитать через вспомогательную функцию]

### Tracing decorator

Существует паттерн Decorator для трейсинга: `TracingCore`, `TracingRegistry`, `TracingPlugin`. Аудит может следовать тому же паттерну — но это дизайн-решение для следующей фазы.

## Инструментарий сборки

- **Оркестратор:** Task (go-task), `Taskfile.yml`
- **Тесты:** `go test ./...`
- **Сборка:** `go build ./cmd/main.go` / `task build` (Docker)
- **Линтер:** `golangci-lint run` 
- **Кодогенерация:** `easyp --cfg easyp.yaml generate` (proto → Go stubs + MCP bindings)
- **Источник:** `Taskfile.yml`

## Рассмотренные варианты

### Вариант A: Прямой вызов из Core методов

Core получает зависимость на канал `chan<- AuditEntry` (или обёртку). Каждый бизнес-метод сам формирует `AuditEntry` и отправляет в канал.

```go
func (c *Core) Generate(ctx context.Context, req GenerateCodeRequest) (*GenerateCodeResponse, error) {
    // ... бизнес-логика ...
    resp, err := plugin.Generate(ctx, req.Payload)
    
    c.audit(ctx, AuditEntry{
        OperationType: OperationGenerateCode,
        PluginName:    req.PluginName,
        Status:        statusFromErr(err),
        Metadata:      map[string]any{"file_count": len(resp.Payload.GetFile())},
        // ...
    })
    
    return resp, err
}
```

**Плюсы:**
- Полный бизнес-контекст — формирует entry с точными данными
- Покрывает все транспорты (gRPC, MCP, будущие)
- Тестируется юнит-тестами
- Прямой, понятный код

**Минусы:**
- Boilerplate в каждом методе
- Core знает про audit — дополнительная зависимость
- Duration нужно измерять вручную в каждом методе

**Сложность:** Низкая

### Вариант B: Decorator pattern (как TracingCore)

Создать `AuditCore` — обёртку вокруг `CoreService`, которая добавляет аудит-логику:

```go
type AuditCore struct {
    next    core.CoreService
    auditCh chan<- core.AuditEntry
}

func (a *AuditCore) Generate(ctx context.Context, req core.GenerateCodeRequest) (*core.GenerateCodeResponse, error) {
    start := time.Now()
    resp, err := a.next.Generate(ctx, req)
    a.send(ctx, core.AuditEntry{...})
    return resp, err
}
```

**Плюсы:**
- Core остаётся чистым — не знает про аудит
- Следует установленному паттерну проекта (TracingCore)
- Единое место для аудит-логики
- Легко включить/выключить (не оборачивать)
- Duration измеряется естественно (wrap вызова)

**Минусы:**
- Доступ к бизнес-данным ограничен: decorator видит request/response, но не промежуточные шаги
- При добавлении нового метода в `CoreService` нужно обновить и decorator

**Сложность:** Низкая

## Ограничения и риски

- **Миграции не нужны** — схема `audit_log` не меняется, меняется только место формирования записей
- **Обратная совместимость** — формат `AuditEntry` и таблица остаются прежними. Нет breaking changes
- **Caller IP** — нужен вспомогательный пакет/функция для извлечения IP из context. `realip` middleware уже кладёт его в context — нужно проверить формат
- **Worker и Store остаются as-is** — меняется только producer (кто формирует entry), consumer не меняется
- **MCP покрытие** — при Варианте B MCP автоматически покрывается, т.к. MCP handler вызывает `CoreService` интерфейс

## Рекомендация

**Вариант A (прямой вызов из Core методов)** — лучший выбор:

1. **Полный бизнес-контекст** — каждый метод core знает все промежуточные данные: реальный plugin info после парсинга, количество сгенерированных файлов, детали retry, payload size. Decorator и interceptor ограничены границей request/response.
2. **Расширяемость** — можно аудитировать любые внутренние шаги без костылей типа `extractMetadata`.
3. **Покрытие MCP** — MCP handler вызывает `CoreService`, аудит в core покрывает оба транспорта.
4. **Тестируемость** — юнит-тесты без gRPC стека, проверяем конкретные AuditEntry.
5. **Boilerplate минимален** — вспомогательный метод `c.audit.Send(ctx, entry)` с non-blocking отправкой в канал, 5-7 строк аудита в каждом методе.

Decorator (Вариант B) был отклонён: хотя он следует паттерну `TracingCore`, он ограничивает аудит тем же окном request/response, что и текущий interceptor. Для детального аудита с бизнес-контекстом этого недостаточно.

[ASSUMPTION: MCP handler вызывает тот же `CoreService` интерфейс, и аудит в core автоматически покроет MCP-вызовы]

[ASSUMPTION: caller IP будет доступен через context (прокидывается realip middleware)]

## Границы скоупа

- **Must-have (v1):**
  - Добавление зависимости на `chan<- AuditEntry` (или обёртку) в `core.Core`
  - Прямые вызовы аудита из каждого бизнес-метода (`Generate`, `ListPlugins`, `CreatePlugin`, `UpdatePlugin`, `DeletePlugin`)
  - Удаление `AuditInterceptor` из gRPC цепочки
  - Вспомогательная функция для извлечения caller IP из context
  - Прокидывание audit-канала в `core.New()` в `cmd/main.go`
  - Тесты на аудит-записи в каждом методе core

- **Deferred (v2):**
  - Расширение `AuditEntry` новыми полями (request size, trace ID)
  - Аудит MCP-специфичных инструментов (`easyp_config_describe`)
  - Настраиваемый фильтр операций (какие методы аудитировать)

- **Needs spike:** нет

## Предположения и открытые вопросы

**Предположения:**
- [ASSUMPTION: `realip` interceptor записывает IP в context через стандартный механизм, читаемый без gRPC-зависимости]
- [ASSUMPTION: MCP handler использует тот же `CoreService` интерфейс]
- [ASSUMPTION: добавление зависимости на audit в Core — допустимая цена за полный бизнес-контекст в аудит-записях]

**Открытые вопросы:** нет
