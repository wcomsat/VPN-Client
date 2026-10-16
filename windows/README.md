# VPN-Client для Windows

Минимальный клиент: подключение в один клик, выбор канала (xHTTP / Vision).

## Режим работы

**Системный прокси.** Приложение поднимает локальные инбаунды Xray (SOCKS
`127.0.0.1:10808` и HTTP `127.0.0.1:10809`) и включает **системный HTTP-прокси**
Windows на `127.0.0.1:10809`. Покрываются браузеры и приложения, уважающие
системный прокси; права администратора и драйвер не нужны.

Не покрывается: приложения, игнорирующие системный прокси, и UDP/QUIC.
Полноценный TUN (весь трафик) — следующий шаг.

## Стек
- **Go** — приложение.
- **Xray-core** (`xray.exe`) — ядро, поставляется рядом с приложением.
- `golang.org/x/sys` — доступ к реестру Windows (permissive).

## Состояние

Готово:
- разбор `vless://` и сборка конфига Xray-core (тесты против `shared/fixtures`);
- запуск встроенного `xray.exe`;
- системный прокси Windows (вкл. при `connect`, восстановление при выходе);
- загрузка подписки из файла/URL.

Дальше: трей-иконка; режим TUN (Wintun + tun2socks) для полного покрытия.

## Сборка и запуск

```powershell
cd windows
go test ./...
go build -trimpath -ldflags "-s -w" -o bin/vpn-client.exe .
```

Рядом с `vpn-client.exe` положите `xray.exe` (Xray-core v26.x). Затем:

```powershell
# подписка из файла или URL (https://.../subscription.txt)
.\bin\vpn-client.exe parse   --sub ..\subscription.txt
.\bin\vpn-client.exe connect --sub ..\subscription.txt --channel xhttp

# выключить системный прокси, если он остался включённым
.\bin\vpn-client.exe proxy off
```

## Структура

```
main.go                        CLI (version/parse/config/connect/proxy)
internal/config/config.go      парсер vless:// + сборка конфига ядра
internal/core/core.go          запуск встроенного xray.exe
internal/proxy/proxy_windows.go системный прокси Windows (реестр + WinINet)
```

> `connect` меняет настройки прокси в реестре (`HKCU\...\Internet Settings`) и
> восстанавливает их при Ctrl+C. Если приложение было закрыто аварийно —
> выполните `vpn-client proxy off`.
