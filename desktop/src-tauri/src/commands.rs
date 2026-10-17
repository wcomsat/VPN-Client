//! Tauri-команды: статус, подписка, подключение/отключение.

use std::net::{SocketAddr, TcpStream, ToSocketAddrs};
use std::path::PathBuf;
use std::time::{Duration, Instant};

use tauri::path::BaseDirectory;
use tauri::{AppHandle, Manager, State};
use vpn_core::{build_core_config, parse_subscription, pick_channel};

use crate::coreproc::XrayProc;
use crate::state::{AppState, HTTP_PORT, SOCKS_PORT};
use crate::{proxy, tun};

#[derive(serde::Serialize)]
pub struct Status {
    pub running: bool,
    pub mode: String,
    pub channel: String,
    pub server: String,
    pub admin: bool,
}

#[tauri::command]
pub fn get_status(mgr: State<AppState>) -> Status {
    let i = mgr.inner.lock().unwrap();
    Status {
        running: i.running,
        mode: i.mode.clone(),
        channel: i.channel.clone(),
        server: i.server.clone(),
        admin: tun::is_admin(),
    }
}

/// Файл подписки — в каталоге конфигурации приложения.
fn subscription_path(app: &AppHandle) -> Result<PathBuf, String> {
    let dir = app
        .path()
        .app_config_dir()
        .map_err(|e| e.to_string())?;
    std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    Ok(dir.join("subscription.txt"))
}

/// Путь к ресурсу. В установленной сборке Tauri кладёт `resources/*` в
/// `<resource_dir>/resources/`, в dev — в `src-tauri/resources/`.
fn resource(app: &AppHandle, name: &str) -> Result<PathBuf, String> {
    for rel in [name.to_string(), format!("resources/{name}")] {
        if let Ok(p) = app.path().resolve(&rel, BaseDirectory::Resource) {
            if p.exists() {
                return Ok(p);
            }
        }
    }
    let dev = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("resources").join(name);
    if dev.exists() {
        return Ok(dev);
    }
    Err(format!("{name} не найден — положите его в resources/"))
}

#[tauri::command]
pub fn load_subscription(app: AppHandle) -> Result<String, String> {
    let p = subscription_path(&app)?;
    match std::fs::read_to_string(&p) {
        Ok(s) => Ok(s.trim().to_string()),
        Err(_) => Ok(String::new()),
    }
}

#[tauri::command]
pub fn save_subscription(app: AppHandle, text: String) -> Result<(), String> {
    let p = subscription_path(&app)?;
    std::fs::write(&p, format!("{}\n", text.trim())).map_err(|e| e.to_string())
}

/// Вернуть список доступных каналов («xhttp»/«vision»).
#[tauri::command]
pub fn parse_channels(text: String) -> Result<Vec<String>, String> {
    let chans = parse_subscription(&text)?;
    let mut names = Vec::new();
    for c in chans {
        let n = if c.transport == "tcp" { "vision".to_string() } else { c.transport.clone() };
        if !names.contains(&n) {
            names.push(n);
        }
    }
    Ok(names)
}

#[tauri::command]
pub fn connect(
    app: AppHandle,
    subscription: String,
    channel: String,
    mode: String,
) -> Result<(), String> {
    do_connect(&app, &subscription, &channel, &mode)
}

#[tauri::command]
pub fn disconnect(app: AppHandle) -> Result<(), String> {
    do_disconnect(&app)
}

/// Перезапустить приложение с правами администратора (для режима TUN).
#[tauri::command]
pub fn elevate(app: AppHandle) -> Result<(), String> {
    tun::elevate()?;
    app.exit(0);
    Ok(())
}

/// Подключение (общее для команды и трея).
pub fn do_connect(
    app: &AppHandle,
    subscription: &str,
    channel: &str,
    mode: &str,
) -> Result<(), String> {
    let mgr = app.state::<AppState>();
    {
        let i = mgr.inner.lock().unwrap();
        if i.running {
            return Err("уже подключено — сначала отключите".into());
        }
    }

    let channels = parse_subscription(subscription)?;
    let ch = pick_channel(&channels, channel)?;
    let cfg = build_core_config(&ch, SOCKS_PORT, HTTP_PORT);
    let xray = resource(app, "xray.exe")?;
    let mut proc = XrayProc::start(&xray, &cfg)?;

    if !wait_port(SOCKS_PORT, Duration::from_secs(10)) {
        proc.stop();
        return Err(format!(
            "ядро Xray не подняло SOCKS (см. {})",
            proc.log_path().display()
        ));
    }

    match mode {
        "proxy" => {
            let st = proxy::set(&format!("127.0.0.1:{HTTP_PORT}"))?;
            let mut i = mgr.inner.lock().unwrap();
            i.proxy = Some(st);
            i.xray = Some(proc);
            i.running = true;
            i.mode = "proxy".into();
            i.channel = ch.transport.clone();
            i.server = ch.address.clone();
            i.last = Some((subscription.to_string(), channel.to_string(), mode.to_string()));
            Ok(())
        }
        "tun" => {
            if !tun::is_admin() {
                proc.stop();
                return Err("режим TUN требует прав администратора".into());
            }
            let server_ip = resolve_ipv4(&ch.address)?;
            let tun2socks = resource(app, "tun2socks.exe")?;
            match tun::start(&tun2socks, &format!("127.0.0.1:{SOCKS_PORT}"), &server_ip) {
                Ok(sess) => {
                    let mut i = mgr.inner.lock().unwrap();
                    i.tun = Some(sess);
                    i.xray = Some(proc);
                    i.running = true;
                    i.mode = "tun".into();
                    i.channel = ch.transport.clone();
                    i.server = ch.address.clone();
                    i.last = Some((subscription.to_string(), channel.to_string(), mode.to_string()));
                    Ok(())
                }
                Err(e) => {
                    proc.stop();
                    Err(e)
                }
            }
        }
        other => {
            proc.stop();
            Err(format!("неизвестный режим {other:?}"))
        }
    }
}

/// Отключение: снять TUN/прокси и остановить ядро.
pub fn do_disconnect(app: &AppHandle) -> Result<(), String> {
    let mgr = app.state::<AppState>();
    let mut i = mgr.inner.lock().unwrap();
    if let Some(mut t) = i.tun.take() {
        t.stop();
    }
    if let Some(p) = i.proxy.take() {
        let _ = proxy::restore(&p);
    }
    if let Some(mut x) = i.xray.take() {
        x.stop();
    }
    i.running = false;
    i.mode.clear();
    i.channel.clear();
    i.server.clear();
    Ok(())
}

/// Подключиться из трея: подписка + канал xhttp + режим proxy, best-effort.
pub fn tray_connect(app: &AppHandle) -> Result<(), String> {
    let sub = load_subscription(app.clone())?;
    if sub.trim().is_empty() {
        return Err("нет сохранённой подписки".into());
    }
    do_connect(app, &sub, "xhttp", "proxy")
}

/// Сторож: если после сна/сбоя ядро или tun2socks умерли, а статус «подключено» —
/// перезапускаем туннель; если не выходит — снимаем настройки, чтобы был интернет.
pub fn watchdog(app: AppHandle) {
    std::thread::spawn(move || loop {
        std::thread::sleep(Duration::from_secs(5));

        let recover = {
            let st = app.state::<AppState>();
            let mut i = st.inner.lock().unwrap();
            if !i.running {
                continue;
            }
            let xray_dead = match i.xray.as_mut() {
                Some(p) => !p.alive(),
                None => true,
            };
            let tun_dead = i.mode == "tun"
                && match i.tun.as_mut() {
                    Some(t) => !t.alive(),
                    None => true,
                };
            if xray_dead || tun_dead {
                i.last.clone()
            } else {
                continue;
            }
        };

        let _ = do_disconnect(&app);
        if let Some((sub, ch, mode)) = recover {
            let _ = do_connect(&app, &sub, &ch, &mode);
        }
    });
}

pub fn wait_port(port: u16, timeout: Duration) -> bool {
    let addr = SocketAddr::from(([127, 0, 0, 1], port));
    let deadline = Instant::now() + timeout;
    while Instant::now() < deadline {
        if TcpStream::connect_timeout(&addr, Duration::from_millis(400)).is_ok() {
            return true;
        }
        std::thread::sleep(Duration::from_millis(200));
    }
    false
}

fn resolve_ipv4(host: &str) -> Result<String, String> {
    if let Ok(ip) = host.parse::<std::net::IpAddr>() {
        return match ip {
            std::net::IpAddr::V4(v4) => Ok(v4.to_string()),
            std::net::IpAddr::V6(_) => Err("нужен IPv4-адрес сервера".into()),
        };
    }
    let addrs = (host, 0u16)
        .to_socket_addrs()
        .map_err(|e| format!("не удалось разрешить {host}: {e}"))?;
    for a in addrs {
        if let SocketAddr::V4(v4) = a {
            return Ok(v4.ip().to_string());
        }
    }
    Err(format!("у {host} нет IPv4-адреса"))
}

