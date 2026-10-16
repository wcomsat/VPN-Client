# Сборка

## Общее

- Версия ядра: **Xray-core v26.x** (≥ v24.11.30 — нужен режим xHTTP `stream-one`).
- Секреты не нужны для сборки: конфиг подписки пользователь вводит в приложении.

## Windows (Go)

Требуется Go 1.22+ (Windows, amd64).

```powershell
cd windows
go mod download
go build -trimpath -ldflags "-s -w" -o bin/vpn-client.exe .
```

Комплект для раздачи (см. `tools/package-windows.ps1`):
- `vpn-client.exe`
- ядро `xray.exe` и `geoip.dat`/`geosite.dat`
- `tun2socks.exe` (v2.7.0, MIT) — для режима TUN
- `wintun.dll` (prebuilt, x64) — драйвер для режима TUN
- `THIRD-PARTY-LICENSES/` (тексты лицензий)

Скачать для упаковки: [Xray-core Windows](https://github.com/XTLS/Xray-core/releases)
(`Xray-windows-64.zip`) и [tun2socks](https://github.com/xjasonlyu/tun2socks/releases)
(`tun2socks-windows-amd64.zip`; внутри `tun2socks-windows-amd64.exe` → переименовать
в `tun2socks.exe`).

## Android (Kotlin/Gradle)

Требуется JDK 17, Android SDK (compileSdk 34), Gradle 8.x.

```bash
cd android
./gradlew :app:assembleRelease   # подписанный APK (keystore из keystore.properties)
./gradlew :app:assembleDebug     # отладочный APK
```

`libXray` подключается как локальная AAR-зависимость (`android/app/libs/`):
положите собранный из [XTLS/libXray](https://github.com/XTLS/libXray) артефакт.
Подпись: файл `keystore.properties` (не коммитится) со ссылкой на `.jks`.

## CI

[`.github/workflows/release.yml`](../.github/workflows/release.yml) собирает
`vpn-client-windows.zip` и `app-release.apk` по тегу и прикладывает их к
GitHub Release вместе с `SHA256SUMS`.

## Проверки (по мере реализации)

- Парсер `vless://` — юнит-тесты на общих фикстурах (`shared/fixtures`).
- Windows: `go vet ./...`, `go test ./...`.
- Android: `./gradlew test`.
