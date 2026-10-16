//! Общее состояние подключения (ядро, прокси, TUN).

use std::sync::Mutex;

use crate::coreproc::XrayProc;
use crate::proxy::ProxyState;
use crate::tun::TunSession;

pub const SOCKS_PORT: u16 = 10808;
pub const HTTP_PORT: u16 = 10809;

#[derive(Default)]
pub struct Inner {
    pub xray: Option<XrayProc>,
    pub proxy: Option<ProxyState>,
    pub tun: Option<TunSession>,
    pub running: bool,
    pub mode: String,
    pub channel: String,
    pub server: String,
}

pub struct AppState {
    pub inner: Mutex<Inner>,
}

impl AppState {
    pub fn new() -> Self {
        AppState { inner: Mutex::new(Inner::default()) }
    }
}
