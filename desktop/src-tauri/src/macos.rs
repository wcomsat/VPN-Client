//! macOS: привилегированные команды через `osascript`, сетевые параметры системы.

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicU32, Ordering};

static SEQ: AtomicU32 = AtomicU32::new(0);

/// Обернуть строку в одинарные кавычки для `/bin/sh`.
pub fn sh_quote(s: &str) -> String {
    format!("'{}'", s.replace('\'', "'\\''"))
}

/// Выполнить shell-скрипт с правами администратора (один запрос пароля).
pub fn privileged(script: &str) -> Result<String, String> {
    let dir = std::env::temp_dir().join("vpn-client");
    fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    let n = SEQ.fetch_add(1, Ordering::Relaxed);
    let path: PathBuf = dir.join(format!("privileged-{}-{}.sh", std::process::id(), n));
    fs::write(&path, script).map_err(|e| e.to_string())?;

    let cmd = format!("/bin/sh {}", sh_quote(&path.to_string_lossy()));
    let apple = format!(
        "do shell script \"{}\" with administrator privileges",
        cmd.replace('\\', "\\\\").replace('"', "\\\"")
    );

    let out = Command::new("/usr/bin/osascript")
        .args(["-e", &apple])
        .output()
        .map_err(|e| format!("osascript: {e}"))?;
    let _ = fs::remove_file(&path);

    if out.status.success() {
        Ok(String::from_utf8_lossy(&out.stdout).trim().to_string())
    } else {
        let err = String::from_utf8_lossy(&out.stderr).trim().to_string();
        if err.contains("-128") || err.to_lowercase().contains("cancel") {
            Err("запрос прав администратора отменён".into())
        } else {
            Err(format!(
                "команда с правами администратора не выполнена: {err}"
            ))
        }
    }
}

/// Выполнить команду без повышения прав.
pub fn run(program: &str, args: &[&str]) -> Result<String, String> {
    let out = Command::new(program)
        .args(args)
        .output()
        .map_err(|e| format!("{program}: {e}"))?;
    if out.status.success() {
        Ok(String::from_utf8_lossy(&out.stdout).to_string())
    } else {
        Err(String::from_utf8_lossy(&out.stderr).trim().to_string())
    }
}

/// Основной сетевой интерфейс и его шлюз.
pub fn default_route() -> Result<(String, String), String> {
    let text = run("/sbin/route", &["-n", "get", "default"])?;
    let mut iface = String::new();
    let mut gateway = String::new();
    for line in text.lines() {
        let line = line.trim();
        if let Some(v) = line.strip_prefix("interface:") {
            iface = v.trim().to_string();
        } else if let Some(v) = line.strip_prefix("gateway:") {
            gateway = v.trim().to_string();
        }
    }
    if iface.is_empty() || gateway.is_empty() {
        return Err("не удалось определить основной интерфейс/шлюз".into());
    }
    Ok((iface, gateway))
}

/// Имя сетевого сервиса macOS для интерфейса (Wi-Fi, Ethernet, …).
pub fn service_for(iface: &str) -> Result<String, String> {
    let text = run("/usr/sbin/networksetup", &["-listnetworkserviceorder"])?;
    let mut service = String::new();
    for line in text.lines() {
        let line = line.trim();
        if line.starts_with('(') && !line.contains("Hardware Port") {
            service = line
                .split_once(')')
                .map(|(_, rest)| rest.trim().to_string())
                .unwrap_or_default();
            continue;
        }
        if let Some(rest) = line.strip_prefix("(Hardware Port:") {
            if let Some(pos) = rest.find("Device:") {
                let device = rest[pos + "Device:".len()..]
                    .trim()
                    .trim_end_matches(')')
                    .trim();
                if device == iface && !service.is_empty() {
                    return Ok(service);
                }
            }
        }
    }
    Err(format!("не найден сетевой сервис для {iface}"))
}

/// Сделать файл исполняемым (Tauri может потерять права при упаковке).
pub fn ensure_executable(p: &Path) {
    use std::os::unix::fs::PermissionsExt;
    if let Ok(meta) = fs::metadata(p) {
        let mut perm = meta.permissions();
        if perm.mode() & 0o111 == 0 {
            perm.set_mode(perm.mode() | 0o755);
            let _ = fs::set_permissions(p, perm);
        }
    }
}
