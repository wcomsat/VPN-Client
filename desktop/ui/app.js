const { invoke } = window.__TAURI__.core;

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
}

async function refresh() {
  let st;
  try { st = await invoke("get_status"); } catch { return; }
  S.running = st.running;
  S.admin = st.admin;
  if (st.mode) S.mode = st.mode;

  const power = $("power");
  power.classList.toggle("on", S.running);
  $("state").textContent = S.running ? "Подключено" : "Отключено";
  $("detail").textContent = S.running
    ? `${labelChannel(mapChannel(st.channel))} · ${st.mode === "tun" ? "TUN" : "Прокси"} · ${st.server}`
    : "Готово к подключению";
  renderModes();
}

async function connect() {
  if (!S.sub) { showSetup(); return; }
  $("err").textContent = "";
  $("power").classList.add("busy");
  try {
    await invoke("save_subscription", { text: S.sub });
    await invoke("connect", { subscription: S.sub, channel: S.channel, mode: S.mode });
  } catch (e) {
    $("err").textContent = String(e);
  }
  $("power").classList.remove("busy");
  await refresh();
}

async function disconnect() {
  $("err").textContent = "";
  try { await invoke("disconnect"); } catch (e) { $("err").textContent = String(e); }
  await refresh();
}

$("power").onclick = () => (S.running ? disconnect() : connect());

document.querySelectorAll("#modes .seg-btn").forEach((b) => {
  b.onclick = async () => {
    const m = b.dataset.mode;
    if (m === "tun" && !S.admin) {
      $("err").textContent = "Для режима TUN приложение нужно запустить от имени администратора.";
      try { await invoke("elevate"); } catch (e) { $("err").textContent = String(e); }
      return;
    }
    S.mode = m;
    renderModes();
  };
});

$("edit").onclick = () => showSetup();

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
