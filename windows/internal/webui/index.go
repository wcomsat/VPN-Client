package webui

// indexHTML — встроенная страница локального интерфейса. Без внешних ресурсов.
const indexHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>VPN</title>
<style>
 :root{color-scheme:dark;--bg:#0f131a;--card:#171c25;--line:#252c38;--muted:#98a2b3;--blue:#3b82f6;--green:#22c55e;--red:#ef4444}
 *{box-sizing:border-box}
 body{margin:0;min-height:100vh;background:radial-gradient(1200px 600px at 50% -10%,#1b2740,var(--bg));color:#e8eaed;font-family:system-ui,"Segoe UI",Roboto,sans-serif;display:flex;align-items:center;justify-content:center;padding:24px}
 .card{width:100%;max-width:560px;background:var(--card);border:1px solid var(--line);border-radius:20px;box-shadow:0 24px 60px rgba(0,0,0,.45);padding:26px}
 .head{display:flex;align-items:center;gap:14px;margin-bottom:20px}
 .logo{width:44px;height:44px;border-radius:12px;background:linear-gradient(135deg,#2563eb,#3b82f6);display:flex;align-items:center;justify-content:center;font-weight:800;font-size:24px;color:#fff;flex:0 0 auto}
 h1{font-size:18px;margin:0}
 .sub{color:var(--muted);font-size:13px;margin-top:2px}
 label{display:block;margin:16px 0 7px;color:var(--muted);font-size:13px}
 textarea,select{width:100%;background:#0f131a;color:#e8eaed;border:1px solid var(--line);border-radius:12px;padding:11px 12px;font:13px/1.45 ui-monospace,Consolas,monospace;outline:none}
 textarea:focus,select:focus{border-color:var(--blue)}
 .row{display:flex;gap:12px;margin-top:10px}
 .row>div{flex:1}
 .btns{display:flex;gap:10px;flex-wrap:wrap;margin-top:22px}
 button{flex:1;min-width:130px;padding:13px 14px;border-radius:12px;border:0;font-size:14px;font-weight:600;cursor:pointer;transition:filter .15s}
 button:hover{filter:brightness(1.08)}
 button:disabled{opacity:.45;cursor:default}
 .green{background:var(--green);color:#04220f}
 .red{background:var(--red);color:#fff}
 .ghost{background:#10151d;color:#cbd5e1;border:1px solid var(--line)}
 .status{margin-top:22px;padding:14px 16px;border-radius:12px;background:#10151d;border:1px solid var(--line);font-size:14px;display:flex;align-items:center;gap:10px}
 .dot{width:10px;height:10px;border-radius:50%;background:#6b7280;flex:0 0 auto;box-shadow:0 0 0 4px rgba(107,114,128,.15)}
 .dot.on{background:var(--green);box-shadow:0 0 0 4px rgba(34,197,94,.18)}
 .err{color:#fca5a5;margin-top:12px;font-size:13px;white-space:pre-wrap;min-height:1em}
 form{display:contents}
</style>
</head>
<body>
<div class="card">
  <div class="head">
    <div class="logo">V</div>
    <div>
      <h1>VPN</h1>
      <div class="sub">Личный доступ · Нидерланды</div>
    </div>
  </div>

  <label>Подписка (две ссылки vless:// — по одной в строке)</label>
  <textarea id="sub" rows="4" placeholder="vless://..."></textarea>

  <div class="row">
    <div>
      <label>Канал</label>
      <select id="channel"></select>
    </div>
    <div>
      <label>Режим</label>
      <select id="mode">
        <option value="proxy">Прокси (браузеры)</option>
        <option value="tun">TUN (весь трафик)</option>
      </select>
    </div>
  </div>

  <div class="btns">
    <button class="ghost" onclick="save()">Сохранить</button>
    <button class="ghost" onclick="check()">Проверить</button>
    <button id="btnOn" class="green" onclick="connect()">Подключить</button>
    <button id="btnOff" class="red" onclick="disconnect()">Отключить</button>
  </div>

  <div class="err" id="err"></div>
  <div class="status" id="status"><span class="dot"></span><span>Отключено</span></div>
</div>
<script>
async function api(path, body){
  var opt = {method: body ? "POST" : "GET"};
  if (body){ opt.headers = {"Content-Type":"application/json"}; opt.body = JSON.stringify(body); }
  var r = await fetch(path, opt);
  return r.json();
}
function setErr(t){ document.getElementById("err").textContent = t || ""; }
function mapName(t){ return t === "tcp" ? "vision" : t; }
function label(n){ if(n === "xhttp"){ return "xhttp — основной"; } if(n === "vision"){ return "vision — резерв"; } return n; }
async function refresh(){
  var s = await api("/status");
  var dot = document.querySelector("#status .dot");
  var txt = document.querySelector("#status span:last-child");
  if (s.running){ dot.classList.add("on"); txt.textContent = "Подключено — " + s.mode + ", канал " + s.channel + ", " + s.server; }
  else { dot.classList.remove("on"); txt.textContent = "Отключено"; }
  document.getElementById("btnOn").disabled = s.running;
  document.getElementById("btnOff").disabled = !s.running;
  if (!s.admin){ var m = document.getElementById("mode").querySelector('[value="tun"]'); if (m){ m.disabled = true; m.textContent = "TUN (нужен запуск от админа)"; } }
}
async function load(){
  var d = await api("/load");
  document.getElementById("sub").value = d.subscription || "";
}
async function check(){
  setErr("");
  var d = await api("/parse",{subscription:document.getElementById("sub").value});
  var sel = document.getElementById("channel");
  if (d.error){ setErr(d.error); sel.innerHTML = ""; return; }
  var names = (d.channels || []).map(mapName);
  var cur = sel.value;
  sel.innerHTML = "";
  names.forEach(function(n){ var o = document.createElement("option"); o.value = n; o.textContent = label(n); sel.appendChild(o); });
  if (names.indexOf(cur) >= 0){ sel.value = cur; }
  setErr("Каналы в подписке: " + (names.length ? names.join(", ") : "нет"));
}
async function save(){ setErr(""); var d = await api("/save",{subscription:document.getElementById("sub").value}); if(d.error){ setErr(d.error); } else { await check(); } }
async function connect(){
  setErr("");
  await save();
  var ch = document.getElementById("channel").value;
  if(!ch){ setErr("Вставьте подписку и нажмите «Проверить»"); return; }
  var d = await api("/connect",{subscription:document.getElementById("sub").value, channel:ch, mode:document.getElementById("mode").value});
  if(d.error) setErr(d.error);
  refresh();
}
async function disconnect(){ setErr(""); var d = await api("/disconnect",{}); if(d.error) setErr(d.error); refresh(); }
load().then(function(){ return refresh(); }).then(check);
setInterval(refresh, 2000);
</script>
</body>
</html>`
