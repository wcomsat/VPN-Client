#!/usr/bin/env bash
# ============================================================================
#  Загружает ядро туннеля для Android в android/app/src/main:
#    jniLibs/<abi>/libxray.so       — Xray-core (MPL-2.0)
#    jniLibs/<abi>/libtun2socks.so  — tun2socks (MIT)
#    assets/xray/geoip.dat, geosite.dat
#
#  Использование:  ./tools/fetch-core-android.sh
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
JNI="$ROOT/android/app/src/main/jniLibs"
ASSETS="$ROOT/android/app/src/main/assets/xray"
XRAY_VERSION="${XRAY_VERSION:-v26.3.27}"
TUN2SOCKS_VERSION="${TUN2SOCKS_VERSION:-v2.7.0}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$JNI/arm64-v8a" "$JNI/x86_64" "$ASSETS"

fetch_xray() {
    local abi="$1" asset="$2"
    local dir="$tmp/xray-$abi"
    mkdir -p "$dir"
    curl -fL --retry 3 -o "$tmp/xray-$abi.zip" \
        "https://github.com/XTLS/Xray-core/releases/download/$XRAY_VERSION/$asset"
    unzip -q "$tmp/xray-$abi.zip" -d "$dir"
    install -m 755 "$dir/xray" "$JNI/$abi/libxray.so"
    if [ ! -f "$ASSETS/geoip.dat" ]; then
        install -m 644 "$dir/geoip.dat" "$ASSETS/geoip.dat"
        install -m 644 "$dir/geosite.dat" "$ASSETS/geosite.dat"
    fi
}

fetch_tun2socks() {
    local abi="$1" asset="$2"
    local dir="$tmp/t2s-$abi"
    mkdir -p "$dir"
    curl -fL --retry 3 -o "$tmp/t2s-$abi.zip" \
        "https://github.com/xjasonlyu/tun2socks/releases/download/$TUN2SOCKS_VERSION/$asset"
    unzip -q "$tmp/t2s-$abi.zip" -d "$dir"
    local bin
    bin="$(find "$dir" -type f -name 'tun2socks*' | head -n 1)"
    if [ -z "$bin" ]; then
        echo "в архиве $asset не найден исполняемый файл" >&2
        exit 1
    fi
    install -m 755 "$bin" "$JNI/$abi/libtun2socks.so"
}

echo "[*] Xray-core $XRAY_VERSION"
fetch_xray arm64-v8a "Xray-android-arm64-v8a.zip"
fetch_xray x86_64 "Xray-android-amd64.zip"

echo "[*] tun2socks $TUN2SOCKS_VERSION"
fetch_tun2socks arm64-v8a "tun2socks-linux-arm64.zip"
fetch_tun2socks x86_64 "tun2socks-linux-amd64.zip"

echo "[ok] Готово:"
find "$JNI" "$ASSETS" -type f -printf '%p %s\n' | sort
