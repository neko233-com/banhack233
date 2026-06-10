#!/bin/sh
set -eu

VERSION="${1:-latest}"
REPO="neko233-com/banhack233"
BINARY="banhack233"

detect_os() {
    case "$(uname -s)" in
        Linux*) echo "linux" ;;
        Darwin*) echo "darwin" ;;
        CYGWIN*|MINGW*|MSYS*) echo "windows" ;;
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
[ "$OS" = "windows" ] && ASSET="${ASSET}.exe"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ASSET}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "download $URL"
curl -fsSL "$URL" -o "$TMP/$BINARY"
chmod +x "$TMP/$BINARY"
if [ "$OS" = "windows" ]; then
    INSTALL_DIR="${LOCALAPPDATA:-$HOME/AppData/Local}/banhack233"
    CONFIG_DIR="$INSTALL_DIR"
    mkdir -p "$INSTALL_DIR" "$CONFIG_DIR"
    mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY.exe"
else
    INSTALL_DIR="/usr/local/bin"
    CONFIG_DIR="/etc/banhack233"
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
    else
        sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
    fi
    if [ ! -d "$CONFIG_DIR" ]; then
        sudo mkdir -p "$CONFIG_DIR"
    fi
    if [ "$OS" = "linux" ]; then
        GEOIP_DIR="/var/lib/banhack233"
        sudo mkdir -p "$GEOIP_DIR"
    elif [ "$OS" = "darwin" ]; then
        GEOIP_DIR="/usr/local/var/banhack233"
        sudo mkdir -p "$GEOIP_DIR"
    fi
    GEOIP_DB="${GEOIP_DIR}/ip2region_v4.xdb"
    if [ ! -f "$GEOIP_DB" ]; then
        echo "download ip2region db"
        sudo curl -fsSL "https://github.com/lionsoul2014/ip2region/raw/master/data/ip2region_v4.xdb" -o "$GEOIP_DB"
    fi
fi

CONFIG_PATH="$CONFIG_DIR/config.json"
if [ ! -f "$CONFIG_PATH" ]; then
    if [ "$OS" = "windows" ]; then
        curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/configs/config.json.example" -o "$CONFIG_PATH"
    else
        sudo curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/configs/config.json.example" -o "$CONFIG_PATH"
    fi
fi

echo "installed: $INSTALL_DIR/$BINARY"
echo "config: $CONFIG_PATH"
echo "next: banhack233 status"
