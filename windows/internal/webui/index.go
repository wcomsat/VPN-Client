package webui

// indexHTML — встроенная страница локального интерфейса. Без внешних ресурсов.
const indexHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>VPN-Client</title>
<style>
 :root{color-scheme:dark}
 body{font-family:system-ui,Segoe UI,Roboto,sans-serif;margin:0;background:#12161d;color:#e8eaed}
 .wrap{max-width:640px;margin:40px auto;padding:0 20px}
 h1{font-size:22px;font-weight:600}
 label{display:block;margin:14px 0 6px;color:#9aa4b2;font-size:13px}
 textarea,select{width:100%;box-sizing:border-box;background:#1b212b;color:#e8eaed;border:1px solid #2b3441;border-radius:10px;padding:10px;font:13px/1.4 ui-monospace,Consolas,monospace}
 .row{display:flex;gap:12px;margin-top:14px}
 .row>div{flex:1}
 .btns{display:flex;gap:10px;flex-wrap:wrap;margin-top:20px}
 button{flex:1;min-width:120px;padding:12px 14px;border-radius:10px;border:0;font-size:14px;cursor:pointer}
 .primary{background:#3b82f6;color:#fff}
 .green{background:#22c55e;color:#04220f}
 .red{background:#ef4444;color:#fff}
 .ghost{background:#1b212b;color:#e8eaed;border:1px solid #2b3441}
 button:disabled{opacity:.5;cursor:default}
 #status{margin-top:22px;padding:14px;border-radius:10px;background:#1b212b;border:1px solid #2b3441;font-size:14px}
 .dot{display:inline-block;width:9px;height:9px;border-radius:50%;margin-right:8px;vertical-align:middle}
 .on{background:#22c55e}.off{background:#6b7280}
 .err{color:#fca5a5;margin-top:10px;font-size:13px;white-space:pre-wrap}
</style>
</head>
<body>
<div class="wrap">
  <h1>VPN-Client</h1>
  <label>Подписка (ссылки vless://, можно две)</label>
  <textarea id="sub" rows="4" placeholder="vless://..."></textarea>
  <div class="row">
    <div>
      <label>Канал</label>
      <select id="channel">
        <option value="xhttp">xhttp — основной</option>
        <option value="vision">vision — резерв</option>
      </select>
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
  <div id="status"><span class="dot off"></span>Отключено</div>
</div>
<script>
async function api(path, body){
  var opt = {method: body ? "POST" : "GET"};
  if (body){ opt.headers = {"Content-Type":"application/json"}; opt.body = JSON.stringify(body); }
  var r = await fetch(path, opt);
  return r.json();
}
function setErr(t){ document.getElementById("err").textContent = t || ""; }
async function refresh(){
  var s = await api("/status");
  var st = document.getElementById("status");
  var dot = s.running ? '<span class="dot on"></span>' : '<span class="dot off"></span>';
  var text = s.running ? ("Подключено — режим " + s.mode + ", канал " + s.channel + ", сервер " + s.server) : "Отключено";
  st.innerHTML = dot + text;
  document.getElementById("btnOn").disabled = s.running;
  document.getElementById("btnOff").disabled = !s.running;
  if (!s.admin){
    var m = document.getElementById("mode").querySelector('[value="tun"]');
    if (m){ m.disabled = true; m.textContent = "TUN (нужен запуск от админа)"; }
  }
}
async function load(){
  var d = await api("/load");
  document.getElementById("sub").value = d.subscription || "";
}
async function save(){ setErr(""); var d = await api("/save",{subscription:document.getElementById("sub").value}); if(d.error) setErr(d.error); }
async function check(){ setErr(""); var d = await api("/parse",{subscription:document.getElementById("sub").value}); if(d.error){ setErr(d.error); } else { setErr("Каналы: " + d.channels.join(", ")); } }
async function connect(){
  setErr("");
  var d = await api("/connect",{subscription:document.getElementById("sub").value, channel:document.getElementById("channel").value, mode:document.getElementById("mode").value});
  if(d.error) setErr(d.error);
  refresh();
}
async function disconnect(){ setErr(""); var d = await api("/disconnect",{}); if(d.error) setErr(d.error); refresh(); }
load(); refresh(); setInterval(refresh, 2000);
</script>
</body>
</html>`
