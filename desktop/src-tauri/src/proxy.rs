//! Системный прокси Windows (реестр WinINET) — режим «прокси».

use std::ffi::c_void;
use std::net::{TcpStream, ToSocketAddrs};
use std::time::Duration;

use winreg::enums::{HKEY_CURRENT_USER, KEY_QUERY_VALUE, KEY_SET_VALUE};
use winreg::RegKey;

const SUBKEY: &str = r"Software\Microsoft\Windows\CurrentVersion\Internet Settings";
const OPT_SETTINGS_CHANGED: u32 = 39;
const OPT_REFRESH: u32 = 37;

#[link(name = "wininet")]
extern "system" {
    fn InternetSetOptionW(
        h: *mut c_void,
        option: u32,
        buffer: *mut c_void,
        len: u32,
    ) -> i32;
}

fn notify() {
    unsafe {
        InternetSetOptionW(std::ptr::null_mut(), OPT_SETTINGS_CHANGED, std::ptr::null_mut(), 0);
        InternetSetOptionW(std::ptr::null_mut(), OPT_REFRESH, std::ptr::null_mut(), 0);
    }
}

/// Прежние настройки прокси — чтобы вернуть их при отключении.
#[derive(Default, Clone)]
pub struct ProxyState {
    had_enable: bool,
    enable: u32,
    had_server: bool,
    server: String,
    had_override: bool,
    override_: String,
}

fn key() -> Result<RegKey, String> {
    RegKey::predef(HKEY_CURRENT_USER)
        .open_subkey_with_flags(SUBKEY, KEY_QUERY_VALUE | KEY_SET_VALUE)
        .map_err(|e| e.to_string())
}

/// Включить системный HTTP-прокси на `addr` (например "127.0.0.1:10809").
pub fn set(addr: &str) -> Result<ProxyState, String> {
    let k = key()?;
    let mut st = ProxyState::default();
    if let Ok(v) = k.get_value::<u32, _>("ProxyEnable") {
        st.had_enable = true;
        st.enable = v;
    }
    if let Ok(v) = k.get_value::<String, _>("ProxyServer") {
        st.had_server = true;
        st.server = v;
    }
    if let Ok(v) = k.get_value::<String, _>("ProxyOverride") {
        st.had_override = true;
        st.override_ = v;
    }

    k.set_value("ProxyEnable", &1u32).map_err(|e| e.to_string())?;
    k.set_value("ProxyServer", &addr.to_string()).map_err(|e| e.to_string())?;
    k.set_value("ProxyOverride", &"<local>".to_string()).map_err(|e| e.to_string())?;
    notify();
    Ok(st)
}

/// Вернуть прежние настройки прокси.
pub fn restore(st: &ProxyState) -> Result<(), String> {
    let k = key()?;
    if st.had_enable {
        let _ = k.set_value("ProxyEnable", &st.enable);
    } else {
        let _ = k.delete_value("ProxyEnable");
    }
    if st.had_server {
        let _ = k.set_value("ProxyServer", &st.server);
    } else {
        let _ = k.delete_value("ProxyServer");
    }
    if st.had_override {
        let _ = k.set_value("ProxyOverride", &st.override_);
    } else {
        let _ = k.delete_value("ProxyOverride");
    }
    notify();
    Ok(())
}

/// Выключить системный прокси.
pub fn disable() -> Result<(), String> {
    let k = key()?;
    k.set_value("ProxyEnable", &0u32).map_err(|e| e.to_string())?;
    notify();
    Ok(())
}

/// Если прокси включён, указывает на `addr`, но порт уже не слушается — выключаем.
/// Иначе браузер останется без интернета, обращаясь к мёртвому прокси.
pub fn clear_stale(addr: &str) {
    let k = match key() {
        Ok(k) => k,
        Err(_) => return,
    };
    let enable: u32 = k.get_value("ProxyEnable").unwrap_or(0);
    if enable != 1 {
        return;
    }
    let server: String = k.get_value("ProxyServer").unwrap_or_default();
    let server = server.strip_prefix("http://").unwrap_or(&server).to_string();
    if server != addr {
        return;
    }
    if port_open(addr) {
        return;
    }
    let _ = disable();
}

fn port_open(addr: &str) -> bool {
    if let Ok(mut it) = addr.to_socket_addrs() {
        if let Some(sa) = it.next() {
            return TcpStream::connect_timeout(&sa, Duration::from_millis(400)).is_ok();
        }
    }
    false
}
