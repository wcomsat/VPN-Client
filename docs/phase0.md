# Фаза 0. Спайк: выбор ядра и зависимостей

Задача фазы — до написания кода зафиксировать, на чём строим клиент, чтобы не
уперёться в лицензии или отсутствие поддержки нужного транспорта.

## Итог (решения)

| Вопрос | Решение |
|---|---|
| Ядро туннеля | **Xray-core** (MPL-2.0) — тот же, что на сервере |
| Android-обёртка ядра | **XTLS/libXray** (MIT), AAR |
| Маршрутизация TUN→SOCKS | **xjasonlyu/tun2socks** (MIT) |
| TUN-драйвер Windows | **WinTun prebuilt DLL** (не исходники) |
| GPL-компоненты (sing-box, v2rayNG, Hiddify) | **не используем** |
| Основной канал клиента | **xHTTP + REALITY** (есть в Xray-core) |
| Резервный канал | **TCP + XTLS Vision** |

## Поддержка транспорта xHTTP

Клиент обязан уметь наш основной канал. Проверено:

- **Xray-core — да.** SplitHTTP появился в v1.8.16 (середина 2024),
  переработан и переименован в **XHTTP** в **v24.10.31**, режим
  **`stream-one`** — в **v24.11.30**. Сервер использует Xray-core **v26.3.27**,
  поэтому на клиенте фиксируем Xray-core **≥ v24.11.30** (фактически — ту же
  ветку v26.x).
- **sing-box — нет** нативной поддержки xHTTP (только HTTP/WS/QUIC/gRPC/
  HTTPUpgrade). Поэтому sing-box отпадает независимо от лицензии.

## Лицензии зависимостей

| Компонент | Лицензия | GPL? | Годится |
|---|---|---|---|
| XTLS/Xray-core | MPL-2.0 | нет | ✅ |
| XTLS/libXray | MIT | нет | ✅ |
| xjasonlyu/tun2socks | MIT | нет | ✅ |
| eycorsican/go-tun2socks | MIT | нет | ✅ (запасной) |
| SagerNet/sing-box | GPL-3.0-or-later | да | ❌ |
| 2dust/v2rayNG | GPL-3.0 | да | ❌ |
| Hiddify/hiddify-app | GPLv3 + NonCommercial | да (+NC) | ❌ |
| WinTun: исходники драйвера | GPL-2.0 | да | ⚠️ только prebuilt DLL |
| WinTun: prebuilt `wintun.dll` | proprietary-style binaries license | нет | ✅ (с атрибуцией) |

Вывод: набор **Xray-core (MPL) + libXray (MIT) + tun2socks (MIT) + Wintun DLL**
позволяет собрать клиент целиком под permissive-лицензиями, без GPL.

## Что осталось доказать на живом железе (PoC)

Спайк на этом этапе — кабинетный (лицензии + поддержка транспорта). Остаётся
проверить вживую:

1. **Android PoC:** libXray поднимает наш xHTTP+REALITY и Vision; `VpnService`
   отдаёт tun, tun2socks маршрутизирует в SOCKS-инбаунд Xray.
2. **Windows PoC:** Xray-core + tun2socks + Wintun поднимают оба канала под
   администратором, DNS/маршруты применяются.
3. Зафиксировать версию Xray-core (v26.x) и способ её обновления.

Эти пункты — первые задачи Фазы 1/2/3 (см. [build.md](build.md)).

## Влияние на структуру ссылок (без изменений на сервере)

Клиент потребляет уже существующий `subscription.txt`:

```
vless://UUID@IP:443?encryption=none&security=reality&sni=SNI&fp=firefox&pbk=PUB&sid=SID&type=xhttp&path=...&mode=stream-one#name-xhttp
vless://UUID@IP:8443?encryption=none&security=reality&sni=SNI&fp=firefox&pbk=PUB&sid=SID&type=tcp&flow=xtls-rprx-vision#name-vision
```

Сервер менять не нужно. Общий контракт парсинга — в каталоге
[`shared/`](../shared/README.md), фикстуры используются тестами и Android, и
Windows.
