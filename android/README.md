# VPN-Client для Android

Минимальный клиент: вставить подписку, выбрать канал (xHTTP / Vision), подключить.

## Стек
- **Kotlin + Gradle**
- **libXray** (MIT) — ядро Xray-core для Android (AAR)
- **tun2socks** — маршрутизация tun → SOCKS (нативный модуль, добавляется отдельно)

## Текущее состояние каркаса

Реализовано:
- `Channel.kt` — модель канала и парсер `vless://`;
- `ClientConfig.kt` — сборка конфига Xray-core (совпадает с
  `shared/fixtures/expected/*.json`);
- `MainActivity.kt` — минимальный UI (подписка, канал, connect);
- `TunnelVpnService.kt` — каркас `VpnService` (туннель — TODO Фазы 2).

Ещё не сделано:
- подключение `libXray` и запуск ядра;
- `tun2socks` и маршруты/DNS;
- подпись release-APK (`keystore.properties`).

## libXray

Соберите AAR из [XTLS/libXray](https://github.com/XTLS/libXray) и положите в
`android/app/libs/libXray.aar`, затем раскомментируйте зависимость в
`app/build.gradle.kts`.

## Сборка

```bash
cd android
./gradlew :app:assembleDebug
```

## Структура

```
app/src/main/java/com/wcomsat/vpnclient/
  Channel.kt            модель + парсер vless://
  ClientConfig.kt       конфиг Xray-core из канала
  MainActivity.kt       UI
  TunnelVpnService.kt   системный VPN (каркас)
```
