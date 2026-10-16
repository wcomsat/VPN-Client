# tools

Вспомогательные скрипты упаковки.

## `package-windows.ps1`

Собирает `vpn-client.exe` и складывает в `dist/vpn-client-windows.zip` вместе с
ядром Xray (`xray.exe`, `geoip.dat`, `geosite.dat`), `wintun.dll` и текстами
лицензий.

```powershell
pwsh tools/package-windows.ps1 `
  -XrayDir C:\downloads\xray `
  -WintunDll C:\downloads\wintun.dll
```

Что нужно подготовить заранее (скачать вручную, в репозиторий не кладём):
- **Xray-core** (v26.x) — `Xray-windows-64.zip`, извлечь `xray.exe` и geo-файлы;
- **WinTun** — `wintun.dll` (amd64) с <https://www.wintun.net/>.

Android-APK собирается Gradle'ом и упаковывается CI (см.
[`../.github/workflows/release.yml`](../.github/workflows/release.yml)).
