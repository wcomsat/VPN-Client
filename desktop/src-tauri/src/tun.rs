//! TUN-режим: Wintun + tun2socks + маршруты/DNS. Требует прав администратора.

use std::ffi::c_void;
use std::os::windows::ffi::OsStrExt;
use std::os::windows::process::CommandExt;
use std::path::Path;
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

const CREATE_NO_WINDOW: u32 = 0x0800_0000;
const SW_SHOWNORMAL: i32 = 1;

const TUN_NAME: &str = "wintun";

#[link(name = "shell32")]
extern "system" {
    fn IsUserAnAdmin() -> i32;
    fn ShellExecuteW(
        hwnd: *mut c_void,
        operation: *const u16,
        file: *const u16,
        parameters: *const u16,
        directory: *const u16,
        show: i32,
    ) -> *mut c_void;
}

fn wide(s: &str) -> Vec<u16> {
    std::ffi::OsStr::new(s).encode_wide().chain(std::iter::once(0)).collect()
}

/// Запущено ли с правами администратора.
pub fn is_admin() -> bool {
    unsafe { IsUserAnAdmin() != 0 }
}

/// Перезапустить приложение с повышением прав (UAC).
pub fn elevate() -> Result<(), String> {
    let exe = std::env::current_exe().map_err(|e| e.to_string())?;
    let args: Vec<String> = std::env::args().skip(1).collect();
    let op = wide("runas");
    let file = wide(&exe.to_string_lossy());
    let params = wide(&args.join(" "));
    let ret = unsafe {
        ShellExecuteW(
            std::ptr::null_mut(),
            op.as_ptr(),
            file.as_ptr(),
            params.as_ptr(),
            std::ptr::null(),
            SW_SHOWNORMAL,
        )
    };
    if ret as isize <= 32 {
        return Err("запрос повышения прав отклонён".into());
    }
    Ok(())
}

/// Активная сессия TUN.
pub struct TunSession {
    child: Child,
    server_ip: String,
    alias: String,
    gateway: String,
}

/// Поднять TUN: запускает tun2socks и настраивает маршруты.
pub fn start(tun2socks: &Path, socks: &str, server_ip: &str) -> Result<TunSession, String> {
    let (alias, gateway) = primary_interface()?;

    let child = Command::new(tun2socks)
        .args([
            "--device", "wintun",
            "--proxy", &format!("socks5://{socks}"),
            "--interface", &alias,
            "--loglevel", "warn",
        ])
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .creation_flags(CREATE_NO_WINDOW)
        .spawn()
        .map_err(|e| format!("не удалось запустить tun2socks: {e}"))?;

    let mut session = TunSession {
        child,
        server_ip: server_ip.to_string(),
        alias,
        gateway,
    };

    if !wait_adapter(TUN_NAME, Duration::from_secs(15)) {
        session.stop();
        return Err("интерфейс wintun не появился (нужен wintun.dll рядом с tun2socks.exe)".into());
    }

    let steps: [&[&str]; 3] = [
        &["interface", "ipv4", "set", "address", "name=wintun", "source=static", "addr=192.168.123.1", "mask=255.255.255.0"],
        &["interface", "ipv4", "set", "dnsservers", "name=wintun", "static", "address=1.1.1.1", "register=none", "validate=no"],
        &["interface", "ipv4", "add", "route", "0.0.0.0/0", "wintun", "192.168.123.1", "metric=1"],
    ];
    for s in steps {
        if let Err(e) = netsh(s) {
            session.stop();
            return Err(format!("netsh {}: {e}", s.join(" ")));
        }
    }
    // host-маршрут к серверу — МИМО туннеля (иначе петля)
    let host = format!("{server_ip}/32");
    if let Err(e) = netsh(&["interface", "ipv4", "add", "route", &host, &session.alias.clone(), &session.gateway.clone(), "metric=1"]) {
        session.stop();
        return Err(format!("netsh host-route: {e}"));
    }

    Ok(session)
}

impl TunSession {
    pub fn stop(&mut self) {
        let _ = netsh(&["interface", "ipv4", "delete", "route", "0.0.0.0/0", "wintun", "192.168.123.1"]);
        let host = format!("{}/32", self.server_ip);
        let _ = netsh(&["interface", "ipv4", "delete", "route", &host, &self.alias, &self.gateway]);
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

impl Drop for TunSession {
    fn drop(&mut self) {
        self.stop();
    }
}

fn netsh(args: &[&str]) -> Result<String, String> {
    let out = Command::new("netsh")
        .args(args)
        .creation_flags(CREATE_NO_WINDOW)
        .output()
        .map_err(|e| e.to_string())?;
    let text = String::from_utf8_lossy(&out.stdout).to_string()
        + &String::from_utf8_lossy(&out.stderr);
    if out.status.success() {
        Ok(text)
    } else {
        Err(text.trim().to_string())
    }
}

fn wait_adapter(name: &str, timeout: Duration) -> bool {
    let deadline = Instant::now() + timeout;
    while Instant::now() < deadline {
        if let Ok(out) = netsh(&["interface", "ipv4", "show", "interfaces"]) {
            if out.contains(name) {
                return true;
            }
        }
        std::thread::sleep(Duration::from_millis(400));
    }
    false
}

/// Имя активного интерфейса и его шлюз по умолчанию.
fn primary_interface() -> Result<(String, String), String> {
    let ps = "$c = Get-NetIPConfiguration | Where-Object { $_.IPv4DefaultGateway -ne $null } | Select-Object -First 1; if ($c) { $c.InterfaceAlias; $c.IPv4DefaultGateway.NextHop }";
    let out = Command::new("powershell")
        .args(["-NoProfile", "-NonInteractive", "-Command", ps])
        .creation_flags(CREATE_NO_WINDOW)
        .output()
        .map_err(|e| format!("не удалось определить основной интерфейс: {e}"))?;
    let text = String::from_utf8_lossy(&out.stdout);
    let mut lines = text.lines().map(|l| l.trim()).filter(|l| !l.is_empty());
    let alias = lines.next().unwrap_or("").to_string();
    let gateway = lines.next().unwrap_or("").to_string();
    if alias.is_empty() || gateway.is_empty() {
        return Err("не найден основной интерфейс/шлюз".into());
    }
    Ok((alias, gateway))
}
