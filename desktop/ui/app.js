// Фронтенд. На M0 — проверка связи с Rust-ядром; дальше — подключение/статус.
const { invoke } = window.__TAURI__.core;

async function boot() {
  try {
    document.getElementById("ping").textContent = await invoke("ping");
  } catch (e) {
    document.getElementById("ping").textContent = "ошибка: " + e;
  }
}

boot();
