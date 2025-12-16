#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"

if [[ -f "$ENV_FILE" ]]; then
  set -a
  source "$ENV_FILE"
  set +a
else
  echo "Файл с переменными окружения не найден: $ENV_FILE" >&2
  exit 1
fi

: "${POSTGRES_DB:?Ошибка: переменная POSTGRES_DB не задана}"
: "${POSTGRES_USER:?Ошибка: переменная POSTGRES_USER не задана}"

CONTAINER="qtiva-postgres"

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
  echo "Ошибка: контейнер ${CONTAINER} не запущен."
  exit 1
fi

echo "Применяем миграции..."
for f in ./migrations/*.sql; do
  echo " - $(basename "$f")"
  docker exec -i "${CONTAINER}" psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -v ON_ERROR_STOP=1 < "$f"
done

echo "Все миграции применены!"
