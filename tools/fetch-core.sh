#!/usr/bin/env bash
# ============================================================================
#  Загружает ядро туннеля для macOS в resources/ перед сборкой:
#    Xray-core (xray, geoip.dat, geosite.dat) — MPL-2.0
#    tun2socks (tun2socks)                     — MIT
#
#  Использование:  ./tools/fetch-core.sh
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="${1:-$ROOT/desktop/src-tauri/resources}"
XRAY_VERSION="${XRAY_VERSION:-v26.3.27}"
TUN2SOCKS_VERSION="${TUN2SOCKS_VERSION:-v2.7.0}"

case "$(uname -m)" in
  arm64)
    XRAY_ASSET="Xray-macos-arm64-v8a.zip"
    TUN2SOCKS_ASSET="tun2socks-darwin-arm64.zip"
    ;;
  x86_64)
    XRAY_ASSET="Xray-macos-64.zip"
    TUN2SOCKS_ASSET="tun2socks-darwin-amd64.zip"
    ;;
  *)
    echo "Неподдерживаемая архитектура: $(uname -m)" >&2
    exit 1
    ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$DEST"

echo "[*] Xray-core $XRAY_VERSION ($XRAY_ASSET)"
curl -fL --retry 3 -o "$tmp/xray.zip" \
  "https://github.com/XTLS/Xray-core/releases/download/$XRAY_VERSION/$XRAY_ASSET"
mkdir -p "$tmp/xray"
unzip -q "$tmp/xray.zip" -d "$tmp/xray"
install -m 755 "$tmp/xray/xray" "$DEST/xray"
install -m 644 "$tmp/xray/geoip.dat" "$tmp/xray/geosite.dat" "$DEST/"

echo "[*] tun2socks $TUN2SOCKS_VERSION ($TUN2SOCKS_ASSET)"
curl -fL --retry 3 -o "$tmp/t2s.zip" \
  "https://github.com/xjasonlyu/tun2socks/releases/download/$TUN2SOCKS_VERSION/$TUN2SOCKS_ASSET"
mkdir -p "$tmp/t2s"
unzip -q "$tmp/t2s.zip" -d "$tmp/t2s"
bin="$(find "$tmp/t2s" -type f -name 'tun2socks*' ! -name '*.txt' | head -n 1)"
if [ -z "$bin" ]; then
  echo "В архиве tun2socks не найден исполняемый файл" >&2
  exit 1
fi
install -m 755 "$bin" "$DEST/tun2socks"

echo "[ok] Готово:"
ls -l "$DEST"
