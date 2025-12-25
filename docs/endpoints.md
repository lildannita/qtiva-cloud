# ПРИЛОЖЕНИЕ А. СПЕЦИФИКАЦИЯ HTTP API

## А.1. Общие положения

Все endpoints требуют указания заголовка `Content-Type: application/json` для запросов с телом. Защищённые endpoints требуют JWT-токен в заголовке `Authorization: Bearer <token>`.

### Коды ответов

- **200 OK** — успешное выполнение запроса
- **201 Created** — ресурс успешно создан
- **400 Bad Request** — ошибка валидации входных данных
- **401 Unauthorized** — отсутствует или невалиден токен аутентификации
- **403 Forbidden** — недостаточно прав для выполнения операции
- **404 Not Found** — запрашиваемый ресурс не найден
- **409 Conflict** — конфликт состояния (например, запрос лога незавершённого прогона)
- **413 Payload Too Large** — размер загружаемого файла превышает лимит
- **500 Internal Server Error** — внутренняя ошибка сервера

### Формат ошибок

```json
{
  "error": "ERROR_CODE",
  "message": "Описание ошибки",
  "request_id": "req_xxxxxxxxxxxxx"
}
```

---

## А.2. Группа: Authentication (Аутентификация)

### POST /auth/register

Регистрация нового пользователя по invite-коду.

**Доступ:** публичный

**Тело запроса:**

```json
{
  "email": "string (обязательно, email формат)",
  "password": "string (обязательно, минимум 8 символов)",
  "invite_code": "string (обязательно)"
}
```

**Пример запроса:**

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepass123",
    "invite_code": "INV-XMPW-01KD"
  }'
```

**Успешный ответ (201 Created):**

```json
{
  "client_id": "cl_01KDAWZ5FG32XDZG5QEZB3WH4T",
  "email": "user@example.com",
  "user_id": "usr_01KDAXC5M1PH81HCNZZVC58S2Y"
}
```

**Ошибки:**

- `400 VALIDATION_ERROR` — некорректный формат email или слабый пароль
- `400 INVITE_NOT_FOUND` — invite-код не найден или уже использован
- `409 EMAIL_EXISTS` — пользователь с таким email уже существует

---

### POST /auth/login

Аутентификация пользователя и получение JWT-токена.

**Доступ:** публичный

**Тело запроса:**

```json
{
  "email": "string (обязательно)",
  "password": "string (обязательно)"
}
```

**Пример запроса:**

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepass123"
  }'
```

**Успешный ответ (200 OK):**

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in_sec": 86400,
  "token_type": "Bearer"
}
```

**Ошибки:**

- `401 UNAUTHORIZED` — неверный email или пароль

---

### GET /me

Получение информации о текущем аутентифицированном пользователе.

**Доступ:** требуется JWT-токен

**Пример запроса:**

```bash
curl -X GET http://localhost:8080/me \
  -H "Authorization: Bearer $TOKEN"
```

**Успешный ответ (200 OK):**

```json
{
  "client_id": "cl_01KDAWZ5FG32XDZG5QEZB3WH4T",
  "email": "user@example.com",
  "role": "user",
  "user_id": "usr_01KDAXC5M1PH81HCNZZVC58S2Y"
}
```

**Ошибки:**

- `401 UNAUTHORIZED` — невалидный или истёкший токен

---

## А.3. Группа: Admin (Административные функции)

### POST /admin/clients

Создание нового клиента.

**Доступ:** требуется JWT-токен с ролью `admin`

**Тело запроса:**

```json
{
  "name": "string (обязательно)",
  "network_allowed": "boolean (обязательно)",
  "concurrency_limit": "integer (обязательно, > 0)"
}
```

**Пример запроса:**

```bash
curl -X POST http://localhost:8080/admin/clients \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "BMSTU TEST",
    "network_allowed": false,
    "concurrency_limit": 3
  }'
```

**Успешный ответ (201 Created):**

```json
{
  "client_id": "cl_01KDAWZ5FG32XDZG5QEZB3WH4T"
}
```

**Ошибки:**

- `401 UNAUTHORIZED` — невалидный токен
- `403 FORBIDDEN` — недостаточно прав (роль не admin)
- `400 VALIDATION_ERROR` — некорректные входные данные

---

### PATCH /admin/clients/{id}

Изменение параметров существующего клиента.

**Доступ:** требуется JWT-токен с ролью `admin`

**Параметры пути:**

- `id` — идентификатор клиента (например, `cl_01KDAWZ5FG32XDZG5QEZB3WH4T`)

**Тело запроса (все поля опциональны):**

```json
{
  "name": "string",
  "network_allowed": "boolean",
  "concurrency_limit": "integer"
}
```

**Пример запроса:**

```bash
curl -X PATCH http://localhost:8080/admin/clients/cl_01KDAWZ5FG32XDZG5QEZB3WH4T \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "concurrency_limit": 5
  }'
```

**Успешный ответ (200 OK):**

```json
{
  "client_id": "cl_01KDAWZ5FG32XDZG5QEZB3WH4T",
  "updated": true
}
```

**Ошибки:**

- `401 UNAUTHORIZED` — невалидный токен
- `403 FORBIDDEN` — недостаточно прав
- `404 NOT_FOUND` — клиент не найден

---

### POST /admin/invites

Создание invite-кода для регистрации пользователей.

**Доступ:** требуется JWT-токен с ролью `admin`

**Тело запроса:**

```json
{
  "client_id": "string (обязательно)",
  "max_uses": "integer (обязательно, > 0)"
}
```

**Пример запроса:**

```bash
curl -X POST http://localhost:8080/admin/invites \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "cl_01KDAWZ5FG32XDZG5QEZB3WH4T",
    "max_uses": 10
  }'
```

**Успешный ответ (201 Created):**

```json
{
  "invite_code": "INV-XMPW-01KD"
}
```

**Ошибки:**

- `401 UNAUTHORIZED` — невалидный токен
- `403 FORBIDDEN` — недостаточно прав
- `404 NOT_FOUND` — клиент не найден

---

## А.4. Группа: Artifacts (Управление артефактами)

### POST /artifacts

Загрузка тестового артефакта (TAR-архив с приложением и тестами).

**Доступ:** требуется JWT-токен

**ВАЖНО:** Endpoint доступен только через Upload API на порту **8081**. Запросы на порт 8080 будут отклонены с ошибкой `UPLOAD_PORT_REQUIRED`.

**Тип запроса:** `multipart/form-data`

**Параметры формы:**

- `file` — TAR-архив (обязательно, максимум 1 ГБ)

**Структура архива:**

- Должен содержать файл `manifest.json` в корне
- Обязательные поля манифеста: `config_path`, `scripts`, `application`, `timeout_sec`

**Пример запроса:**

```bash
curl -X POST http://localhost:8081/artifacts \
  -H "Authorization: Bearer $USER_TOKEN" \
  -F "file=@./qtwidgets_example-ubuntu.tar.gz"
```

**Успешный ответ (201 Created):**

```json
{
  "artifact_id": "art_01KDAYRJSQMD4X9T90Q18WNDS5",
  "sha256": "50dc9a6335171dfdda98931988d546ae7b863ddfc99182915e93c572ee14c84",
  "size_bytes": 9426
}
```

**Ошибки:**

- `401 UNAUTHORIZED` — невалидный токен
- `400 UPLOAD_PORT_REQUIRED` — попытка загрузки на порт 8080
- `400 INVALID_MANIFEST` — отсутствует или некорректный manifest.json
- `413 PAYLOAD_TOO_LARGE` — размер файла превышает 1 ГБ

---

## А.5. Группа: Runs (Управление прогонами)

### POST /runs

Создание нового прогона теста.

**Доступ:** требуется JWT-токен

**Тело запроса:**

```json
{
  "artifact_id": "string (обязательно)",
  "os": "string (обязательно: ubuntu | manjaro)",
  "display": "string (обязательно: x11 | wayland)",
  "qt_version": "string (обязательно: 5.15)"
}
```

**Пример запроса:**

```bash
curl -X POST http://localhost:8080/runs \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "artifact_id": "art_01KDAYRJSQMD4X9T90Q18WNDS5",
    "os": "manjaro",
    "display": "x11",
    "qt_version": "5.15"
  }'
```

**Успешный ответ (201 Created):**

```json
{
  "run_id": "run_01KDAYXHAMGPNAF9116JVY2HRV",
  "status": "pending"
}
```

**Ошибки:**

- `401 UNAUTHORIZED` — невалидный токен
- `404 ARTIFACT_NOT_FOUND` — артефакт не найден или не принадлежит клиенту пользователя
- `400 VALIDATION_ERROR` — некорректные параметры окружения

---

### GET /runs/{id}

Получение статуса и информации о прогоне.

**Доступ:** требуется JWT-токен

**Параметры пути:**

- `id` — идентификатор прогона (например, `run_01KDAYXHAMGPNAF9116JVY2HRV`)

**Пример запроса:**

```bash
curl -X GET http://localhost:8080/runs/run_01KDAYXHAMGPNAF9116JVY2HRV \
  -H "Authorization: Bearer $USER_TOKEN"
```

**Успешный ответ (200 OK):**

```json
{
  "run_id": "run_01KDAYXHAMGPNAF9116JVY2HRV",
  "status": "passed",
  "exit_code": 0,
  "created_at": "2025-12-25T17:34:30.099739+03:00",
  "started_at": "2025-12-25T17:34:30.23568+03:00",
  "finished_at": "2025-12-25T17:34:31.928426+03:00",
  "log": {
    "available": true,
    "download_url": "/runs/run_01KDAYXHAMGPNAF9116JVY2HRV/log"
  }
}
```

**Возможные значения `status`:**

- `pending` — прогон ожидает выполнения в очереди
- `running` — прогон выполняется
- `passed` — прогон завершён успешно (exit_code = 0)
- `failed` — прогон завершён с ошибкой (exit_code ≠ 0)
- `timeout` — превышено максимальное время выполнения
- `error` — системная ошибка при выполнении

**Ошибки:**

- `401 UNAUTHORIZED` — невалидный токен
- `404 NOT_FOUND` — прогон не найден или не принадлежит клиенту пользователя

---

### GET /runs/{id}/log

Скачивание лога выполнения прогона.

**Доступ:** требуется JWT-токен

**Параметры пути:**

- `id` — идентификатор прогона

**Пример запроса:**

```bash
curl -X GET http://localhost:8080/runs/run_01KDAYXHAMGPNAF9116JVY2HRV/log \
  -H "Authorization: Bearer $USER_TOKEN" \
  -o run.log
```

**Успешный ответ (200 OK):**

```
Content-Type: text/plain; charset=utf-8
Content-Disposition: attachment; filename="run.log"

[2025-12-25T14:34:30+00:00] === qtiva Runner Start ===
[2025-12-25T14:34:30+00:00] Run ID: run_01KDAYXHAMGPNAF9116JVY2HRV
...
[2025-12-25T14:34:31+00:00] === OVERALL: PASSED ===
```

**Ошибки:**

- `401 UNAUTHORIZED` — невалидный токен
- `404 NOT_FOUND` — прогон не найден или лог недоступен
- `409 CONFLICT` — прогон ещё не завершён (статус `pending` или `running`)

---

## А.6. Группа: Service (Служебные endpoints)

### GET /health

Проверка работоспособности сервисов Manager и Agent (health check).

**Доступ:** публичный

**Пример запроса:**

```bash
curl -X GET http://localhost:8080/health
curl -X GET http://localhost:8090/health
```

**Успешный ответ (200 OK):**

```json
{
  "status": "ok",
  "service": "qtiva-manager", # "qtiva-agent"
  "version": "dev",
  "commit": "abea2c0"
}
```

---

### GET /ready

Проверка готовности сервиса (readiness probe с проверкой подключения к БД).

**Доступ:** публичный

**Пример запроса:**

```bash
curl -X GET http://localhost:8080/ready
```

**Успешный ответ (200 OK):**

```
ok
```

**Ошибка (503 Service Unavailable):**

```
db not ready
```

---

## А.7. Сводная таблица endpoints

| Метод | Путь                | Порт       | Аутентификация | Описание                  |
| ----- | ------------------- | ---------- | -------------- | ------------------------- |
| POST  | /auth/register      | 8080       | Публичный      | Регистрация пользователя  |
| POST  | /auth/login         | 8080       | Публичный      | Аутентификация            |
| GET   | /me                 | 8080       | JWT            | Информация о пользователе |
| POST  | /admin/clients      | 8080       | JWT (admin)    | Создание клиента          |
| PATCH | /admin/clients/{id} | 8080       | JWT (admin)    | Изменение клиента         |
| POST  | /admin/invites      | 8080       | JWT (admin)    | Генерация invite-кода     |
| POST  | /artifacts          | **8081**   | JWT            | Загрузка артефакта        |
| POST  | /runs               | 8080       | JWT            | Создание прогона          |
| GET   | /runs/{id}          | 8080       | JWT            | Получение статуса         |
| GET   | /runs/{id}/log      | 8080       | JWT            | Скачивание лога           |
| GET   | /health             | 8080, 8090 | Публичный      | Health check              |
| GET   | /ready              | 8080       | Публичный      | Readiness probe           |

---