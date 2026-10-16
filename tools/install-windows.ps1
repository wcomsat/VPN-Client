# ============================================================================
#  Установка Windows-клиента: копирует файлы в профиль пользователя и создаёт
#  ярлыки на рабочем столе и в меню «Пуск» (запуск без окна консоли, с иконкой).
#
#  Запуск из папки с распакованным пакетом:
#     pwsh -ExecutionPolicy Bypass -File install-windows.ps1
#
#  Удалить:  pwsh -File install-windows.ps1 -Uninstall
# ============================================================================
param(
    [string]$Source      = $PSScriptRoot,
    [string]$InstallDir  = (Join-Path $env:LOCALAPPDATA "VPN-Client"),
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
$exe     = Join-Path $InstallDir "vpn-client.exe"
$vbs     = Join-Path $InstallDir "VPN.vbs"
$ico     = Join-Path $InstallDir "vpn-client.ico"
$desktop = [Environment]::GetFolderPath("Desktop")
$programs = [Environment]::GetFolderPath("Programs")
$startMenu = Join-Path $programs "VPN-Client"
$shortcutName = "VPN.lnk"

function Remove-Link($path) { if (Test-Path $path) { Remove-Item $path -Force } }

if ($Uninstall) {
    Remove-Link (Join-Path $desktop $shortcutName)
    Remove-Link (Join-Path $startMenu $shortcutName)
    if (Test-Path $startMenu) { Remove-Item $startMenu -Recurse -Force }
    if (Test-Path $InstallDir) { Remove-Item $InstallDir -Recurse -Force }
    Write-Host "[✓] VPN-Client удалён"
    exit 0
}

if (-not (Test-Path (Join-Path $Source "vpn-client.exe"))) {
    throw "Не найден vpn-client.exe в $Source — распакуйте пакет целиком и запустите установщик из его папки."
}

Write-Host "[•] Копирование в $InstallDir"
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item (Join-Path $Source "*") $InstallDir -Recurse -Force

Write-Host "[•] Создание ярлыков"
$ws = New-Object -ComObject WScript.Shell
foreach ($dir in @($desktop, $startMenu)) {
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    $lnk = $ws.CreateShortcut((Join-Path $dir $shortcutName))
    $lnk.TargetPath       = Join-Path $env:SystemRoot "System32\wscript.exe"
    $lnk.Arguments        = '"' + $vbs + '"'
    $lnk.WorkingDirectory = $InstallDir
    if (Test-Path $ico) { $lnk.IconLocation = $ico }
    $lnk.Description      = "VPN-Client"
    $lnk.Save()
}

Write-Host "[✓] Готово."
Write-Host "    Ярлык «VPN» на рабочем столе и в меню Пуск."
Write-Host "    Запуск открывает интерфейс в браузере, значок остаётся в трее."
Write-Host "    Для режима TUN запускайте ярлык от имени администратора."
