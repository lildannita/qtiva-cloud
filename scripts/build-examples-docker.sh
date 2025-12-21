#!/usr/bin/env bash
# =============================================================================
# Сборка тестовых Qt-проектов внутри Docker-контейнеров.
# Создаёт архивы для каждой комбинации проект+платформа:
#   - qtwidgets_example-manjaro.tar.gz
#   - qtwidgets_example-ubuntu.tar.gz  
#   - qtquick_example-manjaro.tar.gz
#   - qtquick_example-ubuntu.tar.gz
#
# Архивы содержат:
#   - Скомпилированный бинарник
#   - manifest.json
#   - tests/ (config.json + *.js скрипты)
#
# Использование:
#   ./scripts/build-examples-docker.sh [--clean] [--project NAME] [--platform NAME]
#
# Опции:
#   --clean         Пересобрать Docker образы
#   --project NAME  Собрать только указанный проект (qtwidgets_example или qtquick_example)
#   --platform NAME Собрать только для указанной платформы (manjaro или ubuntu)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
EXAMPLES_DIR="$ROOT_DIR/examples/projects"
ARCHIVES_DIR="$ROOT_DIR/examples/archives"
DOCKER_DIR="$ROOT_DIR/examples/docker"

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "${BLUE}[STEP]${NC} $*"; }

# Параметры по умолчанию
CLEAN_BUILD=false
FILTER_PROJECT=""
FILTER_PLATFORM=""

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        --clean)
            CLEAN_BUILD=true
            shift
            ;;
        --project)
            FILTER_PROJECT="$2"
            shift 2
            ;;
        --platform)
            FILTER_PLATFORM="$2"
            shift 2
            ;;
        *)
            log_error "Неизвестный аргумент: $1"
            exit 1
            ;;
    esac
done

# Списки проектов и платформ
PROJECTS=("qtwidgets_example" "qtquick_example")
PLATFORMS=("manjaro" "ubuntu")

# Применяем фильтры
if [[ -n "$FILTER_PROJECT" ]]; then
    PROJECTS=("$FILTER_PROJECT")
fi
if [[ -n "$FILTER_PLATFORM" ]]; then
    PLATFORMS=("$FILTER_PLATFORM")
fi

# Проверяем наличие Docker
if ! command -v docker &> /dev/null; then
    log_error "Docker не найден"
    exit 1
fi

# Создаём директории
mkdir -p "$ARCHIVES_DIR"
mkdir -p "$DOCKER_DIR"

# =============================================================================
# Функция сборки Docker-образа для платформы
# =============================================================================
build_docker_image() {
    local platform="$1"
    local image_name="qtiva-builder:$platform"
    local dockerfile="$DOCKER_DIR/Dockerfile.build-$platform"
    
    # Проверяем существует ли образ
    if [[ "$CLEAN_BUILD" == "false" ]] && docker image inspect "$image_name" &> /dev/null; then
        log_info "Docker образ $image_name уже существует (используйте --clean для пересборки)"
        return 0
    fi
    
    log_step "Сборка Docker образа: $image_name"
    
    if [[ ! -f "$dockerfile" ]]; then
        log_error "Dockerfile не найден: $dockerfile"
        return 1
    fi
    
    docker build -f "$dockerfile" -t "$image_name" "$DOCKER_DIR"
    
    log_info "Docker образ $image_name успешно собран"
}

# =============================================================================
# Функция сборки проекта внутри Docker
# =============================================================================
build_project_in_docker() {
    local project_name="$1"
    local platform="$2"
    local image_name="qtiva-builder:$platform"
    local project_dir="$EXAMPLES_DIR/$project_name"
    local tests_dir="$project_dir/tests"
    local archive_name="${project_name}-${platform}.tar.gz"
    local archive_path="$ARCHIVES_DIR/$archive_name"
    
    log_step "Сборка $project_name для $platform"
    
    # Проверяем существование проекта
    if [[ ! -d "$project_dir" ]]; then
        log_error "Директория проекта не найдена: $project_dir"
        return 1
    fi
    
    if [[ ! -f "$project_dir/CMakeLists.txt" ]]; then
        log_error "CMakeLists.txt не найден в $project_dir"
        return 1
    fi
    
    if [[ ! -d "$tests_dir" ]]; then
        log_error "Директория tests не найдена: $tests_dir"
        return 1
    fi
    
    # Создаём временную директорию для вывода
    local temp_output
    temp_output=$(mktemp -d)
    
    # Команда сборки внутри контейнера
    local build_script='
set -e
cd /src
mkdir -p /build
cmake -S . -B /build -DCMAKE_BUILD_TYPE=Release
cmake --build /build --parallel $(nproc)
cp /build/'"$project_name"' /output/
echo "Build completed successfully"
'
    
    # Запускаем сборку в Docker
    log_info "Запуск сборки в контейнере..."
    
    if ! docker run --rm \
        -v "$project_dir:/src:ro" \
        -v "$temp_output:/output" \
        "$image_name" \
        "$build_script"; then
        log_error "Сборка в Docker завершилась с ошибкой"
        rm -rf "$temp_output"
        return 1
    fi
    
    # Проверяем что бинарник создан
    local executable="$temp_output/$project_name"
    if [[ ! -f "$executable" ]]; then
        log_error "Исполняемый файл не найден после сборки: $executable"
        rm -rf "$temp_output"
        return 1
    fi
    
    log_info "Сборка успешна: $executable"
    
    # Собираем архив
    log_info "Создание архива..."
    
    local archive_temp
    archive_temp=$(mktemp -d)
    
    # Копируем исполняемый файл
    cp "$executable" "$archive_temp/"
    chmod +x "$archive_temp/$project_name"
    
    # Копируем тестовые данные
    mkdir -p "$archive_temp/tests"
    cp "$tests_dir/config.json" "$archive_temp/tests/"
    cp "$tests_dir"/*.js "$archive_temp/tests/"
    
    # Формируем список тестовых скриптов для manifest.json
    local scripts_json="["
    local first=true
    for js_file in "$tests_dir"/*.js; do
        local js_name
        js_name=$(basename "$js_file")
        if [[ "$first" == true ]]; then
            scripts_json="$scripts_json\"tests/$js_name\""
            first=false
        else
            scripts_json="$scripts_json, \"tests/$js_name\""
        fi
    done
    scripts_json="$scripts_json]"
    
    # Создаём manifest.json
    cat > "$archive_temp/manifest.json" << EOF
{
  "config_path": "tests/config.json",
  "scripts": $scripts_json,
  "application": "./$project_name",
  "timeout_sec": 60,
  "env": {}
}
EOF
    
    log_info "Сгенерирован manifest.json:"
    cat "$archive_temp/manifest.json"
    
    # Создаём архив
    tar -czf "$archive_path" -C "$archive_temp" .
    
    log_info "Архив создан: $archive_path"
    log_info "Содержимое архива:"
    tar -tzf "$archive_path"
    
    # Очищаем временные директории
    rm -rf "$temp_output"
    rm -rf "$archive_temp"
    
    log_info "Готово: $archive_name"
    echo ""
}

# =============================================================================
# Основной процесс
# =============================================================================

log_info "=========================================="
log_info "Сборка тестовых Qt-проектов через Docker"
log_info "=========================================="
log_info "Проекты: ${PROJECTS[*]}"
log_info "Платформы: ${PLATFORMS[*]}"
log_info "Директория архивов: $ARCHIVES_DIR"
echo ""

# Сначала собираем все необходимые Docker-образы
log_step "Подготовка Docker-образов..."
for platform in "${PLATFORMS[@]}"; do
    if ! build_docker_image "$platform"; then
        log_error "Не удалось собрать Docker-образ для $platform"
        exit 1
    fi
done
echo ""

# Собираем проекты
TOTAL=0
FAILED=0

for project in "${PROJECTS[@]}"; do
    for platform in "${PLATFORMS[@]}"; do
        TOTAL=$((TOTAL + 1))
        if ! build_project_in_docker "$project" "$platform"; then
            log_error "Не удалось собрать $project для $platform"
            FAILED=$((FAILED + 1))
        fi
    done
done

# Итоговый отчёт
echo ""
log_info "=========================================="
log_info "Итоги сборки"
log_info "=========================================="
log_info "Всего сборок: $TOTAL"
log_info "Успешно: $((TOTAL - FAILED))"
log_info "Ошибок: $FAILED"

if [[ $FAILED -eq 0 ]]; then
    log_info "Все проекты успешно собраны!"
    echo ""
    log_info "Созданные архивы:"
    ls -la "$ARCHIVES_DIR"/*.tar.gz 2>/dev/null || true
    exit 0
else
    log_error "Некоторые проекты не были собраны"
    exit 1
fi
