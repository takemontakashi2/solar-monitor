package main

const indexHTML = `<!doctype html>
<html lang="ja">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Solar Monitor</title>
  <style>
    :root { color-scheme: light dark; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f6f7f2; color: #20231f; }
    main { max-width: 1120px; margin: 0 auto; padding: 24px; }
    header { display: flex; justify-content: space-between; align-items: end; gap: 16px; margin-bottom: 20px; }
    h1 { font-size: clamp(28px, 5vw, 56px); margin: 0; letter-spacing: 0; }
    .header-actions { display: flex; align-items: center; gap: 12px; }
    .time { color: #62675e; font-size: 14px; }
    .icon-button { border: 1px solid #cfd6c6; background: #ffffff; color: #20231f; border-radius: 8px; padding: 8px 10px; font-size: 18px; line-height: 1; cursor: pointer; }
    .icon-button[aria-pressed="true"] { background: #e8efe0; border-color: #9dad8d; }
    .grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 18px; }
    .card { background: #ffffff; border: 1px solid #dfe3d8; border-radius: 8px; padding: 16px; min-height: 100px; }
    .label { color: #62675e; font-size: 13px; overflow-wrap: anywhere; }
    .value { font-size: clamp(26px, 4vw, 42px); font-weight: 750; margin-top: 8px; line-height: 1.05; overflow-wrap: anywhere; }
    .unit { font-size: 16px; color: #62675e; margin-left: 4px; }
    .health-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-bottom: 18px; }
    .health-card { background: #ffffff; border: 1px solid #dfe3d8; border-radius: 8px; padding: 14px; }
    .health-title { font-weight: 700; margin-bottom: 10px; }
    .health-row { display: flex; justify-content: space-between; align-items: center; gap: 12px; border-top: 1px solid #edf0e8; padding: 8px 0; font-size: 14px; }
    .health-row:first-of-type { border-top: 0; }
    .health-value { font-weight: 720; overflow-wrap: anywhere; text-align: right; }
    .state-icon { display: inline-grid; place-items: center; width: 28px; height: 28px; border-radius: 999px; font-size: 18px; font-weight: 800; line-height: 1; }
    .state-on { color: #ffffff; background: #168a45; }
    .state-off { color: #ffffff; background: #bd2d2d; }
    .state-unknown { color: #62675e; background: #edf0e8; }
    .error-ok { color: #62675e; }
    .error-bad { color: #bd2d2d; }
    .chart { background: #ffffff; border: 1px solid #dfe3d8; border-radius: 8px; padding: 12px; margin-bottom: 18px; }
    canvas { width: 100%; height: 360px; display: block; }
    .panel { background: #ffffff; border: 1px solid #dfe3d8; border-radius: 8px; padding: 16px; margin-bottom: 18px; }
    .panel h2 { font-size: 18px; margin: 0 0 12px; }
    .panel[hidden] { display: none; }
    .prop-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 18px; }
    .prop-value { font-size: 24px; font-weight: 720; margin-top: 8px; line-height: 1.15; overflow-wrap: anywhere; }
    .prop-key { color: #62675e; font-size: 12px; margin-top: 8px; }
    .chooser { display: grid; gap: 14px; max-height: 320px; overflow: auto; padding-right: 4px; }
    .device-group { border-top: 1px solid #e4e8dd; padding-top: 12px; }
    .device-group:first-child { border-top: 0; padding-top: 0; }
    .device-title { font-size: 14px; font-weight: 700; margin-bottom: 8px; }
    .choice-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px 14px; }
    .choice { display: grid; grid-template-columns: 20px minmax(0, 1fr); gap: 8px; align-items: start; font-size: 13px; line-height: 1.35; }
    .choice input { margin-top: 2px; }
    .choice small { color: #62675e; display: block; overflow-wrap: anywhere; }
    .stats { margin-top: 18px; border-top: 1px solid #e4e8dd; padding-top: 14px; }
    .stats-head { display: flex; justify-content: space-between; gap: 12px; align-items: center; margin-bottom: 8px; }
    .stats-head h3 { font-size: 16px; margin: 0; }
    .stats-note { color: #62675e; font-size: 12px; }
    .stats-table-wrap { overflow-x: auto; }
    .stats-table { width: 100%; border-collapse: collapse; font-size: 13px; min-width: 760px; }
    .stats-table th, .stats-table td { border-bottom: 1px solid #e4e8dd; padding: 8px 6px; text-align: right; white-space: nowrap; }
    .stats-table th:first-child, .stats-table td:first-child, .stats-table th:nth-child(2), .stats-table td:nth-child(2) { text-align: left; }
    .stats-table tbody tr:hover { background: #f7f9f4; }
    @media (max-width: 900px) { .grid, .prop-grid, .health-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .choice-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    @media (max-width: 760px) { main { padding: 14px; } header { align-items: start; flex-direction: column; } canvas { height: 300px; } .choice-grid, .health-grid { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
<main>
  <header>
    <h1>Solar Monitor</h1>
    <div class="header-actions"><div class="time" id="updated">--</div><button class="icon-button" id="settingsButton" type="button" aria-label="表示設定" aria-pressed="false">⚙</button></div>
  </header>
  <section class="grid">
    <div class="card"><div class="label">発電</div><div class="value"><span id="pv">--</span><span class="unit">kW</span></div></div>
    <div class="card"><div class="label">消費</div><div class="value"><span id="load">--</span><span class="unit">kW</span></div></div>
    <div class="card"><div class="label">買電 / 売電</div><div class="value"><span id="grid">--</span><span class="unit">kW</span></div></div>
    <div class="card"><div class="label">今日の発電</div><div class="value"><span id="today">--</span><span class="unit">kWh</span></div></div>
  </section>
  <section class="prop-grid" id="selectedProps"></section>
  <section class="chart"><canvas id="chart"></canvas></section>
  <section class="health-grid" id="healthGrid"></section>
  <section class="panel" id="settingsPanel" hidden>
    <h2>表示する値</h2>
    <div class="chooser" id="propertyChooser"></div>
    <div class="stats">
      <div class="stats-head">
        <h3>候補診断</h3>
        <div class="stats-note">直近データの変化量が大きい順</div>
      </div>
      <div class="stats-table-wrap">
        <table class="stats-table">
          <thead><tr><th>機器</th><th>EPC</th><th>最新</th><th>最小</th><th>最大</th><th>変化</th><th>幅</th><th>回数</th></tr></thead>
          <tbody id="propertyStats"><tr><td colspan="8">--</td></tr></tbody>
        </table>
      </div>
    </div>
  </section>
</main>
<script>
const $ = (id) => document.getElementById(id);
const storageKey = 'solar-monitor.selected-properties';
const settingsOpenKey = 'solar-monitor.settings-open';
const defaultSelected = ['027901:e0', '027901:e1', '027d01:e4', '027d01:cf', '027d01:da'];
const healthDevices = [
  {eoj: '027901', name: 'パネル'},
  {eoj: '027d01', name: '電池'},
  {eoj: '05ff01', name: 'コントローラー'}
];
const healthEpcs = ['80', '88', '86'];
const healthKeys = healthDevices.flatMap(device => healthEpcs.map(epc => device.eoj + ':' + epc));
const chartSampleLimit = 720;
let selected = new Set(JSON.parse(localStorage.getItem(storageKey) || 'null') || defaultSelected);
let settingsOpen = localStorage.getItem(settingsOpenKey) === 'true';
const has = (v) => typeof v === 'number' && Number.isFinite(v);
const fmtKW = (w) => has(w) ? (w / 1000).toFixed(2) : '--';
const fmtKWh = (v) => has(v) ? v.toFixed(1) : '--';
const fmtStat = (v) => has(v) ? String(Math.round(v * 1000) / 1000) : '--';
const keyOf = (p) => p.eoj + ':' + p.epc;
const labelOf = (p) => p.name || keyOf(p);
const deviceName = (eoj) => ({'05ff01':'コントローラ', '027901':'太陽光発電', '027d01':'蓄電池'})[eoj] || eoj;
function applySettingsVisibility() {
  $('settingsPanel').hidden = !settingsOpen;
  $('settingsButton').setAttribute('aria-pressed', String(settingsOpen));
}
function saveSelected() {
  localStorage.setItem(storageKey, JSON.stringify([...selected]));
}
function displayValue(p) {
  if (p.description) return p.description;
  if (has(p.float)) return String(p.float);
  if (p.raw) return '0x' + p.raw;
  return '--';
}
async function refresh() {
  const res = await fetch('/api/samples?limit=' + chartSampleLimit, {cache: 'no-store'});
  const samples = await res.json();
  const latest = samples[samples.length - 1];
  if (!latest) return;
  $('pv').textContent = fmtKW(latest.pv_watts);
  $('load').textContent = fmtKW(latest.load_watts);
  $('grid').textContent = has(latest.grid_watts) ? fmtKW(Math.abs(latest.grid_watts)) : '--';
  $('today').textContent = fmtKWh(latest.today_kwh);
  $('updated').textContent = new Date(latest.time).toLocaleString();
  refreshSelectedProperties();
  if (settingsOpen) {
    refreshAllProperties();
    refreshStats();
  }
  draw(samples);
}
async function refreshSelectedProperties() {
  const keysToFetch = [...new Set([...selected, ...healthKeys])];
  if (keysToFetch.length === 0) {
    renderSelectedProperties([]);
    renderHealth([]);
    return;
  }
  const keys = encodeURIComponent(keysToFetch.join(','));
  const res = await fetch('/api/properties?limit=1&keys=' + keys, {cache: 'no-store'});
  const properties = await res.json();
  renderSelectedProperties(properties || []);
  renderHealth(properties || []);
}
async function refreshAllProperties() {
  const res = await fetch('/api/properties?limit=1', {cache: 'no-store'});
  const properties = await res.json();
  renderProperties(properties || []);
}
async function refreshStats() {
  const res = await fetch('/api/property-stats?limit=1440', {cache: 'no-store'});
  const stats = await res.json();
  renderStats(stats || []);
}
function renderSelectedProperties(properties) {
  const selectedProps = properties.filter(p => selected.has(keyOf(p)));
  $('selectedProps').innerHTML = selectedProps.map(p => '<div class="card"><div class="label">' + escapeHTML(labelOf(p)) + '</div><div class="prop-value">' + escapeHTML(displayValue(p)) + '</div><div class="prop-key">' + escapeHTML(keyOf(p)) + '</div></div>').join('');
}
function renderHealth(properties) {
  const byKey = new Map(properties.map(p => [keyOf(p), p]));
  $('healthGrid').innerHTML = healthDevices.map(device => {
    const status = byKey.get(device.eoj + ':80');
    const errorState = byKey.get(device.eoj + ':88');
    const errorCode = byKey.get(device.eoj + ':86');
    return '<div class="health-card"><div class="health-title">' + escapeHTML(device.name) + '</div>' +
      '<div class="health-row"><span>動作状態</span><span class="health-value">' + statusHTML(status) + '</span></div>' +
      '<div class="health-row"><span>エラー</span><span class="health-value ' + escapeHTML(errorClass(errorState)) + '">' + escapeHTML(errorDisplay(errorState, errorCode)) + '</span></div>' +
      '<div class="health-row"><span>異常コード</span><span class="health-value">' + escapeHTML(errorCodeDisplay(errorState, errorCode)) + '</span></div></div>';
  }).join('');
}
function statusHTML(prop) {
  const value = (prop && (prop.description || '')).toUpperCase();
  if (value === 'ON') return '<span class="state-icon state-on">○</span>';
  if (value === 'OFF') return '<span class="state-icon state-off">×</span>';
  return '<span class="state-icon state-unknown">--</span>';
}
function hasError(prop) {
  if (!prop) return false;
  if (prop.raw === '41') return true;
  if (prop.raw === '42') return false;
  return prop.description && prop.description !== '異常なし';
}
function errorDisplay(errorState, errorCode) {
  if (!hasError(errorState)) return '--';
  return errorCodeDisplay(errorState, errorCode);
}
function errorCodeDisplay(errorState, errorCode) {
  if (!hasError(errorState)) return '--';
  if (!errorCode) return '異常あり';
  return errorCode.description || (errorCode.raw ? '0x' + errorCode.raw : '異常あり');
}
function errorClass(errorState) {
  return hasError(errorState) ? 'error-bad' : 'error-ok';
}
function renderProperties(properties) {
  renderSelectedProperties(properties);
  const groups = groupByDevice(properties);
  $('propertyChooser').innerHTML = groups.map(group => '<div class="device-group"><div class="device-title">' + escapeHTML(deviceName(group.eoj)) + ' <small>' + escapeHTML(group.eoj) + '</small></div><div class="choice-grid">' + group.items.map(p => {
    const key = keyOf(p);
    return '<label class="choice"><input type="checkbox" data-key="' + escapeHTML(key) + '" ' + (selected.has(key) ? 'checked' : '') + '><span>' + escapeHTML(labelOf(p)) + '<small>' + escapeHTML(p.epc) + ' ' + escapeHTML(displayValue(p)) + '</small></span></label>';
  }).join('') + '</div></div>').join('');
  $('propertyChooser').querySelectorAll('input').forEach(input => {
    input.addEventListener('change', () => {
      if (input.checked) selected.add(input.dataset.key); else selected.delete(input.dataset.key);
      saveSelected();
      renderProperties(properties);
      refreshSelectedProperties();
    });
  });
}
function renderStats(stats) {
  const rows = stats.slice(0, 80).map(stat => {
    const label = stat.name || deviceName(stat.eoj);
    const latest = stat.latest_description || fmtStat(stat.latest);
    return '<tr><td>' + escapeHTML(deviceName(stat.eoj)) + '</td><td>' + escapeHTML(stat.epc) + '<br><small>' + escapeHTML(label) + '</small></td><td>' + escapeHTML(latest) + '</td><td>' + escapeHTML(fmtStat(stat.min)) + '</td><td>' + escapeHTML(fmtStat(stat.max)) + '</td><td>' + escapeHTML(fmtStat(stat.delta)) + '</td><td>' + escapeHTML(fmtStat(stat.range)) + '</td><td>' + escapeHTML(stat.count) + '</td></tr>';
  }).join('');
  $('propertyStats').innerHTML = rows || '<tr><td colspan="8">データなし</td></tr>';
}
function groupByDevice(properties) {
  const map = new Map();
  properties.forEach(p => {
    if (!map.has(p.eoj)) map.set(p.eoj, []);
    map.get(p.eoj).push(p);
  });
  return [...map.entries()].map(([eoj, items]) => ({eoj, items}));
}
function escapeHTML(value) {
  return String(value).replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
}
function draw(samples) {
  const canvas = $('chart');
  const dpr = window.devicePixelRatio || 1;
  const rect = canvas.getBoundingClientRect();
  canvas.width = rect.width * dpr;
  canvas.height = rect.height * dpr;
  const ctx = canvas.getContext('2d');
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, rect.width, rect.height);
  const pad = {left: 44, right: 18, top: 24, bottom: 44};
  const values = samples.flatMap(s => [s.pv_watts, s.load_watts, has(s.grid_watts) ? Math.abs(s.grid_watts) : null]).filter(has);
  const max = Math.max(1000, ...values);
  ctx.strokeStyle = '#d8ddd0';
  ctx.lineWidth = 1;
  for (let i = 0; i <= 4; i++) {
    const y = pad.top + (rect.height - pad.top - pad.bottom) * i / 4;
    ctx.beginPath(); ctx.moveTo(pad.left, y); ctx.lineTo(rect.width - pad.right, y); ctx.stroke();
  }
  drawTimeAxis(ctx, samples, rect, pad);
  line(samples, 'pv_watts', '#d19b1d', max, rect, pad);
  line(samples, 'load_watts', '#2f6f73', max, rect, pad);
  line(samples, 'grid_watts', '#8357a4', max, rect, pad, true);
}
function drawTimeAxis(ctx, samples, rect, pad) {
  if (samples.length === 0) return;
  const plotWidth = rect.width - pad.left - pad.right;
  const y = rect.height - pad.bottom;
  ctx.fillStyle = '#62675e';
  ctx.strokeStyle = '#d8ddd0';
  ctx.lineWidth = 1;
  ctx.font = '12px system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';
  const ticks = Math.min(5, samples.length);
  for (let i = 0; i < ticks; i++) {
    const index = ticks === 1 ? 0 : Math.round((samples.length - 1) * i / (ticks - 1));
    const x = pad.left + plotWidth * index / Math.max(1, samples.length - 1);
    const label = new Date(samples[index].time).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
    ctx.beginPath(); ctx.moveTo(x, y); ctx.lineTo(x, y + 5); ctx.stroke();
    ctx.fillText(label, x, y + 8);
  }
}
function line(samples, key, color, max, rect, pad, abs) {
  if (samples.length < 2) return;
  const ctx = $('chart').getContext('2d');
  ctx.strokeStyle = color;
  ctx.lineWidth = 3;
  ctx.beginPath();
  let started = false;
  samples.forEach((s, i) => {
    const x = pad.left + (rect.width - pad.left - pad.right) * i / (samples.length - 1);
    const raw = s[key];
    if (!has(raw)) return;
    const val = abs ? Math.abs(raw) : raw;
    const y = rect.height - pad.bottom - (rect.height - pad.top - pad.bottom) * val / max;
    if (!started) { ctx.moveTo(x, y); started = true; } else { ctx.lineTo(x, y); }
  });
  ctx.stroke();
}
$('settingsButton').addEventListener('click', () => {
  settingsOpen = !settingsOpen;
  localStorage.setItem(settingsOpenKey, String(settingsOpen));
  applySettingsVisibility();
  if (settingsOpen) {
    refreshAllProperties();
    refreshStats();
  }
});
applySettingsVisibility();
refresh();
setInterval(refresh, 30000);
</script>
</body>
</html>`
