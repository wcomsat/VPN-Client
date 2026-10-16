# VPN-Client для Windows

Минималистичный клиент: connect + выбор канала (xHTTP / Vision).

## Стек
- **Go** — приложение и управление ядром.
- **Xray-core** (`xray.exe`) — ядро туннеля, поставляется рядом с приложением.
- **tun2socks** (MIT) + **WinTun** (prebuilt DLL) — системная маршрутизация.
  *(интеграция — следующий шаг; сейчас connect поднимает только SOCKS-инбаунд.)*

## Текущее состояние каркаса

Реализовано и покрыто тестами:
- `internal/config` — разбор `vless://` и сборка конфига Xray-core;
  совпадение с `shared/fixtures/expected/*.json`.

Ещё не сделано (следующие шаги):
- `tun2socks` + WinTun, маршруты и DNS, права администратора;
- трей (connect/disconnect, выбор канала);
- упаковка portable exe.

## Сборка и тесты

```powershell
cd windows
go test ./...                 # парсер против общих фикстур
go build -o bin/vpn-client.exe .
```

Для подключения положите рядом `xray.exe` (из релиза Xray-core v26.x):

```powershell
.\bin\vpn-client.exe connect --sub ..\subscription.txt --channel xhttp
```

## Структура

```
main.go                      CLI (version/parse/config/connect)
internal/config/config.go    парсер vless:// + сборка конфига ядра
internal/config/config_test.go
internal/core/core.go        запуск встроенного xray.exe
```
