const { invoke } = window.__TAURI__.core;
const tauriWindow = window.__TAURI__.window;

// Перетаскивание окна за любую область, кроме интерактивных элементов.
if (tauriWindow && tauriWindow.getCurrentWindow) {
  document.addEventListener("mousedown", (e) => {
    if (e.button !== 0) return;
    if (e.detail > 1) return;
    const t = e.target;
    if (t.closest("button, a, input, textarea, select, option, [data-no-drag]")) return;
    tauriWindow.getCurrentWindow().startDragging().catch(() => {});
  });
}

const S = { sub: "", channels: [], channel: "", mode: "proxy", running: false, admin: false };

const $ = (id) => document.getElementById(id);

function show(el) { el.classList.remove("hidden"); }
function hide(el) { el.classList.add("hidden"); }
function showSetup() { hide($("main")); show($("setup")); }
function showMain() { hide($("setup")); show($("main")); }

function mapChannel(t) { return t === "tcp" ? "vision" : t; }
function labelChannel(c) { return c === "vision" ? "Vision" : "xHTTP"; }

async function init() {
  S.sub = (await invoke("load_subscription")) || "";
  $("sub").value = S.sub;
  const st = await invoke("get_status");
  S.admin = st.admin;
  if (!S.sub) { showSetup(); return; }
  await rebuildChannels();
  showMain();
  await refresh();
  setInterval(refresh, 1500);
}

async function rebuildChannels() {
  try {
    S.channels = await invoke("parse_channels", { text: S.sub });
  } catch { S.channels = []; }
  if (!S.channels.includes(S.channel)) S.channel = S.channels[0] || "";
  renderChannels();
}

function renderChannels() {
  const box = $("channels");
  box.innerHTML = "";
  if (!S.channels.length) {
    box.textContent = "—";
    return;
  }
  S.channels.forEach((c) => {
    const b = document.createElement("button");
    b.className = "seg-btn" + (c === S.channel ? " active" : "");
    b.textContent = labelChannel(c);
    b.onclick = () => { S.channel = c; renderChannels(); };
    box.appendChild(b);
  });
}

function renderModes() {
  document.querySelectorAll("#modes .seg-btn").forEach((b) => {
    const m = b.dataset.mode;
    b.classList.toggle("active", m === S.mode);
    if (m === "tun" && !S.admin) {
      b.classList.add("locked");
    } else {
      b.classList.remove("locked");
    }
  });
  const hint = $("mode-hint");
  if (S.mode === "tun") {
    hint.textContent = S.admin
      ? "TUN — весь трафик через VPN (включая игры и UDP)."
      : "TUN — весь трафик, но нужны права администратора. Нажмите TUN, чтобы перезапустить.";
  } else {
    hint.textContent = "Прокси — браузеры и приложения. Без прав администратора.";
  }
}

async function refresh() {
  let st;
  try { st = await invoke("get_status"); } catch { return; }
  S.running = st.running;
  S.admin = st.admin;
  if (st.mode) S.mode = st.mode;

  const power = $("power");
  power.classList.toggle("on", S.running);
  power.classList.toggle("off", !S.running);
  $("state").textContent = S.running ? "Подключено" : "Отключено";
  $("detail").textContent = S.running
    ? `${labelChannel(mapChannel(st.channel))} · ${st.mode === "tun" ? "TUN" : "Прокси"} · ${st.server}`
    : "Готово к подключению";
  renderModes();
}

async function connect() {
  if (!S.sub) { showSetup(); return; }
  $("err").textContent = "";
  const power = $("power");
  power.classList.remove("disconnecting");
  power.classList.add("connecting");
  try {
    await invoke("save_subscription", { text: S.sub });
    await invoke("connect", { subscription: S.sub, channel: S.channel, mode: S.mode });
  } catch (e) {
    $("err").textContent = String(e);
  }
  setTimeout(() => power.classList.remove("connecting"), 900);
  await refresh();
}

async function disconnect() {
  $("err").textContent = "";
  const power = $("power");
  power.classList.remove("connecting");
  power.classList.add("disconnecting");
  try { await invoke("disconnect"); } catch (e) { $("err").textContent = String(e); }
  setTimeout(() => power.classList.remove("disconnecting"), 900);
  await refresh();
}

$("power").onclick = () => (S.running ? disconnect() : connect());

document.querySelectorAll("#modes .seg-btn").forEach((b) => {
  b.onclick = async () => {
    const m = b.dataset.mode;
    if (m === "tun" && !S.admin) {
      const ok = confirm(
        "Режим TUN направляет ВЕСЬ трафик (браузеры, приложения, игры) через VPN.\n\n" +
        "Для него нужны права администратора. Перезапустить приложение с правами администратора?"
      );
      $("err").textContent = "";
      if (ok) {
        try { await invoke("elevate"); } catch (e) { $("err").textContent = String(e); }
      }
      return;
    }
    S.mode = m;
    renderModes();
  };
});

$("edit").onclick = () => showSetup();

$("winClose").onclick = () => { invoke("hide_window").catch(() => {}); };

$("save").onclick = async () => {
  const text = $("sub").value.trim();
  $("setup-err").textContent = "";
  try {
    if (!text) throw new Error("Вставьте подписку");
    await invoke("save_subscription", { text });
    S.sub = text;
    const st = await invoke("get_status");
    S.admin = st.admin;
    await rebuildChannels();
    if (!S.channels.length) throw new Error("Не удалось разобрать подписку — проверьте ссылки");
    showMain();
    await refresh();
    if (!window.__timer) window.__timer = setInterval(refresh, 1500);
  } catch (e) {
    $("setup-err").textContent = String(e);
  }
};

init();
