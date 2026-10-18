//! TUN-режим: Windows (Wintun + tun2socks) и macOS (utun + tun2socks).

#[cfg(windows)]
mod imp {
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
        std::ffi::OsStr::new(s)
            .encode_wide()
            .chain(std::iter::once(0))
            .collect()
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

    /// Убрать маршруты, оставшиеся от прошлого запуска.
    pub fn cleanup_leftovers() {
        let _ = netsh(&[
            "interface",
            "ipv4",
            "delete",
            "route",
            "0.0.0.0/0",
            "wintun",
            "192.168.123.1",
        ]);
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
                "--device",
                "wintun",
                "--proxy",
                &format!("socks5://{socks}"),
                "--interface",
                &alias,
                "--loglevel",
                "warn",
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
            return Err(
                "интерфейс wintun не появился (нужен wintun.dll рядом с tun2socks.exe)".into(),
            );
        }

        for s in [
            &[
                "interface",
                "ipv4",
                "set",
                "address",
                "name=wintun",
                "source=static",
                "addr=192.168.123.1",
                "mask=255.255.255.0",
            ][..],
            &[
                "interface",
                "ipv4",
                "set",
                "dnsservers",
                "name=wintun",
                "static",
                "address=1.1.1.1",
                "register=none",
                "validate=no",
            ][..],
        ] {
            if let Err(e) = netsh(s) {
                session.stop();
                return Err(format!("netsh {}: {e}", s.join(" ")));
            }
        }

        let host = format!("{server_ip}/32");
        let alias = session.alias.clone();
        let gateway = session.gateway.clone();
        if let Err(e) = netsh(&[
            "interface",
            "ipv4",
            "add",
            "route",
            &host,
            &alias,
            &gateway,
            "metric=1",
        ]) {
            session.stop();
            return Err(format!("netsh host-route: {e}"));
        }

        if let Err(e) = netsh(&[
            "interface",
            "ipv4",
            "add",
            "route",
            "0.0.0.0/0",
            "wintun",
            "192.168.123.1",
            "metric=1",
        ]) {
            session.stop();
            return Err(format!("netsh default-route: {e}"));
        }

        Ok(session)
    }

    impl TunSession {
        /// Жив ли процесс tun2socks.
        pub fn alive(&mut self) -> bool {
            matches!(self.child.try_wait(), Ok(None))
        }

        pub fn stop(&mut self) {
            let _ = netsh(&[
                "interface",
                "ipv4",
                "delete",
                "route",
                "0.0.0.0/0",
                "wintun",
                "192.168.123.1",
            ]);
            let host = format!("{}/32", self.server_ip);
            let _ = netsh(&[
                "interface",
                "ipv4",
                "delete",
                "route",
                &host,
                &self.alias,
                &self.gateway,
            ]);
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
}

#[cfg(target_os = "macos")]
mod imp {
    use std::fs;
    use std::path::{Path, PathBuf};
    use std::process::{Command, Stdio};

    use serde::{Deserialize, Serialize};

    use crate::macos::{default_route, ensure_executable, privileged, run, service_for, sh_quote};

    #[derive(Serialize, Deserialize)]
    struct Saved {
        tun: String,
        iface: String,
        gateway: String,
        server: String,
        service: String,
        dns: Vec<String>,
    }

    fn dir() -> PathBuf {
        std::env::temp_dir().join("vpn-client")
    }

    fn state_path() -> PathBuf {
        dir().join("tun-state.json")
    }

    fn pid_path() -> PathBuf {
        dir().join("tun2socks.pid")
    }

    /// На macOS права запрашиваются на каждую операцию, поэтому TUN доступен
    /// без отдельного перезапуска приложения.
    pub fn is_admin() -> bool {
        true
    }

    pub fn elevate() -> Result<(), String> {
        Ok(())
    }

    /// Доделать уборку, если прошлый запуск завершился аварийно.
    pub fn cleanup_leftovers() {
        let Ok(text) = fs::read_to_string(state_path()) else {
            return;
        };
        let Ok(saved) = serde_json::from_str::<Saved>(&text) else {
            let _ = fs::remove_file(state_path());
            return;
        };
        if privileged(&stop_script(&saved)).is_ok() {
            let _ = fs::remove_file(state_path());
            let _ = fs::remove_file(pid_path());
        }
    }

    /// Активная сессия TUN.
    pub struct TunSession {
        saved: Saved,
        pidfile: PathBuf,
        stopped: bool,
    }

    /// Поднять utun через tun2socks и направить в него весь IPv4-трафик.
    pub fn start(tun2socks: &Path, socks: &str, server_ip: &str) -> Result<TunSession, String> {
        let (iface, gateway) = default_route()?;
        let service = service_for(&iface)?;
        let tun = free_utun()?;
        let dns = read_dns(&service)?;
        ensure_executable(tun2socks);

        let saved = Saved {
            tun: tun.clone(),
            iface,
            gateway,
            server: server_ip.to_string(),
            service,
            dns,
        };
        fs::create_dir_all(dir()).map_err(|e| e.to_string())?;
        fs::write(
            state_path(),
            serde_json::to_vec_pretty(&saved).map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;

        if let Err(e) = privileged(&start_script(tun2socks, socks, &saved)) {
            let _ = fs::remove_file(state_path());
            return Err(e);
        }

        if run("/sbin/ifconfig", &[tun.as_str()]).is_err() {
            let _ = privileged(&stop_script(&saved));
            let _ = fs::remove_file(state_path());
            return Err(format!(
                "интерфейс {tun} не появился — проверьте лог {}",
                dir().join("tun2socks.log").display()
            ));
        }

        Ok(TunSession {
            saved,
            pidfile: pid_path(),
            stopped: false,
        })
    }

    impl TunSession {
        /// Жив ли процесс tun2socks.
        pub fn alive(&mut self) -> bool {
            Command::new("/usr/bin/pgrep")
                .arg("-F")
                .arg(&self.pidfile)
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .status()
                .map(|s| s.success())
                .unwrap_or(false)
        }

        pub fn stop(&mut self) {
            if self.stopped {
                return;
            }
            self.stopped = true;
            if privileged(&stop_script(&self.saved)).is_ok() {
                let _ = fs::remove_file(state_path());
                let _ = fs::remove_file(&self.pidfile);
            }
        }
    }

    impl Drop for TunSession {
        fn drop(&mut self) {
            self.stop();
        }
    }

    fn free_utun() -> Result<String, String> {
        for n in 100..200 {
            let name = format!("utun{n}");
            let exists = Command::new("/sbin/ifconfig")
                .arg(&name)
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .status()
                .map(|s| s.success())
                .unwrap_or(false);
            if !exists {
                return Ok(name);
            }
        }
        Err("не удалось найти свободный utun".into())
    }

    fn read_dns(service: &str) -> Result<Vec<String>, String> {
        let text = run("/usr/sbin/networksetup", &["-getdnsservers", service]).unwrap_or_default();
        if text.contains("There aren't any") {
            return Ok(Vec::new());
        }
        Ok(text
            .lines()
            .map(str::trim)
            .filter(|s| !s.is_empty())
            .map(String::from)
            .collect())
    }

    fn dns_restore(saved: &Saved) -> String {
        if saved.dns.is_empty() {
            "Empty".to_string()
        } else {
            saved
                .dns
                .iter()
                .map(|d| sh_quote(d))
                .collect::<Vec<_>>()
                .join(" ")
        }
    }

    fn start_script(tun2socks: &Path, socks: &str, s: &Saved) -> String {
        let t2s = sh_quote(&tun2socks.to_string_lossy());
        let tun = sh_quote(&s.tun);
        let socks = sh_quote(&format!("socks5://{socks}"));
        let iface = sh_quote(&s.iface);
        let log = sh_quote(&dir().join("tun2socks.log").to_string_lossy());
        let pid = sh_quote(&pid_path().to_string_lossy());
        let svc = sh_quote(&s.service);
        let server = sh_quote(&s.server);
        let gateway = sh_quote(&s.gateway);

        format!(
            r#"#!/bin/sh
set -e
cleanup() {{
  if [ -f {pid} ]; then kill "$(cat {pid})" 2>/dev/null || true; fi
  rm -f {pid}
  /sbin/route -n delete -net 0.0.0.0/1 198.18.0.1 >/dev/null 2>&1 || true
  /sbin/route -n delete -net 128.0.0.0/1 198.18.0.1 >/dev/null 2>&1 || true
  /sbin/route -n delete -host {server} >/dev/null 2>&1 || true
  /sbin/ifconfig {tun} down >/dev/null 2>&1 || true
  /usr/sbin/networksetup -setdnsservers {svc} {dns}
}}
ok=0
trap '[ "$ok" = 1 ] || cleanup' EXIT
{t2s} --device {tun} --proxy {socks} --interface {iface} --loglevel warn >{log} 2>&1 &
echo $! > {pid}
i=0
while [ $i -lt 100 ]; do
  /sbin/ifconfig {tun} >/dev/null 2>&1 && break
  sleep 0.2
  i=$((i+1))
done
/sbin/ifconfig {tun} 198.18.0.1 198.18.0.1 up
/sbin/route -n delete -net 0.0.0.0/1 198.18.0.1 >/dev/null 2>&1 || true
/sbin/route -n delete -net 128.0.0.0/1 198.18.0.1 >/dev/null 2>&1 || true
/sbin/route -n add -host {server} {gateway} >/dev/null 2>&1 || true
/sbin/route -n add -net 0.0.0.0/1 198.18.0.1
/sbin/route -n add -net 128.0.0.0/1 198.18.0.1
/usr/sbin/networksetup -setdnsservers {svc} 1.1.1.1
ok=1
"#,
            dns = dns_restore(s),
        )
    }

    fn stop_script(s: &Saved) -> String {
        let tun = sh_quote(&s.tun);
        let server = sh_quote(&s.server);
        let svc = sh_quote(&s.service);
        let pid = sh_quote(&pid_path().to_string_lossy());

        format!(
            r#"#!/bin/sh
if [ -f {pid} ]; then kill "$(cat {pid})" 2>/dev/null || true; fi
rm -f {pid}
/sbin/route -n delete -net 0.0.0.0/1 198.18.0.1 >/dev/null 2>&1 || true
/sbin/route -n delete -net 128.0.0.0/1 198.18.0.1 >/dev/null 2>&1 || true
/sbin/route -n delete -host {server} >/dev/null 2>&1 || true
/sbin/ifconfig {tun} down >/dev/null 2>&1 || true
/usr/sbin/networksetup -setdnsservers {svc} {dns}
"#,
            dns = dns_restore(s),
        )
    }
}

#[cfg(windows)]
pub use imp::*;
#[cfg(target_os = "macos")]
pub use imp::*;
