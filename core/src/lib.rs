//! Ядро VPN-клиента: разбор подписки `vless://` и сборка конфига Xray-core.
//!
//! Общий крейт для всех платформ (Windows/Android). Контракт и тест-данные —
//! в `shared/` (см. `shared/README.md`). Результат сборки сверяется с
//! `shared/fixtures/expected/*.json`.

use serde::{Deserialize, Serialize};

/// Публичные параметры REALITY (без приватных ключей).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Reality {
    pub server_name: String,
    pub public_key: String,
    pub short_id: String,
    pub fingerprint: String,
    #[serde(default = "default_spider")]
    pub spider_x: String,
}

/// Параметры транспорта xHTTP (только для transport = "xhttp").
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Xhttp {
    pub host: String,
    pub path: String,
    pub mode: String,
}

/// Нормализованное представление одного канала.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Channel {
    pub name: String,
    pub protocol: String,
    pub uuid: String,
    pub address: String,
    pub port: u16,
    /// "xhttp" | "tcp"
    pub transport: String,
    pub security: String,
    pub reality: Reality,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub xhttp: Option<Xhttp>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub flow: Option<String>,
}

fn default_spider() -> String {
    "/".to_string()
}

/// Ошибка разбора подписки.
pub type Error = String;
pub type Result<T> = std::result::Result<T, Error>;

fn param(pairs: &[(String, String)], key: &str) -> String {
    pairs
        .iter()
        .find(|(k, _)| k == key)
        .map(|(_, v)| v.clone())
        .unwrap_or_default()
}

/// Разобрать одну ссылку `vless://`.
pub fn parse_link(raw: &str) -> Result<Channel> {
    let raw = raw.trim();
    if raw.is_empty() {
        return Err("пустая ссылка".into());
    }
    let rest = raw
        .strip_prefix("vless://")
        .ok_or_else(|| format!("ожидалась схема vless://, получено {raw:?}"))?;

    // fragment (#name)
    let (rest, name) = match rest.split_once('#') {
        Some((a, b)) => (a, b.to_string()),
        None => (rest, String::new()),
    };
    // query (?params)
    let (authority, query) = match rest.split_once('?') {
        Some((a, b)) => (a, b.to_string()),
        None => (rest, String::new()),
    };
    // userinfo@host:port
    let (userinfo, hostport) = authority
        .rsplit_once('@')
        .ok_or_else(|| "в ссылке нет UUID (userinfo@host)".to_string())?;
    if userinfo.is_empty() {
        return Err("в ссылке нет UUID".into());
    }
    let (host, port_str) = split_hostport(hostport)?;
    let port: u16 = port_str
        .parse()
        .map_err(|_| format!("некорректный порт {port_str:?}"))?;

    let pairs: Vec<(String, String)> = query
        .split('&')
        .filter(|s| !s.is_empty())
        .map(|kv| match kv.split_once('=') {
            Some((k, v)) => (k.to_string(), v.to_string()),
            None => (kv.to_string(), String::new()),
        })
        .collect();

    let mut transport = param(&pairs, "type");
    if transport.is_empty() {
        transport = "tcp".into();
    }
    if transport != "xhttp" && transport != "tcp" {
        return Err(format!("неподдерживаемый транспорт: {transport:?}"));
    }

    let sni = param(&pairs, "sni");
    let fp = {
        let v = param(&pairs, "fp");
        if v.is_empty() {
            "firefox".into()
        } else {
            v
        }
    };
    let spider_x = {
        let v = param(&pairs, "spx");
        if v.is_empty() {
            "/".into()
        } else {
            v
        }
    };
    let mut security = param(&pairs, "security");
    if security.is_empty() {
        security = "reality".into();
    }

    let reality = Reality {
        server_name: sni.clone(),
        public_key: param(&pairs, "pbk"),
        short_id: param(&pairs, "sid"),
        fingerprint: fp,
        spider_x,
    };

    let (xhttp, flow) = if transport == "xhttp" {
        let h = {
            let v = param(&pairs, "host");
            if v.is_empty() {
                sni.clone()
            } else {
                v
            }
        };
        let p = {
            let v = param(&pairs, "path");
            if v.is_empty() {
                "/".into()
            } else {
                v
            }
        };
        let m = {
            let v = param(&pairs, "mode");
            if v.is_empty() {
                "stream-one".into()
            } else {
                v
            }
        };
        (Some(Xhttp { host: h, path: p, mode: m }), None)
    } else {
        let f = param(&pairs, "flow");
        (None, if f.is_empty() { None } else { Some(f) })
    };

    Ok(Channel {
        name,
        protocol: "vless".into(),
        uuid: userinfo.to_string(),
        address: host,
        port,
        transport,
        security,
        reality,
        xhttp,
        flow,
    })
}

fn split_hostport(hostport: &str) -> Result<(String, String)> {
    // IPv6 в скобках: [::1]:443
    if let Some(rest) = hostport.strip_prefix('[') {
        let (host, after) = rest
            .split_once(']')
            .ok_or_else(|| format!("некорректный адрес {hostport:?}"))?;
        let port = after
            .strip_prefix(':')
            .ok_or_else(|| format!("нет порта в {hostport:?}"))?;
        return Ok((host.to_string(), port.to_string()));
    }
    hostport
        .rsplit_once(':')
        .map(|(h, p)| (h.to_string(), p.to_string()))
        .ok_or_else(|| format!("нет host:port в {hostport:?}"))
}

/// Разобрать текст подписки (несколько строк).
pub fn parse_subscription(text: &str) -> Result<Vec<Channel>> {
    let mut out = Vec::new();
    for line in text.lines() {
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        out.push(parse_link(line)?);
    }
    if out.is_empty() {
        return Err("в подписке нет ни одной ссылки".into());
    }
    Ok(out)
}

/// Выбрать канал: «vision» — это транспорт `tcp`; иначе первый доступный.
pub fn pick_channel(channels: &[Channel], want: &str) -> Result<Channel> {
    let target = match want {
        "" | "xhttp" => "xhttp",
        "vision" => "tcp",
        other => other,
    };
    if let Some(c) = channels.iter().find(|c| c.transport == target) {
        return Ok(c.clone());
    }
    channels
        .first()
        .cloned()
        .ok_or_else(|| "в подписке нет ни одного канала".to_string())
}

/// Собрать конфиг Xray-core (SOCKS + HTTP инбаунды + VLESS-аутбаунд).
/// Совпадает с `shared/fixtures/expected/*.json`.
pub fn build_core_config(ch: &Channel, socks_port: u16, http_port: u16) -> serde_json::Value {
    use serde_json::json;

    let mut user = json!({ "id": ch.uuid.clone(), "encryption": "none" });
    if let Some(flow) = &ch.flow {
        if !flow.is_empty() {
            user["flow"] = json!(flow.clone());
        }
    }

    let reality = json!({
        "serverName": ch.reality.server_name.clone(),
        "fingerprint": ch.reality.fingerprint.clone(),
        "publicKey": ch.reality.public_key.clone(),
        "shortId": ch.reality.short_id.clone(),
        "spiderX": ch.reality.spider_x.clone(),
    });

    let mut stream = json!({
        "network": ch.transport.clone(),
        "security": "reality",
        "realitySettings": reality,
    });
    if ch.transport == "xhttp" {
        if let Some(x) = &ch.xhttp {
            stream["xhttpSettings"] = json!({
                "host": x.host.clone(),
                "path": x.path.clone(),
                "mode": x.mode.clone(),
            });
        }
    }

    let sniffing = json!({ "enabled": true, "destOverride": ["http", "tls", "quic"] });
    let socks = json!({
        "listen": "127.0.0.1",
        "port": socks_port,
        "protocol": "socks",
        "settings": { "udp": true },
        "sniffing": sniffing,
    });
    let http = json!({
        "listen": "127.0.0.1",
        "port": http_port,
        "protocol": "http",
        "sniffing": sniffing,
    });

    json!({
        "log": { "loglevel": "warning" },
        "inbounds": [socks, http],
        "outbounds": [
            {
                "protocol": "vless",
                "settings": {
                    "vnext": [
                        { "address": ch.address.clone(), "port": ch.port, "users": [user] }
                    ]
                },
                "streamSettings": stream,
            },
            { "protocol": "freedom", "tag": "direct" }
        ]
    })
}
