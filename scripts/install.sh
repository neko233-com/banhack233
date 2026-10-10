#!/bin/sh
set -eu
umask 077
VERSION="${1:-latest}"
REPO="neko233-com/banhack233"
case "$(uname -s)" in Linux*) OS=linux ;; Darwin*) OS=darwin ;; *) echo 'Use scripts/install.ps1 on Windows.' >&2; exit 1 ;; esac
case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; *) echo 'Unsupported CPU architecture' >&2; exit 1 ;; esac
fetch() { curl --proto '=https' --tlsv1.2 -fsSL --connect-timeout 15 --max-time 180 "$@"; }
elevate() { if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo "$@"; fi; }
if [ "$VERSION" = latest ]; then
    VERSION="$(fetch "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
fi
VERSION="v${VERSION#v}"
if ! printf '%s\n' "$VERSION" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then echo 'A stable release tag is required' >&2; exit 1; fi
ASSET="banhack233-$OS-$ARCH"
BASE="https://github.com/$REPO/releases/download/$VERSION"
TMP="$(mktemp -d)"
STAGE=""
cleanup() {
    rm -f "$TMP/$ASSET" "$TMP/SHA256SUMS.txt"
    rmdir "$TMP"
    if [ -n "$STAGE" ]; then elevate rm -f "$STAGE"; fi
}
trap cleanup EXIT HUP INT TERM
fetch "$BASE/$ASSET" -o "$TMP/$ASSET"
fetch "$BASE/SHA256SUMS.txt" -o "$TMP/SHA256SUMS.txt"
EXPECTED="$(awk -v name="$ASSET" '$2 == name || $2 == "*"name { sum=$1; count++ } END { if (count != 1) exit 1; print sum }' "$TMP/SHA256SUMS.txt")"
if ! printf '%s\n' "$EXPECTED" | grep -Eq '^[0-9a-f]{64}$'; then echo 'Invalid SHA256 manifest' >&2; exit 1; fi
if command -v sha256sum >/dev/null 2>&1; then ACTUAL="$(sha256sum "$TMP/$ASSET" | awk '{print $1}')"; else ACTUAL="$(shasum -a 256 "$TMP/$ASSET" | awk '{print $1}')"; fi
[ "$EXPECTED" = "$ACTUAL" ] || { echo 'SHA256 mismatch; installation refused' >&2; exit 1; }
chmod 755 "$TMP/$ASSET"
"$TMP/$ASSET" version
INSTALL_DIR=/usr/local/bin
CONFIG_DIR=/etc/banhack233
[ "$OS" = darwin ] && CONFIG_DIR=/usr/local/etc/banhack233
CONFIG_PATH="$CONFIG_DIR/config.json"
elevate mkdir -p "$INSTALL_DIR" "$CONFIG_DIR"
# Reuse the historical macOS installer config rather than creating a second policy.
if [ "$OS" = darwin ] && [ -f /etc/banhack233/config.json ]; then CONFIG_PATH=/etc/banhack233/config.json; fi
if [ -f "$CONFIG_PATH" ]; then
    elevate "$TMP/$ASSET" config-check -config "$CONFIG_PATH"
fi
STAGE="$(elevate mktemp "$INSTALL_DIR/.banhack233-install.XXXXXX")"
elevate cp "$TMP/$ASSET" "$STAGE"
elevate chmod 755 "$STAGE"
if [ -f "$INSTALL_DIR/banhack233" ]; then elevate cp "$INSTALL_DIR/banhack233" "$INSTALL_DIR/banhack233.installer-backup"; fi
elevate mv -f "$STAGE" "$INSTALL_DIR/banhack233"
STAGE=""
if [ ! -f "$CONFIG_PATH" ]; then elevate "$INSTALL_DIR/banhack233" init-config -config "$CONFIG_PATH"; fi
printf 'Installed %s. Configuration: %s\n' "$VERSION" "$CONFIG_PATH"
printf 'Review: sudo banhack233 config-check -config %s\n' "$CONFIG_PATH"
printf 'Start: sudo banhack233 install-autostart -config %s\n' "$CONFIG_PATH"
printf 'Daily verified updates: sudo banhack233 auto-update -enable -config %s\n' "$CONFIG_PATH"
printf 'Existing daemon: restart after upgrade. New installations start in dry_run. GeoIP database is optional; configure it separately.\n'
