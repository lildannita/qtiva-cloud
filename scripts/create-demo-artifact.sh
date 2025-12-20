#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="$SCRIPT_DIR/../examples"

echo "=== Создание демо-артефакта ==="

cd "$EXAMPLES_DIR/demo-app"

# Делаем скрипт исполняемым
chmod +x run-tests.sh

# Создаём архив
tar -czvf ../demo-app.tar.gz .

echo ""
echo "Артефакт создан: $EXAMPLES_DIR/demo-app.tar.gz"
echo "SHA256: $(sha256sum ../demo-app.tar.gz | cut -d' ' -f1)"
echo "Size: $(du -h ../demo-app.tar.gz | cut -f1)"
