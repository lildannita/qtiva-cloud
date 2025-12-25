# qtiva-cloud

Облачная платформа для удалённого автоматического тестирования графических пользовательских интерфейсов Qt-приложений.

## Описание

qtiva-cloud — это SaaS-платформа, предоставляющая изолированные тестовые окружения для запуска GUI-тестов Qt-приложений с различными конфигурациями операционных систем и дисплейных подсистем. Система интегрируется с фреймворком [qtiva (QtAda)](https://github.com/lildannita/qtada) для выполнения автоматических тестов над готовыми бинарными файлами.

## Возможности

- **Микросервисная архитектура**: Manager (управление), Agent (исполнение), Runner containers (изолированные окружения)
- **Мультитенантность**: изоляция данных и ресурсов различных клиентов
- **Множественные конфигурации**:
    - ОС: Ubuntu 22.04, Manjaro
    - Дисплейные системы: X11 (Xvfb), Wayland (Weston headless)
    - Qt версия: 5.15
- **Безопасность**: JWT-аутентификация, RBAC, изоляция через Docker (+ gVisor, если доступен)
- **Контейнеризация**: каждый тест выполняется в одноразовом изолированном контейнере
- **Автоматическая очистка**: TTL-based Garbage Collector для артефактов и результатов

## Быстрый старт

### 1. Клонирование репозитория

```bash
git clone https://github.com/yourusername/qtiva-cloud.git
cd qtiva-cloud
```

### 2. Настройка конфигурации

Создайте файл `.env` на основе примера:

```bash
cp .env.example .env
```

### 3. Сборка компонентов

```bash
# Сборка Manager и Agent
make build

# Сборка runner images для всех конфигураций
make build-runners
```

### 4. Запуск системы

```bash
# Запуск PostgreSQL
make up

# Применение миграций
make migrate

# Создание администратора
make update-admin

# Запуск Manager (в отдельном терминале)
make run-manager

# Запуск Agent (в отдельном терминале)
make run-agent
```

### 5. Проверка работоспособности

```bash
make health-manager
make health-agent
```

## Использование

### Подготовка тестового артефакта

Артефакт представляет собой TAR-архив со следующей структурой:

```
app.tar.gz
├── your_app              # Исполняемый файл приложения
├── manifest.json         # Конфигурация выполнения
└── tests/
    ├── config.json       # Конфиг QtAda
    └── test_script.js    # Тестовые скрипты QtAda
```

Пример `manifest.json`:

```json
{
  "config_path": "tests/config.json",
  "scripts": ["tests/test_script.js"],
  "application": "./your_app",
  "timeout_sec": 60,
  "env": {}
}
```

### Создание тестовых артефактов

Для автоматической подготовки примеров:

```bash
make build-examples-docker
```

### Базовый workflow

```bash
# 1. Аутентификация
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password"}' \
  | jq -r '.access_token')

# 2. Загрузка артефакта
ARTIFACT_ID=$(curl -s -X POST http://localhost:8081/artifacts \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@app.tar.gz" \
  | jq -r '.artifact_id')

# 3. Создание прогона
RUN_ID=$(curl -s -X POST http://localhost:8080/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"artifact_id\":\"$ARTIFACT_ID\",\"os\":\"ubuntu\",\"display\":\"x11\",\"qt_version\":\"5.15\"}" \
  | jq -r '.run_id')

# 4. Проверка статуса
curl -s http://localhost:8080/runs/$RUN_ID \
  -H "Authorization: Bearer $TOKEN"

# 5. Получение лога
curl -s http://localhost:8080/runs/$RUN_ID/log \
  -H "Authorization: Bearer $TOKEN"
```

## Архитектура

```
┌─────────────┐
│   Client    │ (HTTPS)
└──────┬──────┘
       │
       ├─────────────────┐
       │                 │
┌──────▼─────────┐  ┌────▼────────┐
│  Manager API   │  │  Upload API │
│   (port 8080)  │  │ (port 8081) │
└────────┬───────┘  └──────┬──────┘
         │                 │
         └────────┬────────┘
                  │
         ┌────────▼────────┐
         │   PostgreSQL    │
         │  (БД + очередь) │
         └────────┬────────┘
                  │ (polling)
         ┌────────▼────────┐
         │      Agent      │
         │  (Worker Pool)  │
         └────────┬────────┘
                  │ (Docker API)
         ┌────────▼─────────┐
         │ Runner Containers│
         │     (Docker)     │
         └──────────────────┘
```

## API документация

Полная спецификация API доступна в [ПРИЛОЖЕНИЕ А](https://claude.ai/chat/docs/API.md).

Основные endpoints:

- `POST /auth/register` — регистрация пользователя
- `POST /auth/login` — аутентификация
- `POST /artifacts` — загрузка артефакта (порт 8081)
- `POST /runs` — создание прогона
- `GET /runs/{id}` — получение статуса
- `GET /runs/{id}/log` — скачивание лога

## Разработка

### Структура проекта

```
qtiva-cloud/
├── cmd/
│   ├── qtiva-manager/    # Manager API
│   └── qtiva-agent/      # Agent service
├── internal/
│   ├── manager/          # Бизнес-логика Manager
│   ├── agent/            # Бизнес-логика Agent
│   └── .../              # Вспомогательный код и утилиты
├── migrations/           # SQL миграции
├── examples/             # Примеры тестовых приложений
├── scripts/              # Вспомогательные скрипты
└── runner/               # Dockerfile для runner images
```

### Makefile команды

```bash
make build                 # Сборка Manager и Agent
make build-runners         # Сборка runner images
make build-examples-docker # Сборка примеров
make up                    # Запуск PostgreSQL
make migrate               # Применение миграций
make update-admin          # Создание/обновление админа
make run-manager           # Запуск Manager
make run-agent             # Запуск Agent
make health-manager        # Health check Manager
make health-agent          # Health check Agent
```

## Технологический стек

- **Backend**: Go 1.21+
- **Database**: PostgreSQL 16 (метаданные + очередь задач)
- **Контейнеризация**: Docker Engine (+ gVisor, runsc)
- **Аутентификация**: JWT (HS256)
- **Хеширование паролей**: bcrypt

## Ограничения текущей версии (MVP)

- Хранение на локальной файловой системе (планируется S3/MinIO)
- Polling-модель взаимодействия (латентность ~5 сек)
- Только текстовые логи (без скриншотов/видео)
- Отсутствие веб-интерфейса

## Roadmap

- [ ] Миграция на S3/MinIO для артефактов
- [ ] Внедрение брокера сообщений (RabbitMQ/NATS)
- [ ] Веб-интерфейс для управления
- [ ] Сохранение скриншотов и видео тестов
- [ ] Расширение поддерживаемых конфигураций
- [ ] Метрики и мониторинг (Prometheus/Grafana)

## Лицензия

GNU GENERAL PUBLIC LICENSE Version 3, dated 29 June 2007
