#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-dev}"
COMMIT="none"

if command -v git >/dev/null 2>&1; then
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    COMMIT="$(git rev-parse --short HEAD)"
  fi
fi

mkdir -p bin

LDFLAGS="-X github.com/lildannita/qtiva-cloud/internal/buildinfo.Version=${VERSION} -X github.com/lildannita/qtiva-cloud/internal/buildinfo.Commit=${COMMIT}"

echo "Сборка: version=${VERSION} commit=${COMMIT}"
go build -ldflags "${LDFLAGS}" -o bin/qtiva-manager ./cmd/qtiva-manager
go build -ldflags "${LDFLAGS}" -o bin/qtiva-agent   ./cmd/qtiva-agent
echo "Бинарники собраны и лежат в ./bin"
