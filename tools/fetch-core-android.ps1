# ============================================================================
#  Загружает ядро туннеля для Android в android/app/src/main:
#    jniLibs/<abi>/libxray.so       — Xray-core (MPL-2.0)
#    jniLibs/<abi>/libtun2socks.so  — tun2socks (MIT)
#    assets/xray/geoip.dat, geosite.dat
#
#  Использование:  pwsh tools/fetch-core-android.ps1
# ============================================================================
param(
    [string]$XrayVersion = "v26.3.27",
    [string]$Tun2socksVersion = "v2.7.0"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$jni = Join-Path $root "android/app/src/main/jniLibs"
$assets = Join-Path $root "android/app/src/main/assets/xray"
New-Item -ItemType Directory -Force -Path `
    (Join-Path $jni "arm64-v8a"), (Join-Path $jni "x86_64"), $assets | Out-Null

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("vpn-android-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

function Get-File([string]$Url, [string]$Out) {
    Invoke-WebRequest $Url -OutFile $Out
}

try {
    Write-Host "[*] Xray-core $XrayVersion"
    foreach ($item in @(
            @{ Abi = "arm64-v8a"; Asset = "Xray-android-arm64-v8a.zip" },
            @{ Abi = "x86_64"; Asset = "Xray-android-amd64.zip" }
        )) {
        $zip = Join-Path $tmp ("xray-" + $item.Abi + ".zip")
        $dir = Join-Path $tmp ("xray-" + $item.Abi)
        Get-File "https://github.com/XTLS/Xray-core/releases/download/$XrayVersion/$($item.Asset)" $zip
        Expand-Archive $zip -DestinationPath $dir -Force
        Copy-Item (Join-Path $dir "xray") (Join-Path $jni "$($item.Abi)/libxray.so") -Force
        if (-not (Test-Path (Join-Path $assets "geoip.dat"))) {
            Copy-Item (Join-Path $dir "geoip.dat") $assets -Force
            Copy-Item (Join-Path $dir "geosite.dat") $assets -Force
        }
    }

    Write-Host "[*] tun2socks $Tun2socksVersion"
    foreach ($item in @(
            @{ Abi = "arm64-v8a"; Asset = "tun2socks-linux-arm64.zip" },
            @{ Abi = "x86_64"; Asset = "tun2socks-linux-amd64.zip" }
        )) {
        $zip = Join-Path $tmp ("t2s-" + $item.Abi + ".zip")
        $dir = Join-Path $tmp ("t2s-" + $item.Abi)
        Get-File "https://github.com/xjasonlyu/tun2socks/releases/download/$Tun2socksVersion/$($item.Asset)" $zip
        Expand-Archive $zip -DestinationPath $dir -Force
        $bin = Get-ChildItem $dir -Recurse -File -Filter "tun2socks*" | Select-Object -First 1
        if (-not $bin) { throw "в архиве $($item.Asset) не найден исполняемый файл" }
        Copy-Item $bin.FullName (Join-Path $jni "$($item.Abi)/libtun2socks.so") -Force
    }

    Write-Host "[ok] Готово:"
    Get-ChildItem $jni, $assets -Recurse -File | Select-Object FullName, Length
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
