# ============================================================================
#  Локальная сборка и публикация релиза (быстрее CI).
#
#  Делает: поднимает версию (если указана) → собирает подписанный установщик
#  → формирует latest.json → коммит/тег/пуш → загружает ассеты в GitHub Release.
#
#  Использование:
#     pwsh tools/release.ps1 -Version 1.0.3
#     pwsh tools/release.ps1                  # версия из tauri.conf.json
#     pwsh tools/release.ps1 -NoUpload        # только собрать (без GitHub)
#     pwsh tools/release.ps1 -NoGit           # не коммитить/не тегировать
# ============================================================================
param(
    [string]$Version,
    [switch]$NoUpload,
    [switch]$NoGit
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$RepoSlug = "wcomsat/VPN-Client"
Push-Location $repo
try {
    $confPath = "desktop/src-tauri/tauri.conf.json"
    $conf = Get-Content $confPath -Raw | ConvertFrom-Json
    $ver = if ($Version) { $Version.TrimStart("v") } else { $conf.version }
    $tag = "v$ver"

    if ($Version) {
        (Get-Content "Cargo.toml" -Raw) -replace 'version = "[0-9]+\.[0-9]+\.[0-9]+"', "version = `"$ver`"" | Set-Content "Cargo.toml"
        (Get-Content $confPath -Raw) -replace '"version": "[0-9]+\.[0-9]+\.[0-9]+"', "`"version`": `"$ver`"" | Set-Content $confPath
        (Get-Content "desktop/package.json" -Raw) -replace '"version": "[0-9]+\.[0-9]+\.[0-9]+"', "`"version`": `"$ver`"" | Set-Content "desktop/package.json"
        Write-Host "[•] Версия: $ver"
    }

    if (-not (Test-Path "desktop/src-tauri/resources/xray.exe")) {
        Write-Host "[•] Нет ядра в resources — скачиваю…"
        & "$PSScriptRoot/fetch-core.ps1"
    }

    $keyPath = "$env:USERPROFILE\.tauri\vpn-client.key"
    if (-not $env:TAURI_SIGNING_PRIVATE_KEY) {
        if (-not (Test-Path $keyPath)) { throw "Нет ключа подписи: $keyPath" }
        $env:TAURI_SIGNING_PRIVATE_KEY = (Get-Content $keyPath -Raw).Trim()
    }
    if (-not $env:TAURI_SIGNING_PRIVATE_KEY_PASSWORD) {
        $env:TAURI_SIGNING_PRIVATE_KEY_PASSWORD = "vpnclient123"
    }

    '{"bundle":{"createUpdaterArtifacts":true}}' | Out-File -Encoding utf8 "updater.json"

    Write-Host "[•] Сборка v$ver (с подписью)…"
    Push-Location desktop
    npx --yes "@tauri-apps/cli@^2" build --config (Join-Path $repo "updater.json")
    if ($LASTEXITCODE -ne 0) { throw "tauri build failed" }
    Pop-Location

    $nsis = "target/release/bundle/nsis"
    $setup = Get-ChildItem $nsis -Filter "*-setup.exe" | Select-Object -First 1
    if (-not $setup) { throw "установщик не найден в $nsis" }
    $sigPath = "$($setup.FullName).sig"
    if (-not (Test-Path $sigPath)) { throw "нет подписи $sigPath" }

    $dist = "dist-release"
    if (Test-Path $dist) { Remove-Item $dist -Recurse -Force }
    New-Item -ItemType Directory -Path $dist | Out-Null
    Copy-Item $setup.FullName $dist
    Copy-Item $sigPath $dist

    $url = "https://github.com/$RepoSlug/releases/download/$tag/$($setup.Name)"
    $latest = [ordered]@{
        version   = $ver
        notes     = "VPN $ver"
        pub_date  = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        platforms = [ordered]@{
            "windows-x86_64" = [ordered]@{
                signature = (Get-Content $sigPath -Raw).Trim()
                url       = $url
            }
        }
    }
    $latest | ConvertTo-Json -Depth 6 | Out-File -Encoding utf8 (Join-Path $dist "latest.json")

    if (-not $NoGit) {
        git add Cargo.toml Cargo.lock desktop/src-tauri/tauri.conf.json desktop/package.json
        git commit -m "release: v$ver" --allow-empty | Out-Null
        git tag -f -a $tag -m "VPN $tag"
        git push origin main
        git push -f origin $tag
    }

    if (-not $NoUpload) {
        $assets = @(
            (Join-Path $dist $setup.Name),
            (Join-Path $dist "$($setup.Name).sig"),
            (Join-Path $dist "latest.json")
        )
        $exists = $false
        try { gh release view $tag --repo $RepoSlug *> $null; $exists = $true } catch {}
        if ($exists) {
            gh release upload $tag --repo $RepoSlug @assets --clobber
        } else {
            gh release create $tag --repo $RepoSlug @assets --title "VPN $tag" --notes "VPN $ver"
        }
        Write-Host "[✓] Релиз: https://github.com/$RepoSlug/releases/tag/$tag"
    } else {
        Write-Host "[✓] Собрано: $($setup.FullName)"
    }
} finally {
    Remove-Item "updater.json" -ErrorAction SilentlyContinue
    Pop-Location
}
