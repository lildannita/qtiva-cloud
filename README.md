# qtiva-cloud

Система удалённого GUI-тестирования Qt-приложений.

## Архитектура

```
                                      ┌─────────────────┐
                                      │   PostgreSQL    │
                                      │ (metadata, jobs)│
                                      └────────┬────────┘
                                               │
┌──────────────┐     HTTPS/JWT     ┌──────────┴──────────┐
│  User / CI   │ ─────────────────►│   qtiva-manager     │
└──────────────┘                   │  (HTTP API + GC)    │
                                   └──────────┬──────────┘
                                              │
                                    jobs table│polling
                                              │
                                   ┌──────────┴──────────┐
                                   │    qtiva-agent      │
                                   │ (worker pool)       │
                                   └──────────┬──────────┘
                                              │
                                    Docker API│
                                              │
                                   ┌──────────┴──────────┐
                                   │   Runner Container  │
                                   │ (gVisor + Wayland)  │
                                   └─────────────────────┘
```

## Быстрый старт

### 1. Подготовка окружения

```bash
# Клонируем репозиторий
git clone https://github.com/lildannita/qtiva-cloud.git
cd qtiva-cloud

# Создаём .env из примера
cp .env.example .env
# Редактируем .env при необходимости
```

### 2. Запуск PostgreSQL

```bash
# Запуск БД
docker compose -f docker-compose.dev.yml up -d

# Применение миграций
./scripts/migrate.sh
```

### 3. Сборка

```bash
# Сборка бинарников manager и agent
make build

# Сборка runner образов (требуется Docker)
./scripts/build-runners.sh
```

### 4. Запуск сервисов

```bash
# Terminal 1: Manager
./bin/qtiva-manager serve

# Terminal 2: Agent
./bin/qtiva-agent serve
```

### 5. Создание администратора

```bash
./bin/qtiva-manager seed-admin \
  --email admin@qtiva.local \
  --password SuperSecurePassword123!
```

## API Endpoints

### Аутентификация

```bash
# Логин (получение JWT)
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@qtiva.local", "password": "SuperSecurePassword123!"}'

# Информация о пользователе
curl http://localhost:8080/me \
  -H "Authorization: Bearer $TOKEN"
```

### Администрирование (только для admin)

```bash
# Создание клиента
curl -X POST http://localhost:8080/admin/clients \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "ClientA", "network_allowed": false, "concurrency_limit": 2}'

# Создание invite-кода
curl -X POST http://localhost:8080/admin/invites \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"client_id": "cl_...", "max_uses": 10}'
```

### Регистрация пользователя

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePassword123!",
    "invite_code": "INV-XXXX-YYYY"
  }'
```

### Загрузка артефакта

```bash
# Загрузка на upload-порт (8081)
curl -X POST http://localhost:8081/artifacts \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@app.tar.gz"
```

### Прогоны

```bash
# Создание прогона
curl -X POST http://localhost:8080/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "artifact_id": "art_...",
    "os": "manjaro",
    "display": "wayland",
    "qt_version": "5.15"
  }'

# Статус прогона
curl http://localhost:8080/runs/run_... \
  -H "Authorization: Bearer $TOKEN"

# Скачивание лога
curl http://localhost:8080/runs/run_.../log \
  -H "Authorization: Bearer $TOKEN" \
  -o run.log
```

## Формат manifest.json

```json
{
  "run_cmd": "./run-tests.sh",
  "timeout_sec": 60,
  "needs_display": true,
  "qt_version": "5.15",
  "os": "manjaro",
  "display": "wayland",
  "env": {
    "LC_ALL": "C.UTF-8",
    "QT_QPA_PLATFORM": "wayland"
  }
}
```

### Поля:

| Поле | Тип | Описание |
|------|-----|----------|
| `run_cmd` | string | Команда для запуска (обязательно) |
| `timeout_sec` | int | Максимальное время выполнения в секундах (обязательно) |
| `needs_display` | bool | Требуется ли display server |
| `qt_version` | string | Версия Qt (`5.15`) |
| `os` | string | Целевая ОС (`manjaro`, `ubuntu`) |
| `display` | string | Тип дисплея (`wayland`, `x11`) |
| `env` | object | Дополнительные переменные окружения |

## Статусы прогона

| Статус | Описание |
|--------|----------|
| `pending` | Ожидает выполнения |
| `running` | Выполняется |
| `passed` | Успешно завершён (exit_code = 0) |
| `failed` | Завершён с ошибкой (exit_code != 0) |
| `timeout` | Превышен лимит времени |
| `error` | Системная ошибка |

## Безопасность

- Контейнеры запускаются через **gVisor** (`runsc`)
- Сеть отключена по умолчанию (`--network none`)
- Read-only root filesystem
- Все capabilities сброшены
- `no-new-privileges` включен
- Ресурсы ограничены (CPU, RAM, pids)

## Переменные окружения

См. `.env.example` для полного списка.

### Основные:

| Переменная | Описание |
|------------|----------|
| `QTIVA_MANAGER_HTTP_ADDR` | Адрес API сервера |
| `QTIVA_MANAGER_UPLOAD_HTTP_ADDR` | Адрес upload сервера |
| `QTIVA_AGENT_HTTP_ADDR` | Адрес agent сервера |
| `QTIVA_AGENT_CONCURRENCY` | Количество воркеров |
| `QTIVA_DOCKER_RUNTIME` | Docker runtime (`runsc`) |
| `POSTGRES_DSN` | Строка подключения к PostgreSQL |

## Разработка

```bash
# Запуск тестов
go test ./...

# Форматирование
go fmt ./...

# Линтер
golangci-lint run
```

## Лицензия

MIT
