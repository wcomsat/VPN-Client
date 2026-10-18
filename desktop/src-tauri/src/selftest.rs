//! Самопроверка бэкенда без UI: парсинг → запуск ядра → SOCKS → прокси.
//! Запуск: `cargo run -p vpn-desktop -- selftest`

use std::path::PathBuf;
use std::time::Duration;

use vpn_core::{build_core_config, parse_subscription, pick_channel};

use crate::coreproc::XrayProc;
use crate::{commands, proxy, state};

pub fn run() -> Result<(), String> {
    let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let links = std::fs::read_to_string(manifest.join("../../shared/fixtures/sample-links.txt"))
        .map_err(|e| format!("не читается фикстура: {e}"))?;
    let chans = parse_subscription(&links)?;
    let ch = pick_channel(&chans, "xhttp")?;
    let cfg = build_core_config(&ch, state::SOCKS_PORT, state::HTTP_PORT);

    #[cfg(windows)]
    let (bin, hint) = ("resources/xray.exe", "tools/fetch-core.ps1");
    #[cfg(not(windows))]
    let (bin, hint) = ("resources/xray", "tools/fetch-core.sh");

    let xray = manifest.join(bin);
    if !xray.exists() {
        return Err(format!("нет {bin} — запустите {hint}"));
    }

    println!("selftest: запуск ядра xray…");
    let mut proc = XrayProc::start(&xray, &cfg)?;
    if !commands::wait_port(state::SOCKS_PORT, Duration::from_secs(10)) {
        proc.stop();
        return Err(format!(
            "SOCKS не поднялся (лог: {})",
            proc.log_path().display()
        ));
    }
    println!("selftest: SOCKS 127.0.0.1:{} — OK", state::SOCKS_PORT);

    println!("selftest: включение системного прокси…");
    let st = proxy::set(&format!("127.0.0.1:{}", state::HTTP_PORT))?;
    proxy::restore(&st)?;
    println!("selftest: прокси включён и восстановлен — OK");

    proc.stop();
    println!("selftest: OK");
    Ok(())
}
