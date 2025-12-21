#!/usr/bin/env bash

set -euo pipefail

# Переходим в корень проекта
ROOT_DIR="$(pwd)"
cd "$ROOT_DIR"

# Локаль
export LC_ALL=en_US.UTF-8
export LANG=en_US.UTF-8

# Файлы для исключения (в корне проекта)
EXCLUDE_FILES=(
  ".env"
  "ToR.md"
  "ToR.pdf"
  "LICENSE"
  "go.mod"
  "go.sum"
  ".DS_Store"
)

# Директории для полного исключения (вместе с содержимым)
EXCLUDE_DIRS=(
  "bin"
  "docs"
  ".git"
  "examples/archives"
  "examples/projects/qtwidgets_example/build-qt"
  "examples/projects/qtquick_example/build-qt"
)

# Расширения файлов для исключения
EXCLUDE_EXTS=(
  "png"
  "jpg"
  "jpeg"
  "gif"
  "svg"
  "ico"
  "pdf"
  "zip"
  "tar"
  "gz"
  "md"
  "log"
)

find_args=()

# Исключаем конкретные директории
for dir in "${EXCLUDE_DIRS[@]}"; do
  find_args+=( -path "./$dir" -prune -o )
done

# Дополнительное исключение: все build* директории в examples/projects/*/
find_args+=( -path "./examples/projects/*/build*" -prune -o )

# Исключаем конкретные файлы
for file in "${EXCLUDE_FILES[@]}"; do
  find_args+=( -path "./$file" -prune -o )
done

# Исключаем файлы по расширениям
for ext in "${EXCLUDE_EXTS[@]}"; do
  find_args+=( -name "*.$ext" -prune -o )
done

# Финальное условие: выбираем только обычные файлы
find_args+=( -type f -print )

find . "${find_args[@]}" | sort | while IFS= read -r filepath; do
  # Убираем префикс "./"
  display_path="${filepath#./}"
  
  echo ""
  echo "================================================================================"
  echo "FILE: $display_path"
  echo "================================================================================"
  
  # Выводим содержимое файла
  # Если файл бинарный или нечитаемый, показываем сообщение
  if cat -- "$filepath" 2>/dev/null; then
    true  # Успешно вывели содержимое
  else
    echo "[Ошибка чтения файла или бинарный файл]"
  fi
done

echo ""
echo "================================================================================"
echo "Сбор файлов завершен"
echo "================================================================================"
