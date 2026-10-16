//! Запуск встроенного ядра Xray-core (`xray.exe`).

use std::fs;
use std::os::windows::process::CommandExt;
use std::path::{Path, PathBuf};
use std::process::{Child, Command};

use serde_json::Value;

const CREATE_NO_WINDOW: u32 = 0x0800_0000;

/// Запущенное ядро Xray в комплекте с временным конфигом.
pub struct XrayProc {
    child: Child,
    cfg_path: PathBuf,
    log_path: PathBuf,
}

impl XrayProc {
    /// Запустить ядро с готовым конфигом.
    pub fn start(xray: &Path, cfg: &Value) -> Result<XrayProc, String> {
        let dir = std::env::temp_dir().join("vpn-client");
        fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
        // Расширение .json важно: Xray определяет формат по расширению.
        let cfg_path = dir.join("config.json");
        let json = serde_json::to_vec_pretty(cfg).map_err(|e| e.to_string())?;
        fs::write(&cfg_path, json).map_err(|e| e.to_string())?;

        let log_path = dir.join("xray.log");
        let out = fs::File::create(&log_path).map_err(|e| e.to_string())?;
        let err = out.try_clone().map_err(|e| e.to_string())?;

        let child = Command::new(xray)
            .arg("run")
            .arg("-c")
            .arg(&cfg_path)
            .stdout(out)
            .stderr(err)
            .creation_flags(CREATE_NO_WINDOW)
            .spawn()
            .map_err(|e| format!("не удалось запустить ядро: {e}"))?;

        Ok(XrayProc { child, cfg_path, log_path })
    }

    pub fn log_path(&self) -> &Path {
        &self.log_path
    }

    pub fn stop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        let _ = fs::remove_file(&self.cfg_path);
    }
}

impl Drop for XrayProc {
    fn drop(&mut self) {
        self.stop();
    }
}
