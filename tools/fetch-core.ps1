# ============================================================================
#  Загружает ядро туннеля в resources/ перед сборкой:
#    Xray-core (xray.exe, geoip.dat, geosite.dat, wintun.dll) — MPL-2.0
#    tun2socks (tun2socks.exe)                                 — MIT
#
#  Использование:  pwsh tools/fetch-core.ps1
# ============================================================================
param(
    [string]$Dest = (Join-Path $PSScriptRoot "../desktop/src-tauri/resources"),
    [string]$XrayVersion = "v26.3.27",
    [string]$Tun2socksVersion = "v2.7.0"
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Path $Dest -Force | Out-Null
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("vpn-core-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
    Write-Host "[•] Xray-core $XrayVersion"
    $xz = Join-Path $tmp "xray.zip"
    Invoke-WebRequest "https://github.com/XTLS/Xray-core/releases/download/$XrayVersion/Xray-windows-64.zip" -OutFile $xz
    Expand-Archive $xz -DestinationPath $tmp -Force
    foreach ($f in @("xray.exe", "geoip.dat", "geosite.dat", "wintun.dll")) {
        Copy-Item (Join-Path $tmp $f) $Dest -Force
    }

    Write-Host "[•] tun2socks $Tun2socksVersion"
    $tz = Join-Path $tmp "tun2socks.zip"
    Invoke-WebRequest "https://github.com/xjasonlyu/tun2socks/releases/download/$Tun2socksVersion/tun2socks-windows-amd64.zip" -OutFile $tz
    Expand-Archive $tz -DestinationPath (Join-Path $tmp "t2s") -Force
    Copy-Item (Join-Path $tmp "t2s/tun2socks-windows-amd64.exe") (Join-Path $Dest "tun2socks.exe") -Force

    Write-Host "[✓] Готово:"
    Get-ChildItem $Dest | Select-Object -ExpandProperty Name
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
