# Глубокое расследование: почему сборка плагинов всё ещё падает после фиксов

**Дата:** 2026-07-16
**Контекст:** после `PLUGIN_BUILD_REPORT.md` (коммит `f7e2587`) в ветку прилетели фиксы
(`5b7850b`, `ff4c098`, `a78fe3b`, `74e59f8`, `0469278`, `4fd0005`, `66c2b1f`, `b242f03`, `099b2f0`),
адресующие ровно то, что было описано в первом отчёте. Этот документ — повторное,
более глубокое расследование: **по одному репрезентативному примеру на каждый блок**
(`ruby`, `pyi`, `php`, `csharp`, `cpp`, `go`, `web`), с полными логами и проверкой гипотез
через пересборку и внешние источники (Bazel docs, GitHub Releases API).

## TL;DR

Прежние фиксы **устранили именно те симптомы**, которые были описаны в первом отчёте, но
не решили задачу до конца — почти везде за первой ошибкой сразу обнаружилась **вторая,
более глубокая**. Итог: `dry-run` по-прежнему показывает **466 версий к сборке** (было 470),
т.е. реально ничего массово не заработало, только сместилась точка отказа.

| Блок | Что было (1‑й отчёт) | Что исправили | Что ломается СЕЙЧАС |
|---|---|---|---|
| `grpc/{cpp,csharp,objc,php,python,ruby}` | Bazel sandbox EXDEV | добавили `--spawn_strategy=local` + `--repo_contents_cache` | **`--repo_contents_cache` — Unrecognized option** (флаг не существует в Bazel 7.1.0, который `grpc` пинит сам) |
| `protocolbuffers/{cpp,csharp,java,kotlin,objc,php,pyi,python,ruby}` | Bazel 9 repo-cache-in-workspace | вынесли cache-path наружу, снят WORKSPACE-конфликт | **старые релизы protobuf используют legacy WORKSPACE, а Bazel 9.2.0 (тянется bazelisk без пина) его больше не поддерживает** → `no repository visible as '@bazel_skylib'/'@rules_proto'/'@rules_cc'/...` |
| `protocolbuffers/csharp` (доп.) | невалидная `v`-версия в NuGet | добавили `/p:Version=21.10` и т.п. | ✅ реально пофиксили, но упирается в ту же WORKSPACE-проблему выше |
| `grpc/go` | `v1.2.0` не патчится | `skip: true` для `v1.2.0`/`v1.3.0` | **патч жёстко привязан к `v1.6.2`** (literal-контекст с `const version = "1.6.2"`), поэтому падает и на `v1.5.1`, который не был помечен skip |
| `grpc/web` | нет source-тарбола для `v1.4.2` | `skip: true` для `v1.4.2`/`v1.5.0` | ✅ полностью решено |

Также по ходу расследования обнаружены два **инфраструктурных фактора окружения**, не связанных с конкретными Dockerfile, но влияющих на воспроизводимость:
- Диск хост-машины заполнен на **100%** (свободно ~930 МБ из 915 ГБ), при этом Docker build cache занимает 500 ГБ (32 ГБ можно освободить).
- Периодически (не всегда) `curl` внутри сборки не может достучаться до `release-assets.githubusercontent.com` за разумное время (таймаут ~5 мин), хотя повторная попытка через секунды проходит за 1 секунду — то есть сеть иногда "подвисает", но не заблокирована постоянно.

---

## Блок 1. `grpc/ruby` (представитель Bazel-семейства `grpc/*`)

**Версия:** `v1.63.0`, пересобрана заново командой `plugins build registry --filter 'grpc/ruby:v1.63.0' --force`.

Текущий Dockerfile (после `5b7850b`):
```dockerfile
ARG BAZEL_STARTUP_OPTS="--host_jvm_args=-Djava.net.preferIPv4Stack=true"
ARG BAZEL_BUILD_OPTS="--spawn_strategy=local --repo_contents_cache=/tmp/bazel-repo-cache"
...
RUN bazelisk ${BAZEL_STARTUP_OPTS} build ${BAZEL_BUILD_OPTS} //src/compiler:grpc_plugin_support
```

Свежий лог:
```
#14 [build 7/9] RUN bazelisk --host_jvm_args=-Djava.net.preferIPv4Stack=true build --spawn_strategy=local --repo_contents_cache=/tmp/bazel-repo-cache //src/compiler:grpc_plugin_support
#14 0.152 Downloading https://releases.bazel.build/7.1.0/release/bazel-7.1.0-linux-x86_64...
#14 6.401 INFO: Running bazel wrapper ... bazel version 7.1.0 will be used instead of system-wide bazel installation.
#14 18.68 ERROR: --repo_contents_cache=/tmp/bazel-repo-cache :: Unrecognized option: --repo_contents_cache=/tmp/bazel-repo-cache
```

**Почему.** `grpc/grpc` пинит собственную версию Bazel через `.bazelversion` в своём репозитории — bazelisk видит это и подтягивает **Bazel 7.1.0**, что подтверждается строкой `bazel version 7.1.0 will be used`. Флаг `--repo_contents_cache` появился в Bazel **8.x** (и обязателен к рассмотрению в 9.x) — в 7.1.0 он не зарегистрирован как валидная опция, поэтому Bazel сразу завершает работу с `Unrecognized option`.

Фикс `5b7850b` был написан "по образцу" фикса для `protocolbuffers/*` (где действительно нужен `--repo_contents_cache`, потому что там Bazel не пиннится и используется 9.x), но **скопирован туда, где Bazel-версия другая** — сам `grpc` репозиторий её жёстко фиксирует, и туда этот флаг не подходит.

Идентичная картина подтверждена для `grpc/csharp`, `grpc/objc`, `grpc/php`, `grpc/python` — все используют одинаковый паттерн Dockerfile и все падают на этой же строке `Unrecognized option`.

**Как чинить:** убрать `--repo_contents_cache` из `BAZEL_BUILD_OPTS` для всех `grpc/*` Dockerfile (он там не нужен и не поддерживается) — оставить только `--spawn_strategy=local`, который и был реальным фиксом sandbox-проблемы из первого отчёта.

---

## Блок 2. `protocolbuffers/pyi` (представитель Bazel-семейства `protocolbuffers/*`)

**Версия:** `v21.10`, пересобрана дважды (первый прогон упал по сетевому таймауту, второй — за 21 секунду с другой ошибкой, см. ниже).

Текущий Dockerfile (после `ff4c098`) корректно передаёт startup/build опции раздельно и с `--repo_contents_cache` вне `/build`. В отличие от `grpc/*`, здесь `bazelisk` **не пиннит версию** и подтягивает актуальную — подтверждено логом:
```
#15 9.871 Starting local Bazel server (9.2.0) and connecting to it...
```

Ошибка, которая возникает СРАЗУ после того, как `--repo_contents_cache` был принят (т.е. предыдущий фикс сработал верно на этом этапе):
```
#15 16.58 ERROR: Skipping '//:protoc_lib': error loading package '':
  at /build/protobuf.bzl:1:6: Unable to find package for
  @@[unknown repo 'bazel_skylib' requested from @@]//lib:versions.bzl:
  The repository '@@[unknown repo 'bazel_skylib' requested from @@]' could not be resolved:
  No repository visible as '@bazel_skylib' from main repository.
```

**Почему.** Согласно официальной документации Bazel (bazel.build/external/migration):

> "The WORKSPACE file is already disabled in Bazel 8 (late 2024) and will be removed in Bazel 9 (late 2025)."

Исходники `protobuf v21.10` датированы концом 2022 года и объявляют внешние зависимости (`bazel_skylib`, и для других плагинов — `rules_proto`, `rules_cc`, и т.д.) через **legacy `WORKSPACE`**-механизм (`http_archive` в файле `WORKSPACE`). Bazel 9.2.0, который сейчас реально используется (т.к. версия не запинена), **больше не читает `WORKSPACE` вообще** — отсюда `No repository visible as '@bazel_skylib'`.

Проверено на ещё двух представителях того же блока, чтобы убедиться, что это системная, а не единичная проблема:

- `protocolbuffers/kotlin:v21.10` → `No repository visible as '@rules_cc'`
- `protocolbuffers/php:v21.10` → `No repository visible as '@rules_proto'`
- `protocolbuffers/csharp:v21.10` → `No repository visible as '@bazel_skylib'` (после того как NuGet-фикс из `ff4c098` уже отработал корректно — версия `21.10` теперь принимается `dotnet restore` через `/p:Version=21.10`, эта часть чинена верно)

Разное имя "первого недостающего репозитория" в каждом случае — ожидаемо: это просто первая внешняя зависимость, которую Bazel пытается разрешить при загрузке конкретного `BUILD`/`.bzl` файла данного плагина, но корень один и тот же — **WORKSPACE больше не работает**.

**Важно:** предыдущий фикс (`ff4c098`) был необходим (без него ошибка `repo contents cache is inside the main repo` возникала бы раньше и маскировала эту), но недостаточен — старые версии protobuf (примерно все релизы до перехода на bzlmod/`MODULE.bazel`, ориентировочно < v22–23) в принципе не могут собраться под Bazel 9.x без дополнительных мер.

**Как чинить (варианты):**
1. Запинить версию Bazel через `USE_BAZEL_VERSION` на что-то ≤ 7.x (последняя версия с ещё включённым по умолчанию WORKSPACE) для всех `protocolbuffers/*` Dockerfile — самый надёжный вариант для старых релизов protobuf.
2. Либо явно добавить флаг `--enable_workspace` в `BAZEL_STARTUP_OPTS` — но в Bazel 9 поддержка WORKSPACE обещана быть **удалена**, а не просто выключена по умолчанию, так что этот флаг может не сработать (не проверялось из-за большого времени сборки каждой итерации, см. раздел "Что не успел проверить").
3. Ограничить набор версий `protocolbuffers/*` только теми релизами, что уже перешли на bzlmod (`MODULE.bazel`), и пометить остальные `skip: true` — самый быстрый, но самый разрушительный по охвату версий вариант.

Вариант 1 предпочтителен: он же используется в `grpc/web/Dockerfile` (`ENV USE_BAZEL_VERSION=7.6.1`) как прецедент в этом же репозитории.

---

## Блок 3. `protocolbuffers/csharp` — точечная проверка уже сделанного NuGet-фикса

Отдельно проверено, что фикс NuGet-версии из `ff4c098` **реально работает**:
```
#14 [dotnetrestore 4/4] RUN mkdir /nuget && dotnet restore --packages /nuget /p:Version=21.10 /p:AssemblyVersion=21.10 /p:FileVersion=21.10 /p:PackageVersion=21.10
```
Ошибки `'v21.10' is not a valid version string` больше нет — `dotnet restore` в этом прогоне прошёл успешно. Падение происходит позже и по другой причине (Блок 2 — `bazel_skylib`/WORKSPACE). То есть у `protocolbuffers/csharp` **было два независимых бага, один уже закрыт, второй — общий для всей группы `protocolbuffers/*`.**

---

## Блок 4. `grpc/go` — патч жёстко привязан к одной версии

**Версия:** `v1.5.1` — специально взята НЕ из списка `skip`, чтобы проверить гипотезу "остальные версии, кроме явно помеченных, работают".

```
#12 [build 6/8] RUN git apply separate-package.patch
#12 0.185 error: patch failed: cmd/protoc-gen-go-grpc/main.go:45
#12 0.185 error: cmd/protoc-gen-go-grpc/main.go: patch does not apply
```

**Почему.** Смотрим сам патч (`registry/grpc/go/separate-package.patch`):
```diff
diff --git a/cmd/protoc-gen-go-grpc/main.go b/cmd/protoc-gen-go-grpc/main.go
@@ -45,6 +45,7 @@ const version = "1.6.2"
 const version = "1.6.2"
 
 var requireUnimplemented *bool
+var separatePackage *bool
```

Контекстная строка диффа буквально содержит `const version = "1.6.2"` — то есть патч сгенерирован против исходников тега **`v1.6.2`** и требует, чтобы строка версии в файле совпадала один-в-один. У `v1.5.1` в этом месте будет `const version = "1.5.1"` — контекст не совпадает, `git apply` отказывается патчить файл (это не проблема сборки/окружения, а особенность самого патч-файла).

Проверено, что `v1.6.2` — **единственная версия**, у которой в `plugins/grpc/go/v1.6.2/` уже лежит собранный бинарник `plugin` (7.1 МБ, дата сборки 16 июля) — то есть исторически собиралась и работает только она.

Из текущего `plugin.yaml`:
```yaml
versions:
- v1.6.2         # единственная версия, где патч гарантированно применяется
- version: v1.6.1
  skip: true
- version: v1.6.0
  skip: true
- v1.5.1          # НЕ помечена skip, но патч не применится — см. выше
- version: v1.4.0
  skip: true
- version: v1.3.0
  skip: true
- version: v1.2.0
  skip: true
```

Коммит `a78fe3b` пометил `skip: true` только для `v1.2.0`/`v1.3.0` (самые старые, которые я тестировал в первом отчёте), но **не заметил, что `v1.5.1` тоже сломан** — потому что причина не "старая версия vs новая", а "патч жёстко зашит под конкретный текст файла v1.6.2", что ломает вообще все версии, кроме неё самой.

**Как чинить:** либо
(a) пометить `v1.5.1` тоже как `skip: true` (раз реально собирается только `v1.6.2`), либо
(b) переписать патч так, чтобы контекст не зависел от литерала версии (например, матчить по сигнатуре функции/более стабильному якорю, а не по строке `const version = "X.Y.Z"`), что вернёт возможность собирать `v1.5.1` и потенциально более старые версии.

---

## Блок 5. `grpc/web` — контрольная проверка (уже исправлено верно)

`grpc/web:v1.4.2` и `v1.5.0` теперь помечены `skip: true` в `plugin.yaml` (коммит `a78fe3b`). Дополнительно проверено через GitHub Releases API в прошлом отчёте: релиз `1.4.2` публикует только собранные бинарники `protoc-gen-grpc-web-*`, без `grpc-web-source-*.tar.gz` — это единственно верное решение для этих версий (сборка "из исходников" для них невозможна в принципе на текущей схеме). Регресса не найдено, `grpc/web` больше не появляется в списке `к сборке` из `dry-run` (кроме версий выше `v2.0.0`, которые собираются штатно и уже в кэше).

---

## Дополнительные системные наблюдения (не Dockerfile-специфичные)

### Диск хоста заполнен на 100%
```
Filesystem      Size  Used Avail Use% Mounted on
/dev/nvme0n1p2  915G  868G  928M 100% /

docker system df
Images          202.1GB   146.4GB (72%) reclaimable
Build Cache     500.1GB   32.62GB reclaimable
```
Свободно меньше 1 ГБ на диске, притом что Docker хранит 500 ГБ build cache и 202 ГБ образов. Это не было прямой причиной ни одной из воспроизведённых ошибок в этом расследовании, но это критический риск: любая сборка, которой не хватит нескольких сотен мегабайт временного места (а сборки Bazel/protoc потребляют гигабайты на `bazel-out`), может начать падать с `No space left on device` в любой момент, и такие сбои будет трудно отличать от описанных выше логических багов. Рекомендуется `docker builder prune` / `docker image prune` при первой возможности (не выполнялось мной — решение об удалении GBs чужого кэша оставляю на усмотрение владельца окружения).

### Периодические сетевые таймауты до GitHub Release CDN
Первая попытка собрать `protocolbuffers/pyi:v21.10` зависла на **5 минут** и упала с `curl: (22)` → `exit code: 28` при скачивании `protobuf-all-21.10.tar.gz`. Прямая проверка с хоста:
```
curl -fsSL -o /dev/null https://github.com/.../protobuf-all-21.10.tar.gz --max-time 20
→ curl: (28) Connection timed out after 19850 milliseconds (302 → release-assets.githubusercontent.com)
```
Повторная попытка через несколько секунд отработала за **1.2 секунды** с кодом 200. Похоже на кратковременную сетевую деградацию (не постоянная блокировка конкретного хоста), но означает, что при полном прогоне `plugins build` на весь `registry/` часть версий может падать не по вине Dockerfile, а из-за случайных сетевых сбоев при скачивании исходников/тарболов — стоит закладывать ретраи с backoff на такие `curl`-шаги (сейчас в некоторых Dockerfile уже есть `||`-цепочки fallback URL, но нет ретраев на транспортные таймауты одного и того же URL).

---

## Что не успел проверить (честно, а не додумано)

- Не проверялся флаг `--enable_workspace` как альтернативное решение Блока 2 (итерация занимает 20–60+ секунд на каждую версию, а фиксация Bazel-версии — более предсказуемое и уже прецедентное в этом репо решение, поэтому приоритет отдан ему).
- Не пересобирались все 466 версий целиком (заняло бы часы и заново упёрлось бы в диск/сеть) — проверка сделана на репрезентативных версиях каждой группы, а идентичность ошибки в остальных версиях той же группы подтверждена совпадением паттерна Dockerfile (все версии одной группы используют один и тот же Dockerfile и один и тот же набор `BAZEL_*_OPTS`).
- Не проверялось, есть ли среди 466 версий такие, что реально собираются с текущим кодом (учитывая, что `dry-run` показывает `466 к сборке` против `470` в первом отчёте — то есть 4 версии сейчас корректно исключены `skip`, но не подтверждено, что остальные 462 не содержат единичных "здоровых" версий помимо `v1.6.2` и `v2.0.0+`).

## Рекомендации (обновлено)

1. Убрать `--repo_contents_cache` из `BAZEL_BUILD_OPTS` во всех `grpc/*` Dockerfile — этот флаг не поддерживается Bazel 7.1.0, который `grpc` пинит сам. Оставить только `--spawn_strategy=local`.
2. Запинить версию Bazel (`USE_BAZEL_VERSION=7.x`, по аналогии с `grpc/web`) во всех `protocolbuffers/*` Dockerfile, иначе все версии protobuf, использующие legacy WORKSPACE (примерно всё до перехода на bzlmod), не соберутся вне зависимости от прочих фиксов.
3. Пометить `grpc/go:v1.5.1` как `skip: true` (или переписать `separate-package.patch`, чтобы не зависеть от литерала `"1.6.2"` в контексте диффа).
4. Рассмотреть `docker builder prune`/`docker image prune` — диск хоста заполнен на 100%, что рискует давать случайные, трудно диагностируемые падения сборок.
5. Добавить ретраи с backoff на `curl`-шаги скачивания архивов исходников — сеть до GitHub Release CDN иногда даёт таймаут по 5 минут вместо честной ошибки.
