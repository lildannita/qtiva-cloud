#!/bin/bash
set -euo pipefail

INPUT_ARTIFACT="/input/app.tar.gz"
LOG_FILE="/artifacts/run.log"
# Не работаем чисто в /work, т.к. там могут быть проблемы с правами
WORK_DIR="/work/data"

# Функция логирования
log() {
    echo "[$(date -Iseconds)] $*"
}

# Перенаправляем всё в лог
exec > >(tee -a "$LOG_FILE") 2>&1

log "=== qtiva Runner Start ==="
log "Run ID: ${QTIVA_RUN_ID:-unknown}"

# Проверяем наличие артефакта
if [[ ! -f "$INPUT_ARTIFACT" ]]; then
    log "ERROR: Artifact $INPUT_ARTIFACT not found"
    exit 1
fi

ls -l "$INPUT_ARTIFACT"

# Переходим в рабочую директорию
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

# Распаковываем артефакт
log "Extracting artifact..."
tar -xzf "$INPUT_ARTIFACT" \
  --no-same-owner --no-same-permissions \
  --exclude='._*' \
  --warning=no-unknown-keyword 2>&1 || true

# Проверяем наличие manifest.json
if [[ ! -f manifest.json ]]; then
    log "ERROR: manifest.json not found in artifact"
    exit 1
fi

# Парсим manifest.json для QtAda
CONFIG_PATH=$(jq -r '.config_path // empty' manifest.json)
APPLICATION=$(jq -r '.application // empty' manifest.json)
TIMEOUT_SEC=$(jq -r '.timeout_sec // 60' manifest.json)

# Читаем массив скриптов в bash-массив
mapfile -t SCRIPTS < <(jq -r '.scripts[]' manifest.json 2>/dev/null)

# Валидация обязательных полей
if [[ -z "$CONFIG_PATH" ]]; then
    log "ERROR: config_path not specified in manifest.json"
    exit 1
fi

if [[ ${#SCRIPTS[@]} -eq 0 ]]; then
    log "ERROR: scripts array is empty or not specified in manifest.json"
    exit 1
fi

if [[ -z "$APPLICATION" ]]; then
    log "ERROR: application not specified in manifest.json"
    exit 1
fi

# Проверяем существование конфигурационного файла
if [[ ! -f "$CONFIG_PATH" ]]; then
    log "ERROR: Config file not found: $CONFIG_PATH"
    exit 1
fi

log "Config path: $CONFIG_PATH"
log "Application: $APPLICATION"
log "Scripts count: ${#SCRIPTS[@]}"
log "Timeout per test: ${TIMEOUT_SEC}s"

# Настройка XDG
export XDG_RUNTIME_DIR="/tmp/runtime"
mkdir -p "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR"

# Определяем тип display из переменных окружения контейнера
# (устанавливаются агентом на основе данных из runs)
DISPLAY_TYPE="${QTIVA_DISPLAY_TYPE:-wayland}"
log "Display type: $DISPLAY_TYPE"

# Запуск display server
log "Starting display server ($DISPLAY_TYPE)..."
export LIBGL_ALWAYS_SOFTWARE=1          # Всегда использовать software OpenGL (без GPU/DRI)
export GALLIUM_DRIVER=llvmpipe          # Выбрать софтварный драйвер Mesa (llvmpipe)
export MESA_LOADER_DRIVER_OVERRIDE=llvmpipe  # Жёстко заставить Mesa грузить llvmpipe
export MESA_NO_ZINK=1                   # Отключить Zink (OpenGL поверх Vulkan), чтобы не требовался Vulkan
export QT_OPENGL=software               # Заставить Qt использовать software OpenGL backend

if [[ "$DISPLAY_TYPE" == "wayland" ]]; then
    # Запускаем Weston в headless режиме
    export WAYLAND_DISPLAY="wayland-0"
    export QT_QPA_PLATFORM="wayland"
    weston --backend=headless-backend.so \
           --socket="$WAYLAND_DISPLAY" \
           --width=1920 --height=1080 \
           --use-pixman \
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

# Применяем переменные окружения из manifest
log "Setting environment variables..."
while IFS='=' read -r key value; do
    if [[ -n "$key" && "$key" != "null" ]]; then
        export "$key=$value"
        log "  $key=$value"
    fi
done < <(jq -r '.env // {} | to_entries[] | "\(.key)=\(.value)"' manifest.json 2>/dev/null || true)

# Делаем исполняемые файлы исполняемыми
# find . -name "*.sh" -type f -exec chmod +x {} \; 2>/dev/null || true

# Извлекаем путь к исполняемому файлу и делаем его исполняемым
# APP_BINARY=$(echo "$APPLICATION" | awk '{print $1}')
# if [[ -f "$APP_BINARY" ]]; then
#     chmod +x "$APP_BINARY"
#     log "Made executable: $APP_BINARY"
# fi

log "=== Starting QtAda Test Execution ==="
log "Working directory: $(pwd)"

# Счётчики результатов
TOTAL_TESTS=${#SCRIPTS[@]}
FINAL_EXIT_CODE=0

log "Total scripts: $TOTAL_TESTS"

# # Формируем список скриптов для QtAda
# SCRIPTS_LIST=""
# for SCRIPT in "${SCRIPTS[@]}"; do
#     if [[ ! -f "$SCRIPT" ]]; then
#         log "ERROR: Test script not found: $SCRIPT"
#         exit 1
#     fi
#     SCRIPTS_LIST="$SCRIPTS_LIST $SCRIPT"
#     log "  - $SCRIPT"
# done

# Формируем команду QtAda
# qtada [options] <configuration> --run <script path> [<script path> ...] <application> [args]
# QTADA_CMD="qtada --timeout $TIMEOUT_SEC --show-log --no-highlight --config-path $CONFIG_PATH --run $SCRIPTS_LIST $APPLICATION"
QTADA_ARGS=(qtada --timeout "$TIMEOUT_SEC" --show-elapsed --show-log --no-highlight --config-path "$CONFIG_PATH" --run)
QTADA_ARGS+=("${SCRIPTS[@]}")
QTADA_ARGS+=("$APPLICATION")

log "========================================"
log "=== Running QtAda ==="
log "========================================"
log "Command: ${QTADA_ARGS[*]}"

log "--- Test output start ---"
"${QTADA_ARGS[@]}" || FINAL_EXIT_CODE=$?
log "--- Test output end ---"

# Анализируем результат
if [[ $FINAL_EXIT_CODE -ne 0 ]]; then
    log "RESULT: FAILED (exit code $FINAL_EXIT_CODE)"
else
    log "RESULT: PASSED"
fi

# Останавливаем display server
if [[ -n "${DISPLAY_PID:-}" ]]; then
    log ""
    log "Stopping display server..."
    kill $DISPLAY_PID 2>/dev/null || true
    wait $DISPLAY_PID 2>/dev/null || true
fi

# Финальный отчёт
log ""
log "========================================"
log "=== Test Execution Summary ==="
log "========================================"
log "Total scripts: $TOTAL_TESTS"

if [[ $FINAL_EXIT_CODE -eq 0 ]]; then
    log "=== OVERALL: PASSED ==="
else
    log "=== OVERALL: FAILED ==="
fi

log "=== Qtiva Runner End ==="
exit $FINAL_EXIT_CODE
