# VPN-Client

Свой минималистичный клиент под личный VPN-сервис ([VPN-Server](../VPN-Server)).

Цель: пользователь (вы и друзья) один раз ставит приложение и подключается в
один клик, без третьих клиентов и без ручной правки конфигов. Распространяется
напрямую — через **GitHub Releases** (магазины не используются).

Платформы: **Android** и **Windows**.

> Статус: каркас проекта (Фаза 0). Рабочий функционал туннеля появится в
> следующих фазах — см. [docs/phase0.md](docs/phase0.md).

---

## Архитектура

- **Ядро туннеля — Xray-core** (MPL-2.0), тот же, что на сервере. Поддерживает
  наш основной канал **VLESS + REALITY + xHTTP** и резервный **TCP + Vision**.
- **Android:** Kotlin + [`libXray`](https://github.com/XTLS/libXray) (MIT, AAR) +
  `tun2socks`; системный VPN через `VpnService`.
- **Windows:** Go + Xray-core (как библиотека/процесс в комплекте) + `tun2socks`
  (MIT) + WinTun; трей с кнопкой подключения.
- **Подписка:** приложение забирает персональный `subscription.txt`
  (две ссылки `vless://`, генерируются сервером) и превращает их в конфиг ядра.

Все зависимости — **permissive (MPL/MIT)**, без GPL. Детали: [NOTICE](NOTICE),
[docs/phase0.md](docs/phase0.md).

## Состав репозитория

```
shared/    общий контракт: схема клиентского конфига и фикстуры vless→JSON
windows/   клиент для Windows (Go)
android/   клиент для Android (Kotlin/Gradle)
docs/      архитектура, сборка, распространение, отчёт Фазы 0
tools/     скрипты упаковки
```

## Сборка и распространение

- [docs/build.md](docs/build.md) — как собрать exe и APK.
- [docs/distribute.md](docs/distribute.md) — как раздать друзьям (GitHub Releases,
  sideload, SHA256).

## Лицензия

Собственный код — [MIT](LICENSE). Сторонние компоненты — см. [NOTICE](NOTICE).
