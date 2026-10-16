# VPN

Клиент для личного VPN-сервиса ([VPN-Server](../VPN-Server)). Один клик —
и весь трафик идёт через ваш сервер. Современный интерфейс, простое приложение,
установщик и авто-обновления.

> Статус: активная разработка (Tauri/Rust). См. план ниже.

## Стек

- **Rust** + **Tauri v2** — приложение и нативное окно (WebView2).
- Общее ядро [`core/`](core) — разбор подписки `vless://` и сборка конфига
  Xray-core (используется и на Android).
- Внешнее ядро туннеля: **Xray-core** (`xray.exe`, MPL-2.0) + **tun2socks**
  (`tun2socks.exe`, MIT) + **WinTun** (`wintun.dll`).
- Фронтенд — обычные HTML/CSS/JS (без сборщиков), в каталоге [`desktop/ui`](desktop/ui).

Используются только permissive-лицензии (см. [NOTICE](NOTICE)).

## Состав репозитория

```
core/     Rust-крейт: парсер vless:// + генератор конфига Xray (общий для платформ)
desktop/  Windows-приложение (Tauri v2): ui/ (фронтенд) + src-tauri/ (Rust)
shared/   Контракт подписки и тест-фикстуры (vless -> конфиг ядра)
```

## Режимы подключения

- **Прокси** — системный HTTP-прокси (браузеры и приложения; без прав админа).
- **TUN** — весь трафик через WinTun + tun2socks (нужны права администратора).

## Разработка

```bash
cargo test -p vpn-core                    # парсер/конфиг против shared/fixtures
cargo run -p vpn-desktop -- selftest      # проверка бэкенда без окна

# ядро туннеля в resources/ (Xray + tun2socks + wintun)
pwsh tools/fetch-core.ps1

cd desktop
npx @tauri-apps/cli dev                   # запуск приложения
npx @tauri-apps/cli build                 # установщик NSIS
```

Готовый установщик: `target/release/bundle/nsis/VPN_<версия>_x64-setup.exe`
(ярлык на рабочем столе и в «Пуск», права администратора не нужны).

## Релиз и авто-обновление

Обновления раздаются через GitHub Releases. Клиент при запуске проверяет
`latest.json` и при наличии обновления ставит его сам.

Релиз публикуется workflow [`.github/workflows/release.yml`](.github/workflows/release.yml)
по тегу `v*` (tauri-action). Для подписи обновлений в настройках репозитория
(Secrets) задайте:

- `TAURI_SIGNING_PRIVATE_KEY` — содержимое приватного ключа (создан командой
  `npx @tauri-apps/cli signer generate`);
- `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` — пароль ключа.

Публичный ключ вписан в `desktop/src-tauri/tauri.conf.json` (`plugins.updater.pubkey`).

## Установка (для пользователя)

Скачать `VPN_<версия>_x64-setup.exe` из последнего релиза и запустить. Установка
идёт в профиль пользователя; создаются ярлык на рабочем столе и в «Пуск».

## План

1. **M0** — каркас репозитория и приложения. ✅
2. **M1** — общее ядро: парсер и конфиг (тесты на фикстурах). ✅
3. **M2** — логика Windows: ядро, прокси, TUN, состояние.
4. **M3** — современный интерфейс.
5. **M4** — установщик, иконка, ярлык.
6. **M5** — авто-обновление и CI.
7. **M6** — Android (Kotlin `VpnService` + общее Rust-ядро).

## Лицензия

Собственный код — [MIT](LICENSE). Сторонние компоненты — [NOTICE](NOTICE).
