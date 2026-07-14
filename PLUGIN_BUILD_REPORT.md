# Отчёт: почему не все плагины из `registry/` собираются

**Дата:** 2026-07-14
**Команда:** `easyp-svc plugins build registry --output plugins --parallel 12`
**Окружение:** Docker 29.6.1 (driver: docker, buildx), локальная машина агента

## TL;DR

Из **1744** объявленных версий плагинов в `registry/`:

| Статус | Кол-во | Комментарий |
|---|---|---|
| ✅ Уже собрано и закэшировано | 1265 | бинарник `plugin` уже лежит в `plugins/...` |
| ⏭️ Осознанно пропущено (`skip: true`) | 9 | известные битые версии, помечены раньше |
| ❌ Не собираются | ~470 | требуют внимания |

Из ~470 несобранных версий подавляющее большинство (**~450**) падают всего по **двум системным причинам**, связанным с несовместимостью Bazel с текущим Docker-окружением, а не по вине конкретных версий плагинов. Ещё несколько версий падают по частным, специфичным для конкретного Dockerfile причинам. Отдельно: свежий коммит `d4d5937 "fix generator"` попытался исправить одну из системных проблем для `grpc/cpp`, но **исправление оказалось синтаксически неверным** и теперь ломает сборку `grpc/cpp` ещё раньше, чем до "исправления".

Важно: большая часть файлов `build.log`, лежащих сейчас в `plugins/**/build.log` (492 из 521) — это **не настоящие ошибки сборки**, а следы того, что предыдущий запуск `plugins build` был прерван (SIGINT/SIGTERM) до того, как до этих версий дошла очередь. В логе там только `Error: context canceled` / `signal: killed`, реальный docker build не запускался. Их нужно перепрогнать, а не разбирать как баги.

---

## Как проверялось

1. `go build -o /tmp/easyp-svc ./cmd/easyp-svc/` — собрана CLI-утилита из текущего HEAD.
2. `plugins build registry --output plugins --dry-run --non-interactive` — получен полный план (1744 версии, 470 к сборке, 1265 в кэше, 9 skip).
3. Разобраны существующие `plugins/**/build.log`, оставшиеся от предыдущих запусков (1719 файлов). Разделены на:
   - "мусорные" (≤ 2 строки, `context canceled` / `signal: killed`) — 492 шт., не показательны;
   - содержательные (есть реальный вывод docker build) — 29 шт.
4. Для каждой из **17 уникальных ещё не собранных групп плагинов** (`grpc/{cpp,csharp,objc,php,python,ruby,web,go}`, `protocolbuffers/{cpp,csharp,java,kotlin,objc,php,pyi,python,ruby}`) выполнена контрольная пересборка одной версии с `--force`, чтобы получить свежий, не "прерванный" лог и убедиться, что причина ошибки воспроизводится стабильно.

---

## Причина №1 (системная): Bazel sandbox не работает в текущем Docker-окружении

**Кого касается:** `grpc/csharp`, `grpc/objc`, `grpc/php`, `grpc/python`, `grpc/ruby` (по ~12 версий каждая, итого ~59 версий). До недавнего коммита сюда же относился и `grpc/cpp` (см. причину №2).

Все эти Dockerfile практически идентичны и запускают:
```
RUN bazelisk ${BAZEL_OPTS} build //src/compiler:grpc_xxx_plugin
```
без каких-либо доп. флагов (`BAZEL_OPTS="--host_jvm_args=-Djava.net.preferIPv4Stack=true"`).

Реальная ошибка (пример, `grpc/csharp:v1.63.0`):
```
ERROR: Failed to initialize sandbox: /build/.cache/bazel/_bazel_nobody/.../sandbox/_moved_trash_dir
  -> .../sandbox/stale-trash-0 (Invalid cross-device link)
```

Bazel по умолчанию собирает каждое действие в изолированной sandbox-директории и в конце пытается **атомарно переименовать** временный "trash"-каталог. `rename(2)` не может работать между разными файловыми системами/точками монтирования ("cross-device link"). В используемом Docker buildx-окружении рабочая директория `/build` и внутренний tmpfs/overlay-слой, судя по всему, оказываются на разных устройствах, поэтому такое переименование гарантированно падает на **любой** версии, для любого языка — это не баг конкретной версии grpc, а несовместимость дефолтной bazel-песочницы с этим окружением сборки.

**Стандартное решение:** отключить sandboxing и использовать `--spawn_strategy=local` (или `standalone`), что для Bazel корректно означает: выполнять команды напрямую в рабочей директории без построения sandbox-дерева.

---

## Причина №2 (регрессия из последнего коммита): флаг `--spawn_strategy=local` передан не той команде bazel

Коммит `d4d5937 "fix generator"` попытался исправить причину №1 для `grpc/cpp/Dockerfile`:
```diff
-ARG BAZEL_OPTS="--host_jvm_args=-Djava.net.preferIPv4Stack=true"
+ARG BAZEL_OPTS="--host_jvm_args=-Djava.net.preferIPv4Stack=true --spawn_strategy=local"
```
Однако в самом Dockerfile `BAZEL_OPTS` подставляется **перед** словом `build`:
```
RUN bazelisk ${BAZEL_OPTS} build //src/compiler:grpc_cpp_plugin
```
У Bazel есть два разных класса флагов:
- **startup options** — идут до имени команды (`bazel <startup-flags> build ...`). Сюда относится `--host_jvm_args`.
- **command/build options** — идут после имени команды (`bazel build <build-flags> ...`). Сюда относится `--spawn_strategy`.

Поскольку `--spawn_strategy=local` оказался в позиции startup-опции, Bazel завершает работу мгновенно, ещё до старта сборки:
```
[FATAL ...] Unknown startup option: '--spawn_strategy=local'.
```
Проверено контрольной пересборкой `grpc/cpp:v1.66.2 --force` — падает именно так уже на первом `bazelisk` шаге. То есть текущее "исправление" не просто не помогло — оно **сделало сборку `grpc/cpp` ещё более сломанной**, чем до коммита (раньше падало на середине сборки из-за sandbox, теперь падает мгновенно из-за неверного флага).

**Как правильно чинить (и причину №1, и №2 разом) во всех затронутых Dockerfile:**
```
RUN bazelisk ${BAZEL_STARTUP_OPTS} build ${BAZEL_BUILD_OPTS} //src/compiler:grpc_xxx_plugin
```
где
```
ARG BAZEL_STARTUP_OPTS="--host_jvm_args=-Djava.net.preferIPv4Stack=true"
ARG BAZEL_BUILD_OPTS="--spawn_strategy=local"
```
и применить это единообразно к `grpc/cpp`, `grpc/csharp`, `grpc/objc`, `grpc/php`, `grpc/python`, `grpc/ruby`, `grpc/web`.

---

## Причина №3 (системная, самая массовая): новая версия Bazel (9.2.0) отказывается стартовать из `protocolbuffers/*`

**Кого касается:** `protocolbuffers/cpp`, `csharp`, `java`, `kotlin`, `objc`, `php`, `pyi`, `python`, `ruby` — от 41 до 60 версий на группу, это **основная масса** несобранных плагинов (более 350 версий).

Реальная ошибка (одинакова во всех группах):
```
ERROR: The repo contents cache [/build/.cache/bazel/_bazel_nobody/cache/repos/v1/contents]
  is inside the main repo [/build]. This can cause spurious failures.
  Disable the repo contents cache with `--repo_contents_cache=`,
  or specify `--repo_contents_cache=<path outside the main repo>`.
```

Здесь `bazelisk` без `.bazelversion`/пиннинга подтягивает актуальный релиз Bazel (в момент проверки — **9.2.0**), а начиная примерно с Bazel 8/9 появилась защитная проверка: если директория "repo contents cache" (по умолчанию `$HOME/.cache/bazel/...`) находится **внутри** самого репозитория/рабочей директории, Bazel прерывает работу, чтобы не допустить порчи собственного кэша. В этих Dockerfile `USER nobody` с `HOME=/build`, а сборка идёт из `/build`, — то есть кэш Bazel гарантированно оказывается внутри рабочего каталога.

Это регрессия окружения "снаружи": Dockerfile когда-то работал, но с обновлением Bazel до 8.x/9.x (bazelisk всегда тянет самую свежую версию, если не задано иное) поведение изменилось.

**Решение:** зафиксировать версию Bazel (например, через `USE_BAZEL_VERSION=7.x` как уже сделано в `grpc/web/Dockerfile`) **и/или** явно задать `--repo_contents_cache=/tmp/bazel-repo-cache` (путь вне `/build`) в build-опциях, применить это ко всем 8 `protocolbuffers/*` Dockerfile.

---

## Причина №4 (индивидуальная): `protocolbuffers/csharp` — невалидная версия NuGet-пакета

Помимо причины №3, у `protocolbuffers/csharp` есть вторая, независимая проблема:
```
/usr/share/dotnet/sdk/8.0.421/NuGet.targets(174,5): error :
  'v21.10' is not a valid version string. (Parameter 'value') [/build/build.csproj]
```
Тег версии (`v21.10`) передаётся в `build.csproj` с префиксом `v`, а NuGet ожидает чистый SemVer (`21.10`). В других языковых Dockerfile версия перед использованием в подобных контекстах очищается через `${VERSION#v}`, а здесь — нет. Нужно убрать префикс `v` перед подстановкой версии в `build.csproj`/`dotnet restore`.

---

## Причина №5 (индивидуальная): `grpc/go` — патч не применяется к старым версиям (`v1.2.0`, вероятно и `v1.3.0`)

```
error: patch failed: cmd/protoc-gen-go-grpc/grpc.go:135
error: cmd/protoc-gen-go-grpc/grpc.go: patch does not apply
```
Dockerfile накладывает единый `separate-package.patch` на исходники `grpc-go`, но структура кода `cmd/protoc-gen-go-grpc` в старых релизах (`v1.2.0`) отличается от той, под которую написан патч. Это версионная несовместимость самого патча, а не проблема окружения. Нужно либо исключить версии `v1.2.0`/`v1.3.0` через `skip: true` (по аналогии с уже пропущенными `v1.4.0`, `v1.6.0`, `v1.6.1`), либо подготовить отдельный патч под старый layout.

---

## Причина №6 (индивидуальная): `grpc/web:v1.4.2` — в релизе нет исходного архива

```
curl: (22) The requested URL returned error: 404
  (grpc-web-source-1.4.2.tar.gz)
```
Проверено через GitHub API (`/repos/grpc/grpc-web/releases/tags/1.4.2`): релиз `1.4.2` существует, но публикует только собранные бинарники (`protoc-gen-grpc-web-1.4.2-{os}-{arch}`), а не source-tarball `grpc-web-source-*.tar.gz`. Такой ассет появился в релизах grpc-web начиная примерно с `v2.0.0`. Т.е. для `v1.4.2` (и, вероятно, `v1.5.0`) текущая схема сборки "из исходников через Bazel" в принципе неприменима — этим версиям нужен либо `skip: true`, либо отдельный путь сборки (скачивание готового бинарника вместо сборки из исходников).

---

## Что уже было исправлено раньше (не требует внимания)

`bufbuild/connect-kotlin` (`v0.1.1`, `v0.1.2`, `v0.1.4`) и `connectrpc/python` (`v0.4.2`, `v0.10.0`) — в `registry/**/plugin.yaml` уже стоит `skip: true` для этих версий. Оставшиеся `build.log` в `plugins/` для них — просто старые файлы от момента до простановки `skip`, не активная проблема.

---

## Рекомендации (по приоритету)

1. **Не считать 492 "context canceled/killed" логa реальными падениями.** Перед следующим анализом — перепрогнать `plugins build registry --output plugins` полностью и дать ему завершиться без прерывания (или разбить по `--filter` на части), чтобы получить достоверную картину.
2. **Откатить/поправить регрессию из `d4d5937`** — исправить порядок опций bazel в `registry/grpc/cpp/Dockerfile` (см. причину №2), иначе `grpc/cpp` сейчас в худшем состоянии, чем до "фикса".
3. **Разделить `BAZEL_OPTS` на startup/build-опции и включить `--spawn_strategy=local`** одинаково во всех Dockerfile семейства `grpc/{cpp,csharp,objc,php,python,ruby,web}` — закрывает причину №1/№2 разом (~60+ версий).
4. **Запинить версию Bazel** (`USE_BAZEL_VERSION`, как в `grpc/web`) и/или задать `--repo_contents_cache` вне `/build` во всех `protocolbuffers/*` Dockerfile — закрывает причину №3, самую массовую (350+ версий).
5. Убрать префикс `v` перед версией в `protocolbuffers/csharp/build.csproj` (причина №4).
6. Пометить `grpc/go:v1.2.0`(`v1.3.0`?) и `grpc/web:v1.4.2`(`v1.5.0`?) как `skip: true`, либо завести отдельные Dockerfile-ветки под старые релизы (причины №5, №6).

После пп. 3–4 ожидается, что подавляющее большинство из ~470 несобранных версий (>400) начнут собираться успешно, поскольку они падают по одной из двух системных причин, а не по индивидуальным причинам.
