#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
EXAMPLES_DIR="$ROOT_DIR/examples/projects"
ARCHIVES_DIR="$ROOT_DIR/examples/archives"

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

# Создаём директорию для архивов
mkdir -p "$ARCHIVES_DIR"

# Список проектов для сборки
PROJECTS=(
    "qtwidgets_example"
    "qtquick_example"
)

# Функция сборки одного проекта
build_project() {
    local project_name="$1"
    local project_dir="$EXAMPLES_DIR/$project_name"
    local build_dir="$project_dir/build"
    local tests_dir="$project_dir/tests"
    local archive_path="$ARCHIVES_DIR/${project_name}.tar.gz"
    local temp_dir=$(mktemp -d)

    log_info "=========================================="
    log_info "Building project: $project_name"
    log_info "=========================================="

    # Проверяем существование проекта
    if [[ ! -d "$project_dir" ]]; then
        log_error "Project directory not found: $project_dir"
        return 1
    fi

    if [[ ! -f "$project_dir/CMakeLists.txt" ]]; then
        log_error "CMakeLists.txt not found in $project_dir"
        return 1
    fi

    # Проверяем наличие тестов
    if [[ ! -d "$tests_dir" ]]; then
        log_error "Tests directory not found: $tests_dir"
        return 1
    fi

    # Удаляем старую директорию сборки если есть
    if [[ -d "$build_dir" ]]; then
        log_info "Removing old build directory..."
        rm -rf "$build_dir"
    fi

    # Создаём директорию сборки
    mkdir -p "$build_dir"

    # Собираем проект
    log_info "Running CMake..."
    cmake -S "$project_dir" -B "$build_dir" -DCMAKE_BUILD_TYPE=Release

    log_info "Building..."
    cmake --build "$build_dir" --parallel "$(nproc)"

    # Проверяем что исполняемый файл создан
    local executable="$build_dir/$project_name"
    if [[ ! -f "$executable" ]]; then
        log_error "Executable not found: $executable"
        rm -rf "$temp_dir"
        return 1
    fi

    log_info "Build successful: $executable"

    # Собираем архив
    log_info "Creating archive..."

    # Копируем исполняемый файл
    cp "$executable" "$temp_dir/"

    # Копируем тестовые данные (config.json и все .js файлы)
    mkdir -p "$temp_dir/tests"
    cp "$tests_dir/config.json" "$temp_dir/tests/"
    cp "$tests_dir"/*.js "$temp_dir/tests/"

    # Собираем список тестовых скриптов
    local scripts_json="["
    local first=true
    for js_file in "$tests_dir"/*.js; do
        local js_name=$(basename "$js_file")
        if [[ "$first" == true ]]; then
            scripts_json="$scripts_json\"tests/$js_name\""
            first=false
        else
            scripts_json="$scripts_json, \"tests/$js_name\""
        fi
    done
    scripts_json="$scripts_json]"

    # Создаём manifest.json
    cat > "$temp_dir/manifest.json" << EOF
{
  "config_path": "tests/config.json",
  "scripts": $scripts_json,
  "application": "./$project_name",
  "timeout_sec": 60,
  "env": {}
}
EOF

    log_info "Generated manifest.json:"
    cat "$temp_dir/manifest.json"

    # Создаём архив
    tar -czf "$archive_path" -C "$temp_dir" .

    log_info "Archive created: $archive_path"
    log_info "Archive contents:"
    tar -tzf "$archive_path"

    # Удаляем временную директорию
    rm -rf "$temp_dir"

    # Удаляем директорию сборки
    log_info "Removing build directory..."
    rm -rf "$build_dir"

    log_info "Done: $project_name"
    echo ""
}

# Основной цикл
log_info "Starting build process..."
log_info "Projects: ${PROJECTS[*]}"
log_info "Archives directory: $ARCHIVES_DIR"
echo ""

FAILED=0
for project in "${PROJECTS[@]}"; do
    if ! build_project "$project"; then
        log_error "Failed to build: $project"
        FAILED=$((FAILED + 1))
    fi
done

# Итоговый отчёт
echo ""
log_info "=========================================="
log_info "Build Summary"
log_info "=========================================="
log_info "Total projects: ${#PROJECTS[@]}"
log_info "Failed: $FAILED"

if [[ $FAILED -eq 0 ]]; then
    log_info "All projects built successfully!"
    log_info "Archives location: $ARCHIVES_DIR"
    ls -la "$ARCHIVES_DIR"
    exit 0
else
    log_error "Some projects failed to build"
    exit 1
fi
