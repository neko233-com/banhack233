#!/bin/sh
set -eu

VERSION="${1:-latest}"
REPO="neko233-com/banhack233"
BINARY="banhack233"

detect_os() {
    case "$(uname -s)" in
        Linux*) echo "linux" ;;
        *) echo "unsupported" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) echo "amd64" ;;
    esac
}

latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/'
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
if [ "$OS" = "unsupported" ]; then
    echo "unsupported OS"
    exit 1
fi

if [ "$VERSION" = "latest" ]; then
    VERSION="$(latest_version)"
fi
VERSION="${VERSION#v}"

ASSET="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ASSET}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "download $URL"
curl -fsSL "$URL" -o "$TMP/$BINARY"
chmod +x "$TMP/$BINARY"
if [ -w /usr/local/bin ]; then
    mv "$TMP/$BINARY" /usr/local/bin/$BINARY
else
    sudo mv "$TMP/$BINARY" /usr/local/bin/$BINARY
fi

if [ ! -f /etc/banhack233/config.json ]; then
    sudo mkdir -p /etc/banhack233 /var/lib/banhack233
    sudo curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/configs/config.json.example" -o /etc/banhack233/config.json
fi

echo "installed: /usr/local/bin/$BINARY"
echo "edit: /etc/banhack233/config.json"
echo "enable: sudo banhack233 install-autostart"
