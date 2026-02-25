# План реализации: Миграция стека наблюдаемости

## Обзор

Инкрементальная миграция инфраструктуры наблюдаемости EasyP. Все задачи — создание конфигурационных файлов и обновление `docker-compose.yml`. Go-код не изменяется. Каждый шаг строится на предыдущем: сначала конфиги отдельных сервисов, затем полная сборка docker-compose.

## Задачи

- [x] 1. Создать конфигурацию Traefik
  - Создать файл `configs/traefik/traefik.yml` с содержимым из дизайн-документа
  - Настроить entryPoint `web` на порт 80, Docker provider с `exposedByDefault: false`, сеть `easyp_network`
  - _Требования: 1.1, 1.2, 11.2_

- [x] 2. Создать конфигурацию Mimir
  - Создать файл `configs/mimir/mimir.yaml` с содержимым из дизайн-документа
  - Настроить monolithic mode, S3 backend на `rustfs:9000`, bucket `mimir`, HTTP порт 9009
  - _Требования: 3.2, 3.4, 11.2_

- [x] 3. Создать конфигурацию Loki
  - Создать файл `configs/loki/loki.yml` с содержимым из дизайн-документа
  - Настроить S3 storage на `rustfs:9000`, bucket `loki`, schema v13 с TSDB store
  - _Требования: 4.1, 4.2, 4.3, 11.2_

- [x] 4. Создать конфигурацию Tempo
  - Создать файл `configs/tempo/tempo.yaml` с содержимым из дизайн-документа
  - Настроить OTLP receivers (gRPC :4317, HTTP :4318), S3 backend на `rustfs:9000`, bucket `tempo`, metrics_generator с remote write в Mimir
  - _Требования: 5.2, 5.3, 5.4, 11.2_

- [x] 5. Создать конфигурацию Alloy
  - Создать файл `configs/alloy/config.alloy` в River-синтаксисе с содержимым из дизайн-документа
  - Настроить сбор логов (Docker → Loki), скрейпинг метрик (service:8081 → Mimir), приём/пересылку трейсов (OTLP → Tempo)
  - _Требования: 6.2, 6.3, 6.4, 6.5, 11.2_

- [x] 6. Создать конфигурацию Pyroscope
  - Создать файл `configs/pyroscope/pyroscope.yaml` с содержимым из дизайн-документа
  - Настроить S3 backend на `rustfs:9000`, bucket `pyroscope`
  - _Требования: 7.2, 7.3, 11.2_

- [x] 7. Создать конфигурацию Grafana
  - [x] 7.1 Создать `configs/grafana/config.ini`
    - Настроить анонимную авторизацию, admin root/root, JSON-логирование, тёмную тему
    - _Требования: 8.4, 11.2_

  - [x] 7.2 Создать `configs/grafana/provisioning/datasources/datasources.yaml`
    - Добавить источники данных: Mimir (prometheus, isDefault), Loki (с derivedFields для TraceID), Tempo (с tracesToLogs/tracesToMetrics), Pyroscope
    - Настроить корреляцию между источниками данных
    - _Требования: 8.2, 8.3_

  - [x] 7.3 Создать `configs/grafana/provisioning/dashboards/dashboards.yaml`
    - Настроить провайдеры дашбордов для папок logs, metrics, service
    - _Требования: 8.4_

- [x] 8. Перенести существующие дашборды
  - Скопировать `infrastructure/grafana/dashboards/logs/service.json` → `configs/grafana/provisioning/dashboards/logs/service.json`
  - Скопировать `infrastructure/grafana/dashboards/prometheus/service.json` → `configs/grafana/provisioning/dashboards/metrics/service.json`
  - Скопировать `infrastructure/grafana/dashboards/service/service.json` → `configs/grafana/provisioning/dashboards/service/service.json`
  - Обновить datasource UID с `Prometheus` на `Mimir` в скопированных дашбордах при необходимости
  - _Требования: 8.4_

- [x] 9. Обновить docker-compose.yml
  - [x] 9.1 Заменить `docker-compose.yml` полной конфигурацией из дизайн-документа
    - Добавить сервисы: traefik, rustfs, init-buckets, mimir, loki, tempo, alloy, pyroscope, grafana, registry, postgres, service
    - Удалить сервисы: prometheus, promtail
    - Настроить сеть `easyp_network` и volumes (registry-data, postgres-data, rustfs-data, grafana-data)
    - _Требования: 1.1, 1.2, 1.3, 1.4, 2.1, 2.3, 2.4, 2.5, 3.1, 3.3, 3.5, 4.4, 5.1, 5.5, 6.1, 6.6, 7.1, 7.4, 8.1, 8.5, 9.1, 9.2, 9.3, 9.4, 10.1, 10.2, 10.3, 11.1, 12.1, 12.2, 12.3_

  - [x] 9.2 Настроить зависимости между сервисами
    - `init-buckets` → `rustfs` (service_healthy)
    - `loki`, `mimir`, `tempo`, `pyroscope` → `init-buckets` (service_completed_successfully)
    - `alloy` → `loki`, `mimir`, `tempo` (service_started)
    - `grafana` → `loki`, `mimir`, `tempo`, `pyroscope` (service_started)
    - `service` → `postgres`, `alloy` (service_started)
    - _Требования: 2.5, 3.5, 4.4, 5.5, 7.4, 10.3_

  - [x] 9.3 Настроить переменные окружения для портов
    - Все внешние порты через `${EASYP_*:-default}`: Traefik (80), Grafana (3000), PostgreSQL (5432), Registry (5005), gRPC (8080), metrics (8081), health (8082), gateway (8083)
    - _Требования: 1.4, 9.3, 12.1, 12.2, 12.3_

- [x] 10. Контрольная точка — проверка конфигурации
  - Убедиться что все конфигурационные файлы созданы в `configs/`
  - Убедиться что `docker-compose.yml` валиден
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 11. Property-based тесты для валидации конфигурации
  - [ ]* 11.1 Property-тест: веб-сервисы имеют Traefik-лейблы
    - **Property 1: Веб-сервисы имеют Traefik-лейблы для маршрутизации**
    - Для каждого веб-сервиса (Grafana, RustFS) проверить наличие `traefik.enable=true`, `traefik.http.routers.*.rule` с `Host(...)`, `traefik.http.services.*.loadbalancer.server.port`
    - **Validates: Requirements 1.3, 8.5**

  - [ ]* 11.2 Property-тест: внешние порты используют переменные окружения
    - **Property 2: Внешние порты используют переменные окружения с значениями по умолчанию**
    - Для каждого сервиса с внешними портами проверить формат `${EASYP_*:-N}`
    - **Validates: Requirements 1.4, 9.3, 12.1, 12.3**

  - [ ]* 11.3 Property-тест: S3-зависимые сервисы указывают на RustFS
    - **Property 3: S3-зависимые сервисы указывают на RustFS**
    - Для каждого сервиса из {Mimir, Loki, Tempo, Pyroscope} проверить S3 endpoint `rustfs:9000` и корректное имя бакета
    - **Validates: Requirements 2.2, 3.2, 4.1, 4.2, 5.2, 7.2**

  - [ ]* 11.4 Property-тест: конфигурационные файлы монтируются из ./configs/
    - **Property 4: Конфигурационные файлы монтируются из ./configs/**
    - Для каждого сервиса с конфигом проверить volume mount на `./configs/{service}/` и отсутствие `./infrastructure/`
    - **Validates: Requirements 3.4, 4.3, 5.4, 6.5, 7.3, 8.4, 11.1**

  - [ ]* 11.5 Property-тест: S3-зависимые сервисы ожидают init-buckets
    - **Property 5: S3-зависимые сервисы ожидают создания бакетов**
    - Для каждого сервиса из {Mimir, Loki, Tempo, Pyroscope} проверить `depends_on: init-buckets: condition: service_completed_successfully`
    - **Validates: Requirements 3.5, 4.4, 5.5, 7.4**

  - [ ]* 11.6 Property-тест: init-buckets создаёт все бакеты
    - **Property 6: Init-buckets создаёт все необходимые бакеты**
    - Для каждого бакета из {mimir, loki, tempo, pyroscope} проверить наличие `mc mb` в команде init-buckets
    - **Validates: Requirements 2.3**

  - [ ]* 11.7 Property-тест: Grafana провизионирована со всеми datasources
    - **Property 7: Grafana провизионирована со всеми источниками данных**
    - Для каждого datasource из {Mimir, Loki, Tempo, Pyroscope} проверить наличие записи с корректным типом и URL
    - **Validates: Requirements 8.2**

  - [ ]* 11.8 Property-тест: конфигурационные файлы существуют
    - **Property 8: Конфигурационные файлы существуют для всех сервисов**
    - Для каждого сервиса из {Traefik, Mimir, Loki, Tempo, Alloy, Pyroscope, Grafana} проверить существование конфиг-файла в `./configs/`
    - **Validates: Requirements 11.2**

- [x] 12. Финальная контрольная точка
  - Ensure all tests pass, ask the user if questions arise.

## Примечания

- Задачи с `*` — опциональные (property-тесты), могут быть пропущены для быстрого MVP
- Каждая задача ссылается на конкретные требования для трассируемости
- Go-код EasyP Service не изменяется в рамках данного спека
- Старая директория `infrastructure/` остаётся в репозитории, её удаление — отдельная задача
- Дашборд `prometheus/service.json` переименовывается в `metrics/service.json` при переносе
