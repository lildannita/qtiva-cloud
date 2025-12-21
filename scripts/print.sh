#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(pwd)"
cd "$ROOT_DIR"

EXCLUDE_FILES=(
  ".env"
  "ToR.md"
  "ToR.pdf"
  "LICENSE"
  "go.mod"
  "go.sum"
)

EXCLUDE_DIRS=(
  "bin"
  "docs"
  ".git"
)

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

# Сборка условий исключений для find (в относительных путях: ./...)
find_excludes=()

# исключаем директории (и всё внутри)
for d in "${EXCLUDE_DIRS[@]}"; do
  find_excludes+=( -path "./$d" -o -path "./$d/*" )
done

# исключаем конкретные файлы в корне (./file)
for f in "${EXCLUDE_FILES[@]}"; do
  find_excludes+=( -o -path "./$f" )
done

# исключаем файлы по расширениям
for ext in "${EXCLUDE_EXTS[@]}"; do
  find_excludes+=( -o -name "*.$ext" )
done

find . \
  \( "${find_excludes[@]}" \) -prune -o \
  -type f -print0 \
| while IFS= read -r -d '' file; do
    echo
    echo "================================================================================"
    echo "FILE: ${file#./}"
    echo "================================================================================"
    cat -- "$file"
  done