#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RUNNER_DIR="$SCRIPT_DIR/../runner"

cd "$RUNNER_DIR"

echo "=== Сборка runner образов ==="

# Manjaro + Wayland
echo ""
echo ">>> Сборка qtiva-runner:manjaro-qt5.15-wayland"
docker build -f Dockerfile.manjaro-wayland -t qtiva-runner:manjaro-qt5.15-wayland .

# Manjaro + X11
echo ""
echo ">>> Сборка qtiva-runner:manjaro-qt5.15-x11"
docker build -f Dockerfile.manjaro-x11 -t qtiva-runner:manjaro-qt5.15-x11 .

# Ubuntu + Wayland
echo ""
echo ">>> Сборка qtiva-runner:ubuntu-qt5.15-wayland"
docker build -f Dockerfile.ubuntu-wayland -t qtiva-runner:ubuntu-qt5.15-wayland .

# Ubuntu + X11 (можно добавить позже)
# echo ""
# echo ">>> Сборка qtiva-runner:ubuntu-qt5.15-x11"
# docker build -f Dockerfile.ubuntu-x11 -t qtiva-runner:ubuntu-qt5.15-x11 .

echo ""
echo "=== Все образы собраны ==="
docker images | grep qtiva-runner
