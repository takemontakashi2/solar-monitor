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
    .health-panel { margin-bottom: 18px; }
    .health-panel summary { cursor: pointer; padding: 12px; border: 1px solid #dfe3d8; border-radius: 8px; background: #fff; }
    .health-panel[open] summary { margin-bottom: 12px; }
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
    .chart-legend { display: flex; flex-wrap: wrap; gap: 10px 18px; padding: 2px 6px 8px; color: #4f554c; font-size: 13px; }
    .legend-item { display: inline-flex; align-items: center; gap: 7px; font: inherit; color: inherit; background: transparent; border: 1px solid transparent; border-radius: 6px; padding: 8px; cursor: pointer; }
    .legend-item[aria-pressed="true"] { font-weight: 700; background: #eef2e8; border-color: #87977a; }
    .legend-item:focus-visible { outline: 2px solid #2f6f73; outline-offset: 2px; }
    .legend-line { width: 24px; height: 3px; border-radius: 2px; }
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
    <div class="card"><div class="label" id="gridLabel">買電 / 売電</div><div class="value"><span id="grid">--</span><span class="unit">kW</span></div></div>
    <div class="card"><div class="label">今日の発電</div><div class="value"><span id="today">--</span><span class="unit">kWh</span></div></div>
  </section>
  <section class="prop-grid" id="selectedProps"></section>
  <section class="chart">
    <div class="chart-legend" aria-label="グラフ凡例">
      <button type="button" class="legend-item" data-series="pv_watts" aria-pressed="false" title="クリックで強調、もう一度クリックで解除"><span class="legend-line" style="background:#d19b1d"></span>発電</button>
      <button type="button" class="legend-item" data-series="load_watts" aria-pressed="false" title="クリックで強調、もう一度クリックで解除"><span class="legend-line" style="background:#2f6f73"></span>消費</button>
      <button type="button" class="legend-item" data-series="grid_watts" aria-pressed="false" title="クリックで強調、もう一度クリックで解除"><span class="legend-line" style="background:#8357a4"></span>買電（＋）/ 売電（−）</button>
      <button type="button" class="legend-item" data-series="battery_percent" aria-pressed="false" title="クリックで強調、もう一度クリックで解除"><span class="legend-line" style="background:#2776c5"></span>電池残量（右軸 %）</button>
    </div>
    <div class="chart-legend">
      <button type="button" id="previousDay">← 前日</button>
      <input type="date" id="chartDate" aria-label="表示する日付">
      <button type="button" id="nextDay">翌日 →</button>
      <button type="button" id="todayButton">今日</button>
      <label><input type="checkbox" id="compareDay">前日のデータを点線で重ねる</label>
    </div>
    <div class="label" id="chartStatus" role="status"></div>
    <div class="label">上部のカードは最新の取得値、グラフは選択した日付を表示します。</div>
    <label class="label">グラフ表示 <select id="chartSmoothing"><option value="5">5分平均</option><option value="0">実測値</option></select></label>
    <canvas id="chart"></canvas>
    <div class="label">欠測を挟む5分以内の区間は破線で接続します。5分平均は取得できた値から計算します。</div>
  </section>
  <details class="health-panel" id="healthPanel" open>
    <summary id="healthSummary">機器の状態：確認中</summary>
    <section class="health-grid" id="healthGrid"></section>
  </details>
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
const smoothingKey = 'solar-monitor.chart-smoothing';
const savedSmoothing = localStorage.getItem(smoothingKey);
$('chartSmoothing').value = ['0', '5'].includes(savedSmoothing) ? savedSmoothing : '5';
let selected = new Set(JSON.parse(localStorage.getItem(storageKey) || 'null') || defaultSelected);
let chartSamples = [];
let comparisonSamples = [];
let chartStart = 0, chartEnd = 1;
let historyRequest = 0;
let historyController = null;
function localDate(date) {
  return date.getFullYear() + '-' + String(date.getMonth() + 1).padStart(2, '0') + '-' + String(date.getDate()).padStart(2, '0');
}
$('chartDate').value = localDate(new Date());
async function refreshChart() {
  const day = $('chartDate').value;
  if (!day) return;
  const start = new Date(day + 'T00:00:00');
  const end = new Date(start); end.setDate(end.getDate() + 1);
  const previous = new Date(start); previous.setDate(previous.getDate() - 1);
  const compare = $('compareDay').checked;
  const request = ++historyRequest;
  if (historyController) historyController.abort();
  const controller = new AbortController(); historyController = controller;
  const timeout = setTimeout(() => controller.abort(), 15000);
  chartStart = start.getTime(); chartEnd = end.getTime();
  chartSamples = []; comparisonSamples = []; draw([]);
  $('chartStatus').textContent = day + ' 読み込み中…';
  async function fetchRange(from, to) {
    const query = new URLSearchParams({from: from.toISOString(), to: to.toISOString()});
    const res = await fetch('/api/samples?' + query, {cache:'no-store',signal:controller.signal});
    if (!res.ok) throw new Error('HTTP ' + res.status);
    return res.json();
  }
  try {
    const [current, past] = await Promise.all([fetchRange(start,end),compare ? fetchRange(previous,start) : Promise.resolve([])]);
    if (request !== historyRequest) return;
    chartSamples = current;
    comparisonSamples = past.map(sample => {
      const date = new Date(sample.time); date.setDate(date.getDate() + 1);
      return {...sample,time:date.toISOString()};
    });
    $('chartStatus').textContent = day + '：' + (current.length ? current.length + '件' : '記録なし') + (compare ? ' ／ 比較 ' + localDate(previous) + '：' + (past.length ? past.length + '件（点線）' : '記録なし') : '');
    draw(chartSamples);
  } catch(error) {
    if (request === historyRequest) $('chartStatus').textContent = day + ' の読み出しに失敗しました。' + (error.name === 'AbortError' ? '時間をおいて再試行してください。' : error.message);
  } finally { clearTimeout(timeout); }
}
function shiftChartDay(offset) {
  const date = new Date($('chartDate').value + 'T00:00:00');
  date.setDate(date.getDate() + offset);
  $('chartDate').value = localDate(date); refreshChart();
}
$('previousDay').addEventListener('click', () => shiftChartDay(-1));
$('nextDay').addEventListener('click', () => shiftChartDay(1));
$('todayButton').addEventListener('click', () => { $('chartDate').value = localDate(new Date()); refreshChart(); });
$('chartDate').addEventListener('change', refreshChart);
$('compareDay').addEventListener('change', refreshChart);
let highlightedSeries = null;
let previousHealthState = null;
let samplesLoading = false;
const propertyCache = new Map();
let latestSampleTime = 0;
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
  refreshChart();
  refreshSelectedProperties();
  if (settingsOpen) {
    refreshAllProperties();
    refreshStats();
  }
  if (samplesLoading) return;
  samplesLoading = true;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 15000);
  try {
    const res = await fetch('/api/samples?limit=1', {cache: 'no-store', signal: controller.signal});
    if (!res.ok) throw new Error('HTTP ' + res.status + ': ' + await res.text());
    const samples = await res.json();
    const latest = samples[samples.length - 1];
    if (!latest) {
      $('updated').textContent = '記録がまだありません。機器からの取得待ちです。';
      return;
    }
    latestSampleTime = new Date(latest.time).getTime();
    $('pv').textContent = fmtKW(latest.pv_watts);
    $('load').textContent = fmtKW(latest.load_watts);
    $('gridLabel').textContent = !has(latest.grid_watts) ? '買電 / 売電' : latest.grid_watts > 0 ? '買電' : latest.grid_watts < 0 ? '売電' : '買電 / 売電（0）';
    $('grid').textContent = has(latest.grid_watts) ? fmtKW(Math.abs(latest.grid_watts)) : '--';
    $('today').textContent = fmtKWh(latest.today_kwh);
    $('updated').textContent = new Date(latest.time).toLocaleString();
  } catch (error) {
    $('updated').textContent = error.name === 'AbortError' ? '記録の読み出しが15秒以内に完了しませんでした。再試行します。' : '記録の読み出しに失敗: ' + error.message;
    console.error('samples refresh failed', error);
  } finally {
    clearTimeout(timeout);
    samplesLoading = false;
  }
}
async function refreshSelectedProperties() {
  renderSelectedProperties([]);
  const keysToFetch = [...new Set([...selected, ...healthKeys])];
  if (keysToFetch.length === 0) {
    renderSelectedProperties([]);
    renderHealth([]);
    return;
  }
  const keys = encodeURIComponent(keysToFetch.join(','));
  try {
    const res = await fetch('/api/properties?limit=1&keys=' + keys, {cache: 'no-store'});
    if (!res.ok) throw new Error('properties: HTTP ' + res.status);
    const properties = await res.json();
    renderSelectedProperties(properties || []);
    renderHealth(properties || []);
  } catch (error) {
    renderHealth([...propertyCache.values()]);
    console.warn('項目の更新に失敗しました。カードは保持します。', error);
  }
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
  properties.forEach(p => {
    const previous = propertyCache.get(keyOf(p));
    if (!previous || new Date(p.time) >= new Date(previous.time)) propertyCache.set(keyOf(p), p);
  });
  $('selectedProps').innerHTML = [...selected].map(key => {
    const p = propertyCache.get(key);
    const older = p && new Date(p.time).getTime() < latestSampleTime;
    const stale = p && Date.now() - new Date(p.time).getTime() > 5 * 60 * 1000;
    const note = p ? (p.fixed ? '保存値・' : stale ? '更新待ち・' : older ? '前回値・' : '') + new Date(p.time).toLocaleString() : '未取得';
    return '<div class="card"><div class="label">' + escapeHTML(p ? labelOf(p) : key) + '</div><div class="prop-value">' + escapeHTML(p ? displayValue(p) : '--') + '</div><div class="prop-key">' + escapeHTML(key) + '<br>' + escapeHTML(note) + '</div></div>';
  }).join('');
}
function renderHealth(properties) {
  const byKey = new Map(properties.map(p => [keyOf(p), p]));
  const states = healthDevices.map(device => {
    const status = byKey.get(device.eoj + ':80');
    const error = byKey.get(device.eoj + ':88');
    if (hasError(error)) return 'error';
    const fresh = p => p && Number.isFinite(new Date(p.time).getTime()) && Date.now() - new Date(p.time).getTime() <= 5 * 60000;
    if (!fresh(status) || !fresh(error) || !['30', '31'].includes(status.raw) || error.raw !== '42') return 'unknown';
    return 'normal';
  });
  const state = states.includes('error') ? 'error' : states.includes('unknown') ? 'unknown' : 'normal';
  const affected = healthDevices.filter((_, i) => states[i] !== 'normal').map(device => device.name).join('・');
  $('healthSummary').textContent = state === 'normal' ? '機器の状態：異常なし（クリックで詳細）' : state === 'error' ? '機器の状態：異常あり — ' + affected : '機器の状態：未取得・更新待ち — ' + affected;
  $('healthSummary').classList.toggle('error-bad', state === 'error');
  if (state !== previousHealthState || state !== 'normal') $('healthPanel').open = state !== 'normal';
  previousHealthState = state;
  $('healthGrid').innerHTML = healthDevices.map((device, index) => {
    const status = byKey.get(device.eoj + ':80');
    const errorState = byKey.get(device.eoj + ':88');
    const errorCode = byKey.get(device.eoj + ':86');
    return '<div class="health-card"><div class="health-title">' + escapeHTML(device.name) + '</div>' +
      (states[index] === 'unknown' ? '<div class="label">未取得または5分以上更新されていません</div>' : '') +
      '<div class="health-row"><span>動作状態</span><span class="health-value">' + statusHTML(status) + '</span></div>' +
      '<div class="health-row"><span>エラー</span><span class="health-value ' + escapeHTML(errorClass(errorState)) + '">' + escapeHTML(errorDisplay(errorState, errorCode)) + '</span></div>' +
      '<div class="health-row"><span>異常コード</span><span class="health-value">' + escapeHTML(errorCodeDisplay(errorState, errorCode)) + '</span></div></div>';
  }).join('');
}
function statusHTML(prop) {
  const value = ((prop && prop.description) || '').toUpperCase();
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
  const groups = groupByDevice(properties);
  $('propertyChooser').innerHTML = groups.map(group => '<div class="device-group"><div class="device-title">' + escapeHTML(deviceName(group.eoj)) + ' <small>' + escapeHTML(group.eoj) + '</small></div><div class="choice-grid">' + group.items.map(p => {
    const key = keyOf(p);
    return '<label class="choice"><input type="checkbox" data-key="' + escapeHTML(key) + '" ' + (selected.has(key) ? 'checked' : '') + '><span>' + escapeHTML(labelOf(p)) + '<small>' + escapeHTML(p.epc) + ' ' + escapeHTML(displayValue(p)) + '</small></span></label>';
  }).join('') + '</div></div>').join('');
  $('propertyChooser').querySelectorAll('input').forEach(input => {
    input.addEventListener('change', () => {
      if (input.checked) selected.add(input.dataset.key); else selected.delete(input.dataset.key);
      saveSelected();
      renderSelectedProperties(properties);
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
function smoothSamples(samples, minutes) {
  if (!minutes) return samples;
  return samples.map((sample, index) => {
    const result = {...sample};
    for (const key of ['pv_watts', 'load_watts', 'grid_watts']) {
      if (!has(sample[key])) continue;
      let sum = 0, count = 0;
      for (let j = index; j >= 0; j--) {
        if (new Date(sample.time) - new Date(samples[j].time) >= minutes * 60000) break;
        if (!has(samples[j][key])) continue;
        if (j < index && new Date(samples[j + 1].time) - new Date(samples[j].time) > minutes * 60000) break;
        sum += samples[j][key];
        count++;
      }
      result[key] = sum / count;
    }
    return result;
  });
}
function sampleX(samples, index, rect, pad) {
  const start = chartStart;
  const span = chartEnd - chartStart;
  return pad.left + (rect.width - pad.left - pad.right) * (span > 0 ? (new Date(samples[index].time).getTime() - start) / span : 0);
}
function draw(samples) {
  samples = smoothSamples(samples, Number($('chartSmoothing').value));
  const comparison = smoothSamples(comparisonSamples, Number($('chartSmoothing').value));
  const canvas = $('chart');
  const dpr = window.devicePixelRatio || 1;
  const rect = canvas.getBoundingClientRect();
  canvas.width = rect.width * dpr;
  canvas.height = rect.height * dpr;
  const ctx = canvas.getContext('2d');
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, rect.width, rect.height);
  const pad = {left: 58, right: 58, top: 24, bottom: 44};
  const values = samples.concat(comparison).flatMap(s => [s.pv_watts, s.load_watts, s.grid_watts]).filter(has);
  const lower = Math.min(0, ...values);
  const upper = Math.max(1000, ...values);
  const step = Math.ceil((upper - lower) / 4000) * 1000;
  const min = Math.floor(lower / step) * step;
  const max = Math.ceil(upper / step) * step;
  drawPowerAxis(ctx, min, max, rect, pad, step);
  drawBatteryAxis(ctx, min, max, rect, pad);
  drawTimeAxis(ctx, samples, rect, pad);
  const series = [['pv_watts', '#d19b1d'], ['load_watts', '#2f6f73'], ['grid_watts', '#8357a4'], ['battery_percent', '#2776c5']];
  series.sort((a, b) => Number(a[0] === highlightedSeries) - Number(b[0] === highlightedSeries));
  series.forEach(([key, color]) => line(comparison, key, color, key === 'battery_percent' ? 100 * min / max : min, key === 'battery_percent' ? 100 : max, rect, pad, true));
  series.forEach(([key, color]) => line(samples, key, color, key === 'battery_percent' ? 100 * min / max : min, key === 'battery_percent' ? 100 : max, rect, pad));
}
function powerY(value, min, max, rect, pad) {
  return rect.height - pad.bottom - (rect.height - pad.top - pad.bottom) * (value - min) / (max - min);
}
function drawPowerAxis(ctx, min, max, rect, pad, step) {
  ctx.fillStyle = '#62675e';
  ctx.font = '12px system-ui, sans-serif';
  ctx.textAlign = 'right';
  ctx.textBaseline = 'middle';
  const ticks = new Set([min, max, 0]);
  for (let value = Math.ceil(min / step) * step; value <= max; value += step) ticks.add(value);
  for (const value of [...ticks].sort((a, b) => a - b)) {
    const y = powerY(value, min, max, rect, pad);
    ctx.strokeStyle = value === 0 ? '#62675e' : '#d8ddd0';
    ctx.lineWidth = value === 0 ? 2 : 1;
    ctx.beginPath(); ctx.moveTo(pad.left, y); ctx.lineTo(rect.width - pad.right, y); ctx.stroke();
    ctx.fillText(String(value / 1000), pad.left - 8, y);
  }
  ctx.textAlign = 'left';
  ctx.textBaseline = 'top';
  ctx.fillText('kW', 6, 2);
}
function drawBatteryAxis(ctx, min, max, rect, pad) {
  const x = rect.width - pad.right;
  ctx.fillStyle = '#2776c5';
  ctx.strokeStyle = '#2776c5';
  ctx.lineWidth = 1;
  ctx.font = '12px system-ui, sans-serif';
  ctx.textAlign = 'left';
  ctx.textBaseline = 'middle';
  ctx.beginPath(); ctx.moveTo(x, pad.top); ctx.lineTo(x, powerY(0, min, max, rect, pad)); ctx.stroke();
  for (let value = 0; value <= 100; value += 20) {
    const y = powerY(value, 100 * min / max, 100, rect, pad);
    ctx.beginPath(); ctx.moveTo(x, y); ctx.lineTo(x + 5, y); ctx.stroke();
    ctx.fillText(String(value), x + 8, y);
  }
  ctx.textBaseline = 'top';
  ctx.fillText('%', x + 8, 2);
}
function drawTimeAxis(ctx, samples, rect, pad) {
  const y = rect.height - pad.bottom;
  ctx.fillStyle = '#62675e';
  ctx.strokeStyle = '#d8ddd0';
  ctx.lineWidth = 1;
  ctx.font = '12px system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';
  const start = chartStart;
  const end = chartEnd;
  const span = end - start;
  if (span <= 0) return;
  const target = span / Math.max(2, Math.floor((rect.width - pad.left - pad.right) / 100));
  const step = ([1, 2, 3, 6, 12, 24, 48, 168].find(hours => hours * 3600000 >= target) || 168) * 3600000;
  const tick = new Date(start);
  tick.setMinutes(0, 0, 0);
  if (tick.getTime() < start) tick.setHours(tick.getHours() + 1);
  for (let time = tick.getTime(); time <= end; time += step) {
    const x = pad.left + (rect.width - pad.left - pad.right) * (time - start) / span;
    const date = new Date(time);
    const label = date.toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
    ctx.beginPath(); ctx.moveTo(x, y); ctx.lineTo(x, y + 5); ctx.stroke();
    ctx.fillText(label, x, y + 8);
    if (date.getHours() === 0 || time === tick.getTime()) {
      ctx.fillText((date.getMonth() + 1) + '/' + date.getDate(), x, y + 23);
    }
  }
}
function line(samples, key, color, min, max, rect, pad, comparison = false) {
  const ctx = $('chart').getContext('2d');
  ctx.save();
  ctx.beginPath(); ctx.rect(pad.left, pad.top - 3, rect.width - pad.left - pad.right, rect.height - pad.top - pad.bottom + 6); ctx.clip();
  ctx.strokeStyle = color;
  ctx.fillStyle = color;
  ctx.globalAlpha = highlightedSeries && highlightedSeries !== key ? 0.25 : comparison ? 0.65 : 1;
  ctx.lineWidth = highlightedSeries === key ? 4 : 2;
  ctx.lineJoin = 'round';
  ctx.lineCap = 'round';
  let previous = null;
  samples.forEach((s, i) => {
    const raw = s[key];
    if (!has(raw)) return;
    const point = {x: sampleX(samples, i, rect, pad), y: powerY(raw, min, max, rect, pad), time: new Date(s.time).getTime(), index: i};
    if (previous && point.time - previous.time <= 5 * 60000) {
      // A dashed segment explicitly marks interpolation over missing readings.
      ctx.setLineDash(comparison ? [1, 5] : i - previous.index > 1 ? [4, 4] : []);
      ctx.beginPath(); ctx.moveTo(previous.x, previous.y); ctx.lineTo(point.x, point.y); ctx.stroke();
    } else {
      ctx.beginPath(); ctx.arc(point.x, point.y, highlightedSeries === key ? 3 : 1.5, 0, 2 * Math.PI); ctx.fill();
    }
    previous = point;
  });
  ctx.setLineDash([]);
  ctx.restore();
}
document.querySelectorAll('[data-series]').forEach(button => {
  button.addEventListener('click', () => {
    highlightedSeries = highlightedSeries === button.dataset.series ? null : button.dataset.series;
    document.querySelectorAll('[data-series]').forEach(item => item.setAttribute('aria-pressed', String(item.dataset.series === highlightedSeries)));
    draw(chartSamples);
  });
});
$('chartSmoothing').addEventListener('change', () => {
  localStorage.setItem(smoothingKey, $('chartSmoothing').value);
  draw(chartSamples);
});
window.addEventListener('resize', () => draw(chartSamples));
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
renderSelectedProperties([]);
renderHealth([]);
refresh();
setInterval(refresh, 30000);
</script>
</body>
</html>`
