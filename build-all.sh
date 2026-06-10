#!/bin/sh
set -eu

mkdir -p dist
VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-s -w -X github.com/neko233-com/banhack233/internal/version.Version=$VERSION -X github.com/neko233-com/banhack233/internal/version.Commit=$COMMIT -X github.com/neko233-com/banhack233/internal/version.Date=$DATE"

for os in linux windows; do
  for arch in amd64 arm64; do
    ext=""
    [ "$os" = "windows" ] && ext=".exe"
    echo "build $os/$arch"
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "dist/banhack233-$os-$arch$ext" ./cmd/banhack233
  done
done
