Manager API
	•	GET /health — healthcheck (JSON)
	•	GET /ready — readiness (проверяет БД)
	•	POST /auth/register — регистрация по invite-коду
	•	POST /auth/login — логин, возвращает access_token (Bearer)
	•	GET /me — профиль (нужен Authorization: Bearer <token>)
	•	Admin (нужен role=admin):
	•	POST /admin/clients — создать client/tenant
	•	PATCH /admin/clients/{client_id} — обновить настройки клиента
	•	POST /admin/invites — создать invite-код
	•	Runs (нужен Bearer):
	•	POST /runs — создать прогон
	•	GET /runs/{run_id} — статус прогона
	•	GET /runs/{run_id}/log — скачать лог (после завершения)
На основном порту /artifacts специально отвечает ошибкой “upload-порт required”.

Upload API
	•	GET /health
	•	POST /artifacts — multipart/form-data, поле file, нужен Bearer

Agent API
	•	GET /health
	•	GET /ready — readiness (docker ping + DB ping)

⸻

Health/ready

curl -sS http://127.0.0.1:8080/health
curl -sS http://127.0.0.1:8080/ready
curl -sS http://127.0.0.1:8081/health
curl -sS http://127.0.0.1:8090/health
curl -sS http://127.0.0.1:8090/ready

Регистрация

curl -sS -X POST http://127.0.0.1:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@qtiva.com","password":"user123","invite_code":"INV-XXXX-YYYY"}'

curl -sS -X POST http://127.0.0.1:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$QTIVA_USER_EMAIL\",\"password\":\"$QTIVA_USER_PASSWORD\",\"invite_code\":\"$INVITE_CODE\"}"

Логин → токен

TOKEN="$(curl -sS -X POST http://127.0.0.1:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@qtiva.com","password":"user123"}' \
 | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')"

echo "$TOKEN"

/me

curl -sS http://127.0.0.1:8080/me -H "Authorization: Bearer $TOKEN"

Загрузка артефакта (upload port!)

curl -sS -X POST http://127.0.0.1:8081/artifacts \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./app.tar.gz"

Создать прогон

ARTIFACT_ID="art_..."  # из upload ответа
curl -sS -X POST http://127.0.0.1:8080/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"artifact_id\":\"$ARTIFACT_ID\",\"os\":\"ubuntu\",\"display\":\"x11\",\"qt_version\":\"5.15\"}"

Проверить статус / скачать лог

RUN_ID="run_..."
curl -sS "http://127.0.0.1:8080/runs/$RUN_ID" -H "Authorization: Bearer $TOKEN"
curl -sS "http://127.0.0.1:8080/runs/$RUN_ID/log" -H "Authorization: Bearer $TOKEN" -o "$RUN_ID.log"

Admin: создать клиента

ADMIN_TOKEN="..."  # логин админом через /auth/login
curl -sS -X POST http://127.0.0.1:8080/admin/clients \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Acme LLC","network_allowed":false,"concurrency_limit":1}'

Admin: создать invite

CLIENT_ID="cl_..."
curl -sS -X POST http://127.0.0.1:8080/admin/invites \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"client_id\":\"$CLIENT_ID\",\"max_uses\":10}"


ARTIFACT_ID=$(curl -sS -X POST http://127.0.0.1:8081/artifacts \
  -H "Authorization: Bearer $USER_TOKEN" \
  -F "file=@./examples/archives/qtwidgets_example-ubuntu.tar.gz" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["artifact_id"])') && \
echo "Artifact uploaded: $ARTIFACT_ID" && \
RUN_ID=$(curl -sS -X POST http://127.0.0.1:8080/runs \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"artifact_id\":\"$ARTIFACT_ID\",\"os\":\"ubuntu\",\"display\":\"wayland\",\"qt_version\":\"5.15\"}" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["run_id"])') && \
echo "Run started: $RUN_ID" && \
echo "Waiting for run to complete..." && \
while true; do \
  STATUS=$(curl -sS "http://127.0.0.1:8080/runs/$RUN_ID" -H "Authorization: Bearer $USER_TOKEN" | python3 -c 'import sys,json; print(json.load(sys.stdin)["status"])'); \
  echo "Current status: $STATUS"; \
  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then \
    break; \
  fi; \
  sleep 2; \
done && \
echo "Run finished with status: $STATUS" && \
LOG_FILE="run.log" && \
echo "Downloading log to: $LOG_FILE" && \
curl -sS "http://127.0.0.1:8080/runs/$RUN_ID/log" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -o "$LOG_FILE" && \
echo "Log saved: $LOG_FILE"