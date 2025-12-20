#!/bin/bash
set -euo pipefail

# Qtiva Runner Script
# Этот скрипт выполняется внутри runner контейнера

INPUT_ARTIFACT="/input/app.tar.gz"
LOG_FILE="/artifacts/run.log"
WORK_DIR="/work"

# Функция логирования
log() {
    echo "[$(date -Iseconds)] $*"
}

# Перенаправляем всё в лог
exec > >(tee -a "$LOG_FILE") 2>&1

log "=== Qtiva Runner Start ==="
log "Run ID: ${QTIVA_RUN_ID:-unknown}"
log "Timeout: ${QTIVA_TIMEOUT_SEC:-60} seconds"

# Проверяем наличие артефакта
if [[ ! -f "$INPUT_ARTIFACT" ]]; then
    log "ERROR: Artifact $INPUT_ARTIFACT not found"
    exit 1
fi

ls -l "$INPUT_ARTIFACT"

# Создаём рабочую директорию
# mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

ls -ld "$WORK_DIR"
whoami

# Распаковываем артефакт
log "Extracting artifact..."
tar -xzf "$INPUT_ARTIFACT" \
  --no-same-owner --no-same-permissions \
  --exclude='._*' \
  --warning=no-unknown-keyword 2>&1 || {
    log "ERROR: Failed to extract artifact"
    exit 1
}

# Проверяем наличие manifest.json
if [[ ! -f manifest.json ]]; then
    log "ERROR: manifest.json not found in artifact"
    exit 1
fi

# Парсим manifest.json
RUN_CMD=$(jq -r '.run_cmd // empty' manifest.json)
NEEDS_DISPLAY=$(jq -r '.needs_display // false' manifest.json)
DISPLAY_TYPE=$(jq -r '.display // "wayland"' manifest.json)
TIMEOUT_SEC=${QTIVA_TIMEOUT_SEC:-$(jq -r '.timeout_sec // 60' manifest.json)}

if [[ -z "$RUN_CMD" ]]; then
    log "ERROR: run_cmd not specified in manifest.json"
    exit 1
fi

log "Run command: $RUN_CMD"
log "Needs display: $NEEDS_DISPLAY"
log "Display type: $DISPLAY_TYPE"
log "Timeout: ${TIMEOUT_SEC}s"

# Настройка XDG
export XDG_RUNTIME_DIR="/tmp/runtime"
mkdir -p "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR"

# Запуск display (если требуется)
if [[ "$NEEDS_DISPLAY" == "true" ]]; then
    log "Starting display server ($DISPLAY_TYPE)..."
    
    if [[ "$DISPLAY_TYPE" == "wayland" ]]; then
        # Запускаем Weston в headless режиме
        export WAYLAND_DISPLAY="wayland-0"
        export QT_QPA_PLATFORM="wayland"
        
        weston --backend=headless-backend.so \
               --socket="$WAYLAND_DISPLAY" \
               --width=1920 --height=1080 \
               &>/tmp/weston.log &
        DISPLAY_PID=$!
        
        # Ждём запуска Weston
        for i in {1..30}; do
            if [[ -S "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY" ]]; then
                log "Weston started successfully"
                break
            fi
            sleep 0.1
        done
        
        if [[ ! -S "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY" ]]; then
            log "ERROR: Weston failed to start"
            cat /tmp/weston.log 2>/dev/null || true
            exit 1
        fi
    elif [[ "$DISPLAY_TYPE" == "x11" ]]; then
        # Запускаем Xvfb
        export DISPLAY=":99"
        export QT_QPA_PLATFORM="xcb"
        
        Xvfb :99 -screen 0 1920x1080x24 &>/tmp/xvfb.log &
        DISPLAY_PID=$!
        
        # Ждём запуска Xvfb
        sleep 1
        
        if ! kill -0 $DISPLAY_PID 2>/dev/null; then
            log "ERROR: Xvfb failed to start"
            cat /tmp/xvfb.log 2>/dev/null || true
            exit 1
        fi
        log "Xvfb started successfully"
    fi
fi

# Применяем переменные окружения из manifest
log "Setting environment variables..."
while IFS='=' read -r key value; do
    if [[ -n "$key" && "$key" != "null" ]]; then
        export "$key=$value"
        log "  $key=$value"
    fi
done < <(jq -r '.env // {} | to_entries[] | "\(.key)=\(.value)"' manifest.json 2>/dev/null || true)

# Делаем исполняемые файлы исполняемыми
find . -name "*.sh" -type f -exec chmod +x {} \; 2>/dev/null || true
find . -type f -perm /u+x 2>/dev/null || true

# Выполняем команду с таймаутом
log "=== Executing run command ==="
log "Command: $RUN_CMD"
log "Working directory: $(pwd)"

EXIT_CODE=0
timeout --kill-after=10s "${TIMEOUT_SEC}s" bash -c "$RUN_CMD" || EXIT_CODE=$?

# Проверяем код выхода
if [[ $EXIT_CODE -eq 124 ]]; then
    log "=== TIMEOUT: Command exceeded ${TIMEOUT_SEC}s limit ==="
elif [[ $EXIT_CODE -ne 0 ]]; then
    log "=== FAILED: Exit code $EXIT_CODE ==="
else
    log "=== PASSED: Exit code 0 ==="
fi

# Останавливаем display server
if [[ -n "${DISPLAY_PID:-}" ]]; then
    log "Stopping display server..."
    kill $DISPLAY_PID 2>/dev/null || true
    wait $DISPLAY_PID 2>/dev/null || true
fi

log "=== Qtiva Runner End ==="
exit $EXIT_CODE
