# VPN

Клиент для личного VPN-сервиса ([VPN-Server](../VPN-Server)). Один клик —
и весь трафик идёт через ваш сервер. Современный интерфейс, простое приложение,
установщик и авто-обновления.

Сейчас поддерживаются **Windows** и **macOS (Apple Silicon)**. Android — в плане.

## Стек

- **Rust** + **Tauri v2** — приложение и нативное окно (WebView2 / WKWebView).
- Общее ядро [`core/`](core) — разбор подписки `vless://` и сборка конфига
  Xray-core (используется всеми платформами).
- Внешнее ядро туннеля: **Xray-core** (MPL-2.0) + **tun2socks** (MIT).
  - Windows: `xray.exe`, `tun2socks.exe`, **WinTun** (`wintun.dll`).
  - macOS: `xray`, `tun2socks`, системный `utun`.
- Фронтенд — обычные HTML/CSS/JS (без сборщиков), в каталоге [`desktop/ui`](desktop/ui).

Используются только permissive-лицензии (см. [NOTICE](NOTICE)).

## Состав репозитория

```
core/     Rust-крейт: парсер vless:// + генератор конфига Xray (общий для платформ)
desktop/  Приложение (Tauri v2): ui/ (фронтенд) + src-tauri/ (Rust)
shared/   Контракт подписки и тест-фикстуры (vless -> конфиг ядра)
tools/    Загрузка ядра туннеля и локальный релиз
```

## Режимы подключения

- **Прокси** — системный HTTP/HTTPS-прокси (браузеры и приложения).
  - Windows: реестр WinINET, без прав администратора.
  - macOS: `networksetup`, потребуется пароль администратора.
- **TUN** — весь трафик через туннель (нужны права администратора).
  - Windows: WinTun + tun2socks.
  - macOS: `utun` + tun2socks, пароль запрашивается при подключении и отключении.

## Разработка

```bash
cargo test -p vpn-core                    # парсер/конфиг против shared/fixtures
cargo run -p vpn-desktop -- selftest      # проверка бэкенда без окна

# ядро туннеля в resources/
pwsh tools/fetch-core.ps1                 # Windows
bash tools/fetch-core.sh                  # macOS

cd desktop
npx @tauri-apps/cli dev                   # запуск приложения
npx @tauri-apps/cli build                 # установщик / DMG
```

Готовые артефакты:

- Windows: `target/release/bundle/nsis/VPN_<версия>_x64-setup.exe`;
- macOS: `target/release/bundle/dmg/VPN_<версия>_aarch64.dmg`.

## Релиз и авто-обновление

Обновления раздаются через GitHub Releases. Windows-клиент при запуске проверяет
`latest.json` и при наличии обновления ставит его сам.

- Windows: релиз собирается локально — `pwsh tools/release.ps1 -Version x.y.z`.
- macOS: workflow [`.github/workflows/release-macos.yml`](.github/workflows/release-macos.yml)
  собирает DMG на `macos-14` и загружает его в тот же релиз (тег `v*`).
  Авто-обновление macOS пока не подключено: обновление вручную через новый DMG.

Для подписи обновлений Windows в настройках репозитория (Secrets) задайте:

- `TAURI_SIGNING_PRIVATE_KEY` — содержимое приватного ключа;
- `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` — пароль ключа.

Публичный ключ вписан в `desktop/src-tauri/tauri.conf.json` (`plugins.updater.pubkey`).

## Установка (для пользователя)

- **Windows:** скачать `VPN_<версия>_x64-setup.exe` из последнего релиза и
  запустить. Установка идёт в профиль пользователя.
- **macOS:** скачать `VPN_<версия>_aarch64.dmg`, открыть, перетащить **VPN**
  в **Applications**. Сборка не подписана Apple: при первом запуске —
  правый клик по приложению → **Открыть** (или «Системные настройки →
  Конфиденциальность и безопасность → Всё равно открыть»).

## План

1. **M0** — каркас репозитория и приложения. ✅
2. **M1** — общее ядро: парсер и конфиг (тесты на фикстурах). ✅
3. **M2** — логика Windows: ядро, прокси, TUN, состояние. ✅
4. **M3** — современный интерфейс. ✅
5. **M4** — установщик, иконка, ярлык. ✅
6. **M5** — авто-обновление и CI. ✅ (Windows)
7. **M6** — macOS Apple Silicon: прокси + TUN, DMG. ✅
8. **M7** — Android (Kotlin `VpnService` + Compose + общее Rust-ядро).

## Лицензия

Собственный код — [MIT](LICENSE). Сторонние компоненты — [NOTICE](NOTICE).
