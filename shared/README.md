# shared — общий контракт

Здесь описано, как обе платформы (Android и Windows) интерпретируют подписку.
Логика одна, реализации две, поэтому результат проверяется **одними и теми же
фикстурами**.

## Что такое подписка

Файл `subscription.txt` (выдаёт сервер VPN-Server в `clients/<имя>/`) — две
строки-ссылки:

```
vless://UUID@IP:443  ?encryption=none&security=reality&sni=SNI&fp=firefox&pbk=PUBKEY&sid=SHORTID&type=xhttp&path=/api/v1/updates&mode=stream-one#name-xhttp
vless://UUID@IP:8443 ?encryption=none&security=reality&sni=SNI&fp=firefox&pbk=PUBKEY&sid=SHORTID&type=tcp&flow=xtls-rprx-vision#name-vision
```

- **xhttp** — основной канал (Android/Windows/desktop); `mode=stream-one`.
- **tcp + flow=xtls-rprx-vision** — резервный канал; обязателен для iOS, полезен
  всем при деградации основного.

## Модель канала (после парсинга)

См. [`schema/channel.schema.json`](schema/channel.schema.json). Ключевые поля:
`uuid`, `address`, `port`, `transport` (`xhttp`|`tcp`), `reality{serverName,
publicKey,shortId,fingerprint,spiderX}`, `xhttp{host,path,mode}` (для xhttp),
`flow` (для tcp).

## Как из модели получается конфиг ядра

См. [`fixtures/expected/`](fixtures/expected/): для каждого канала — готовый
Xray-core client-config (SOCKS-инбаунд + VLESS-аутбаунд). Именно его отдаём ядру.

## Фикстуры

- [`subscription.example.txt`](subscription.example.txt) — пример подписки.
- [`fixtures/sample-links.txt`](fixtures/sample-links.txt) — входные ссылки.
- [`fixtures/expected/xhttp.json`](fixtures/expected/xhttp.json),
  [`fixtures/expected/vision.json`](fixtures/expected/vision.json) — ожидаемый
  результат генерации конфига.

> Значения в примерах — **заглушки** (UUID и ключи вымышленные). Настоящая
> подписка — секрет и в репозиторий не попадает.
