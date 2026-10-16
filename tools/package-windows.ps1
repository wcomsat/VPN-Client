# ============================================================================
#  Упаковка Windows-клиента в раздаваемый zip.
#
#  Собирает vpn-client.exe и складывает рядом:
#    • xray.exe, geoip.dat, geosite.dat  (ядро Xray-core, MPL-2.0)
#    • wintun.dll                         (prebuilt, x64)
#    • THIRD-PARTY-LICENSES/              (тексты лицензий)
#
#  Использование:
#    pwsh tools/package-windows.ps1 -XrayDir C:\downloads\xray -WintunDll C:\downloads\wintun.dll
# ============================================================================
param(
    [string]$XrayDir   = "vendor/xray",
    [string]$WintunDll = "vendor/wintun/wintun.dll",
    [string]$Out       = "dist/vpn-client-windows.zip"
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$stage = Join-Path $repo "dist/stage"
$dist = Join-Path $repo "dist"

Write-Host "[•] Сборка vpn-client.exe"
Push-Location (Join-Path $repo "windows")
go build -trimpath -ldflags "-s -w" -o (Join-Path $repo "windows/dist/vpn-client.exe") .
Pop-Location

Write-Host "[•] Подготовка каталога"
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Path $stage | Out-Null

Copy-Item (Join-Path $repo "windows/dist/vpn-client.exe") $stage
foreach ($f in @("xray.exe", "geoip.dat", "geosite.dat")) {
    $p = Join-Path $XrayDir $f
    if (Test-Path $p) { Copy-Item $p $stage } else { Write-Warning "нет $p" }
}
if (Test-Path $WintunDll) { Copy-Item $WintunDll $stage } else { Write-Warning "нет $WintunDll" }

$licDir = Join-Path $stage "THIRD-PARTY-LICENSES"
New-Item -ItemType Directory -Path $licDir | Out-Null
Copy-Item (Join-Path $repo "NOTICE") $licDir
Copy-Item (Join-Path $repo "LICENSE") $licDir

Write-Host "[•] Упаковка $Out"
New-Item -ItemType Directory -Path $dist -Force | Out-Null
if (Test-Path (Join-Path $repo $Out)) { Remove-Item (Join-Path $repo $Out) }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath (Join-Path $repo $Out)

Write-Host "[✓] Готово: $Out"
