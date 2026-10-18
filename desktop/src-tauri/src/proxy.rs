//! Системный прокси: Windows (реестр WinINET) и macOS (`networksetup`).

#[cfg(windows)]
mod imp {
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
        fn InternetSetOptionW(h: *mut c_void, option: u32, buffer: *mut c_void, len: u32) -> i32;
    }

    fn notify() {
        unsafe {
            InternetSetOptionW(
                std::ptr::null_mut(),
                OPT_SETTINGS_CHANGED,
                std::ptr::null_mut(),
                0,
            );
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

        k.set_value("ProxyEnable", &1u32)
            .map_err(|e| e.to_string())?;
        k.set_value("ProxyServer", &addr.to_string())
            .map_err(|e| e.to_string())?;
        k.set_value("ProxyOverride", &"<local>".to_string())
            .map_err(|e| e.to_string())?;
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

    /// Выключить прокси, только если он указывает на наш адрес (аварийная очистка).
    pub fn disable_if_ours(addr: &str) {
        let k = match key() {
            Ok(k) => k,
            Err(_) => return,
        };
        let server: String = k.get_value("ProxyServer").unwrap_or_default();
        let server = server
            .strip_prefix("http://")
            .unwrap_or(&server)
            .to_string();
        if server == addr {
            let _ = disable();
        }
    }

    /// Выключить системный прокси.
    pub fn disable() -> Result<(), String> {
        let k = key()?;
        k.set_value("ProxyEnable", &0u32)
            .map_err(|e| e.to_string())?;
        notify();
        Ok(())
    }

    /// Если прокси включён, указывает на `addr`, но порт уже не слушается — выключаем.
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
        let server = server
            .strip_prefix("http://")
            .unwrap_or(&server)
            .to_string();
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
}

#[cfg(target_os = "macos")]
mod imp {
    use std::net::{TcpStream, ToSocketAddrs};
    use std::time::Duration;

    use crate::macos::{default_route, privileged, run, service_for, sh_quote};

    const NS: &str = "/usr/sbin/networksetup";

    #[derive(Default, Clone)]
    struct Endpoint {
        enabled: bool,
        server: String,
        port: u16,
    }

    /// Прежние настройки прокси активного сетевого сервиса.
    #[derive(Default, Clone)]
    pub struct ProxyState {
        service: String,
        web: Endpoint,
        secure: Endpoint,
        had_bypass: bool,
        bypass: Vec<String>,
    }

    fn parse_proxy(text: &str) -> Endpoint {
        let mut e = Endpoint::default();
        for line in text.lines() {
            let line = line.trim();
            if let Some(v) = line.strip_prefix("Enabled:") {
                e.enabled = v.trim().eq_ignore_ascii_case("yes") || v.trim() == "1";
            } else if let Some(v) = line.strip_prefix("Server:") {
                e.server = v.trim().to_string();
            } else if let Some(v) = line.strip_prefix("Port:") {
                e.port = v.trim().parse().unwrap_or(0);
            }
        }
        e
    }

    fn split_addr(addr: &str) -> Result<(&str, u16), String> {
        let (host, port) = addr
            .rsplit_once(':')
            .ok_or_else(|| format!("некорректный адрес прокси {addr:?}"))?;
        let port = port
            .parse()
            .map_err(|_| format!("некорректный порт {port:?}"))?;
        Ok((host, port))
    }

    fn active_service() -> Result<String, String> {
        let (iface, _) = default_route()?;
        service_for(&iface)
    }

    /// Включить системный HTTP/HTTPS-прокси на `addr` (например "127.0.0.1:10809").
    pub fn set(addr: &str) -> Result<ProxyState, String> {
        let service = active_service()?;
        let (host, port) = split_addr(addr)?;
        let web = parse_proxy(&run(NS, &["-getwebproxy", &service])?);
        let secure = parse_proxy(&run(NS, &["-getsecurewebproxy", &service])?);
        let bypass_text = run(NS, &["-getproxybypassdomains", &service]).unwrap_or_default();
        let had_bypass = !bypass_text.contains("There aren't any");
        let bypass = if had_bypass {
            bypass_text
                .lines()
                .map(str::trim)
                .filter(|s| !s.is_empty())
                .map(String::from)
                .collect()
        } else {
            Vec::new()
        };

        let ns = NS;
        let svc = sh_quote(&service);
        let script = format!(
            "{ns} -setwebproxy {svc} {h} {p}\n\
             {ns} -setsecurewebproxy {svc} {h} {p}\n\
             {ns} -setproxybypassdomains {svc} '*.local' '169.254/16' '127.0.0.1'\n",
            h = sh_quote(host),
            p = port,
        );
        privileged(&script)?;
        Ok(ProxyState {
            service,
            web,
            secure,
            had_bypass,
            bypass,
        })
    }

    /// Вернуть прежние настройки прокси.
    pub fn restore(st: &ProxyState) -> Result<(), String> {
        let ns = NS;
        let svc = sh_quote(&st.service);
        let mut script = String::new();

        if st.web.enabled {
            script.push_str(&format!(
                "{ns} -setwebproxy {svc} {server} {port}\n",
                server = sh_quote(&st.web.server),
                port = st.web.port
            ));
        } else {
            script.push_str(&format!("{ns} -setwebproxystate {svc} off\n"));
        }
        if st.secure.enabled {
            script.push_str(&format!(
                "{ns} -setsecurewebproxy {svc} {server} {port}\n",
                server = sh_quote(&st.secure.server),
                port = st.secure.port
            ));
        } else {
            script.push_str(&format!("{ns} -setsecurewebproxystate {svc} off\n"));
        }
        if st.had_bypass && !st.bypass.is_empty() {
            let list = st
                .bypass
                .iter()
                .map(|d| sh_quote(d))
                .collect::<Vec<_>>()
                .join(" ");
            script.push_str(&format!("{ns} -setproxybypassdomains {svc} {list}\n"));
        } else {
            script.push_str(&format!("{ns} -setproxybypassdomains {svc} Empty\n"));
        }
        privileged(&script).map(|_| ())
    }

    /// Выключить прокси, только если он указывает на наш адрес (аварийная очистка).
    /// Без запроса пароля: сработает, только если приложение запущено от root.
    pub fn disable_if_ours(addr: &str) {
        let Ok((host, port)) = split_addr(addr) else {
            return;
        };
        let Ok(service) = active_service() else {
            return;
        };
        let Ok(text) = run(NS, &["-getwebproxy", &service]) else {
            return;
        };
        let web = parse_proxy(&text);
        if web.enabled && web.server == host && web.port == port {
            let _ = run(NS, &["-setwebproxystate", &service, "off"]);
            let _ = run(NS, &["-setsecurewebproxystate", &service, "off"]);
        }
    }

    /// Выключить системный прокси.
    pub fn disable() -> Result<(), String> {
        let service = active_service()?;
        let ns = NS;
        let svc = sh_quote(&service);
        privileged(&format!(
            "{ns} -setwebproxystate {svc} off\n{ns} -setsecurewebproxystate {svc} off\n"
        ))
        .map(|_| ())
    }

    /// Если прокси включён, указывает на `addr`, но порт уже не слушается — выключаем.
    pub fn clear_stale(addr: &str) {
        let Ok((host, port)) = split_addr(addr) else {
            return;
        };
        let Ok(service) = active_service() else {
            return;
        };
        let Ok(text) = run(NS, &["-getwebproxy", &service]) else {
            return;
        };
        let web = parse_proxy(&text);
        if web.enabled && web.server == host && web.port == port && !port_open(addr) {
            let _ = disable();
        }
    }

    fn port_open(addr: &str) -> bool {
        if let Ok(mut it) = addr.to_socket_addrs() {
            if let Some(sa) = it.next() {
                return TcpStream::connect_timeout(&sa, Duration::from_millis(400)).is_ok();
            }
        }
        false
    }
}

#[cfg(windows)]
pub use imp::*;
#[cfg(target_os = "macos")]
pub use imp::*;
