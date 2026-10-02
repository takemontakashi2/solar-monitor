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
    .time { color: #62675e; font-size: 14px; }
    .grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 18px; }
    .card { background: #ffffff; border: 1px solid #dfe3d8; border-radius: 8px; padding: 16px; min-height: 100px; }
    .label { color: #62675e; font-size: 13px; }
    .value { font-size: clamp(26px, 4vw, 42px); font-weight: 750; margin-top: 8px; line-height: 1; }
    .unit { font-size: 16px; color: #62675e; margin-left: 4px; }
    .chart { background: #ffffff; border: 1px solid #dfe3d8; border-radius: 8px; padding: 12px; }
    canvas { width: 100%; height: 360px; display: block; }
    @media (max-width: 760px) { main { padding: 14px; } header { align-items: start; flex-direction: column; } .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } canvas { height: 300px; } }
  </style>
</head>
<body>
<main>
  <header>
    <h1>Solar Monitor</h1>
    <div class="time" id="updated">--</div>
  </header>
  <section class="grid">
    <div class="card"><div class="label">発電</div><div class="value"><span id="pv">--</span><span class="unit">kW</span></div></div>
    <div class="card"><div class="label">消費</div><div class="value"><span id="load">--</span><span class="unit">kW</span></div></div>
    <div class="card"><div class="label">買電 / 売電</div><div class="value"><span id="grid">--</span><span class="unit">kW</span></div></div>
    <div class="card"><div class="label">今日の発電</div><div class="value"><span id="today">--</span><span class="unit">kWh</span></div></div>
  </section>
  <section class="chart"><canvas id="chart"></canvas></section>
</main>
<script>
const $ = (id) => document.getElementById(id);
const has = (v) => typeof v === 'number' && Number.isFinite(v);
const fmtKW = (w) => has(w) ? (w / 1000).toFixed(2) : '--';
const fmtKWh = (v) => has(v) ? v.toFixed(1) : '--';
async function refresh() {
  const res = await fetch('/api/samples?limit=288', {cache: 'no-store'});
  const samples = await res.json();
  const latest = samples[samples.length - 1];
  if (!latest) return;
  $('pv').textContent = fmtKW(latest.pv_watts);
  $('load').textContent = fmtKW(latest.load_watts);
  $('grid').textContent = fmtKW(Math.abs(latest.grid_watts));
  $('today').textContent = latest.today_kwh.toFixed(1);
  $('updated').textContent = new Date(latest.time).toLocaleString();
  draw(samples);
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
  const pad = 36;
  const values = samples.flatMap(s => [s.pv_watts, s.load_watts, has(s.grid_watts) ? Math.abs(s.grid_watts) : null]).filter(has);
  const max = Math.max(1000, ...values);
  ctx.strokeStyle = '#d8ddd0';
  ctx.lineWidth = 1;
  for (let i = 0; i <= 4; i++) {
    const y = pad + (rect.height - pad * 2) * i / 4;
    ctx.beginPath(); ctx.moveTo(pad, y); ctx.lineTo(rect.width - pad, y); ctx.stroke();
  }
  line(samples, 'pv_watts', '#d19b1d', max, rect, pad);
  line(samples, 'load_watts', '#2f6f73', max, rect, pad);
  line(samples, 'grid_watts', '#8357a4', max, rect, pad, true);
}
function line(samples, key, color, max, rect, pad, abs) {
  if (samples.length < 2) return;
  const ctx = $('chart').getContext('2d');
  ctx.strokeStyle = color;
  ctx.lineWidth = 3;
  ctx.beginPath();
  samples.forEach((s, i) => {
    const x = pad + (rect.width - pad * 2) * i / (samples.length - 1);
    const raw = s[key];
    if (!has(raw)) return;
    const val = abs ? Math.abs(raw) : raw;
    const y = rect.height - pad - (rect.height - pad * 2) * val / max;
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  });
  ctx.stroke();
}
refresh();
setInterval(refresh, 30000);
</script>
</body>
</html>`
