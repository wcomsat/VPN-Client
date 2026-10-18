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
    $expected = Join-Path $nsis "VPN_${ver}_x64-setup.exe"
    if (Test-Path $expected) {
        $setup = Get-Item $expected
    } else {
        # запасной вариант — самый свежий установщик
        $setup = Get-ChildItem $nsis -Filter "*-setup.exe" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    }
    if (-not $setup) { throw "установщик не найден в $nsis" }
    if ($setup.Name -notlike "*${ver}*") {
        throw "найден установщик другой версии: $($setup.Name) (ожидался VPN_${ver}_x64-setup.exe)"
    }
    $sigPath = "$($setup.FullName).sig"
    if (-not (Test-Path $sigPath)) { throw "нет подписи $sigPath" }

    $dist = "dist-release"
    if (Test-Path $dist) { Remove-Item $dist -Recurse -Force }
    New-Item -ItemType Directory -Path $dist | Out-Null
    Copy-Item $setup.FullName $dist
    Copy-Item $sigPath $dist

    $url = "https://github.com/$RepoSlug/releases/download/$tag/$($setup.Name)"

    # Сохраняем платформы (macOS), уже добавленные в latest.json этого же тега,
    # чтобы релиз Windows не затёр запись darwin-aarch64.
    $platforms = [ordered]@{}
    $prev = Join-Path $env:TEMP ("latest-" + [guid]::NewGuid().ToString("N") + ".json")
    try {
        Invoke-WebRequest "https://github.com/$RepoSlug/releases/download/$tag/latest.json" -OutFile $prev -UseBasicParsing
        $old = Get-Content $prev -Raw | ConvertFrom-Json
        if ($old.platforms) {
            foreach ($p in $old.platforms.PSObject.Properties) {
                if ($p.Name -ne "windows-x86_64") { $platforms[$p.Name] = $p.Value }
            }
        }
    } catch { }
    finally { Remove-Item $prev -ErrorAction SilentlyContinue }

    $platforms["windows-x86_64"] = [ordered]@{
        signature = (Get-Content $sigPath -Raw).Trim()
        url       = $url
    }
    $latest = [ordered]@{
        version   = $ver
        notes     = "VPN $ver"
        pub_date  = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        platforms = $platforms
    }
    $latest | ConvertTo-Json -Depth 8 | Out-File -Encoding utf8 (Join-Path $dist "latest.json")

    if (-not $NoGit) {
        git add -A
        git commit -m "release: v$ver" --allow-empty | Out-Null
        git tag -f -a $tag -m "VPN $tag"
        git push origin main
        if ($LASTEXITCODE -ne 0) { throw "git push main failed" }
        git push -f origin $tag
        if ($LASTEXITCODE -ne 0) { throw "git push tag failed" }
    }

    if (-not $NoUpload) {
        $assets = @(
            (Join-Path $dist $setup.Name),
            (Join-Path $dist "$($setup.Name).sig"),
            (Join-Path $dist "latest.json")
        )
        $exists = $false
        gh release view $tag --repo $RepoSlug *> $null
        if ($LASTEXITCODE -eq 0) { $exists = $true }

        if ($exists) {
            gh release upload $tag --repo $RepoSlug @assets --clobber
        } else {
            gh release create $tag --repo $RepoSlug @assets --title "VPN $tag" --notes "VPN $ver"
        }
        if ($LASTEXITCODE -ne 0) { throw "не удалось опубликовать релиз (gh exit $LASTEXITCODE)" }
        Write-Host "[✓] Релиз: https://github.com/$RepoSlug/releases/tag/$tag"
    } else {
        Write-Host "[✓] Собрано: $($setup.FullName)"
    }
} finally {
    Remove-Item "updater.json" -ErrorAction SilentlyContinue
    Pop-Location
}
