# VPN-Client для Windows

Минимальный клиент: подключение одной командой, выбор канала (xHTTP / Vision).

## Как этим пользоваться (для человека, не для программиста)

Проще всего — поставить ярлык и запускать двойным кликом:

```powershell
# из папки с распакованным пакетом
pwsh -ExecutionPolicy Bypass -File .\install-windows.ps1
```

Установщик копирует файлы в `%LOCALAPPDATA%\VPN-Client` и создаёт ярлык **«VPN»**
на рабочем столе и в меню «Пуск». Двойной клик по ярлыку открывает **окно
приложения** (Edge/Chrome в режиме `--app`: без адресной строки, вкладок и меню —
выглядит как нативное приложение, внутри — современный интерфейс). Значок
клиента при этом виден и в трее.

Если Edge/Chrome не найден — откроется во браузере по умолчанию.

Для удаления: `pwsh -File .\install-windows.ps1 -Uninstall`.

> Режим TUN требует прав администратора — запускайте ярлык «VPN» через
> «Запуск от имени администратора».

## Варианты интерфейса

| Команда | Что открывает |
|---|---|
| без аргументов (`vpn-client.exe`) | окно-приложение + трей (по умолчанию) |
| `vpn-client.exe ui` | страницу в браузере (`http://127.0.0.1:8787/`) |
| `vpn-client.exe window` | простое нативное окно (walk) |
| `vpn-client.exe tray` | только значок в трее |

## Режимы работы

- **`--mode proxy`** (по умолчанию) — системный HTTP-прокси на локальный инбаунд
  Xray. Покрывает браузеры и приложения, уважающие системный прокси. **Без прав
  администратора и без драйвера.** Не покрывает UDP/QUIC и игры.
- **`--mode tun`** — полноценный туннель: Wintun + `tun2socks` направляет **весь**
  системный трафик (включая UDP/QUIC, игры) в локальный SOCKS Xray. **Требует
  прав администратора.** Критичный момент: IP VPN-сервера исключается из
  туннеля host-маршрутом (иначе петля), и трафик идёт через драйвер Wintun.

## Стек
- **Go** — приложение.
- **Xray-core** (`xray.exe`) — ядро.
- **tun2socks** (`tun2socks.exe`, MIT) + **wintun.dll** — для режима TUN.
- `golang.org/x/sys` — реестр/токен Windows (permissive).

## Состояние

Готово:
- разбор `vless://` и сборка конфига Xray-core (тесты против `shared/fixtures`);
- запуск встроенного `xray.exe`; загрузка подписки из файла/URL;
- режим `proxy` (системный прокси, восстановление настроек при выходе);
- режим `tun` (Wintun + tun2socks, маршруты/DNS, host-маршрут к серверу,
  самоповышение прав через UAC).

Дальше: трей-иконка; авто-обновление клиента.

## Сборка и запуск

```powershell
cd windows
go test ./...
go build -trimpath -ldflags "-s -w" -o bin/vpn-client.exe .
```

Рядом с `vpn-client.exe` — `xray.exe` (Xray-core v26.x); для TUN ещё
`tun2socks.exe` и `wintun.dll`.

```powershell
# подписка из файла или URL (https://.../subscription.txt)
.\bin\vpn-client.exe parse   --sub ..\subscription.txt

# системный прокси (без админа)
.\bin\vpn-client.exe connect --sub ..\subscription.txt --channel xhttp --mode proxy

# полный туннель (запросит UAC)
.\bin\vpn-client.exe connect --sub ..\subscription.txt --channel xhttp --mode tun

# снять системный прокси, если остался включённым
.\bin\vpn-client.exe proxy off
```

## Структура

```
main.go                        CLI (version/parse/config/connect/proxy)
internal/config/config.go      парсер vless:// + сборка конфига ядра
internal/core/core.go          запуск встроенного xray.exe; поиск файлов рядом
internal/proxy/proxy_windows.go системный прокси Windows (реестр + WinINet)
internal/tun/tun_windows.go    TUN: Wintun + tun2socks, маршруты/DNS, UAC
```

> **TUN и права.** Режим `tun` меняет маршруты и создаёт сетевой адаптер —
> нужны права администратора (клиент сам запросит UAC). При аварийном
> завершении маршруты могут остаться — удалить их можно повторным запуском или
> сбросом адаптера Wintun (`netsh interface set interface "wintun" disable`).
> Режим TUN требует проверки на живом ПК — драйверы и маршруты в песочнице не
> тестируются.
