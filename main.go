package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Sample struct {
	Time       time.Time `json:"time"`
	PVWatts    float64   `json:"pv_watts"`
	LoadWatts  float64   `json:"load_watts"`
	GridWatts  float64   `json:"grid_watts"`
	TodayKWh   float64   `json:"today_kwh"`
	Source     string    `json:"source"`
}

type Source interface {
	Read(ctx context.Context) (Sample, error)
}

type MockSource struct {
	start time.Time
	rand  *rand.Rand
}

func NewMockSource() *MockSource {
	return &MockSource{start: time.Now(), rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (m *MockSource) Read(_ context.Context) (Sample, error) {
	now := time.Now()
	hour := float64(now.Hour()) + float64(now.Minute())/60
	daylight := math.Sin((hour - 6) / 12 * math.Pi)
	if daylight < 0 {
		daylight = 0
	}
	pv := daylight*4200 + m.rand.Float64()*180
	load := 450 + m.rand.Float64()*900
	grid := load - pv
	today := daylight * 24
	return Sample{Time: now, PVWatts: pv, LoadWatts: load, GridWatts: grid, TodayKWh: today, Source: "mock"}, nil
}

type HTTPJSONSource struct {
	url    string
	client *http.Client
}

func NewHTTPJSONSource(url string) *HTTPJSONSource {
	return &HTTPJSONSource{url: url, client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *HTTPJSONSource) Read(ctx context.Context) (Sample, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return Sample{}, err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return Sample{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Sample{}, fmt.Errorf("source returned %s", res.Status)
	}

	var sample Sample
	if err := json.NewDecoder(res.Body).Decode(&sample); err != nil {
		return Sample{}, err
	}
	if sample.Time.IsZero() {
		sample.Time = time.Now()
	}
	if sample.Source == "" {
		sample.Source = "http-json"
	}
	return sample, nil
}

type ECHONETSource struct {
	addr   string
	fields map[string]ECHONETProperty
	tid    uint16
}

type ECHONETProperty struct {
	EOJ   [3]byte
	EPC   byte
	Scale float64
}

func NewECHONETSource(addr string, fields map[string]ECHONETProperty) *ECHONETSource {
	return &ECHONETSource{addr: net.JoinHostPort(addr, "3610"), fields: fields, tid: uint16(time.Now().UnixNano())}
}

func (s *ECHONETSource) Read(ctx context.Context) (Sample, error) {
	result := Sample{Time: time.Now(), Source: "echonet"}
	for name, prop := range s.fields {
		value, err := s.readProperty(ctx, prop, name == "grid")
		if err != nil {
			return Sample{}, fmt.Errorf("%s: %w", name, err)
		}
		switch name {
		case "pv":
			result.PVWatts = value
		case "load":
			result.LoadWatts = value
		case "grid":
			result.GridWatts = value
		case "today":
			result.TodayKWh = value
		}
	}
	return result, nil
}

func (s *ECHONETSource) readProperty(ctx context.Context, prop ECHONETProperty, signed bool) (float64, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp4", s.addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	s.tid++
	req := []byte{
		0x10, 0x81, byte(s.tid >> 8), byte(s.tid),
		0x05, 0xff, 0x01,
		prop.EOJ[0], prop.EOJ[1], prop.EOJ[2],
		0x62,
		0x01,
		prop.EPC, 0x00,
	}
	if _, err := conn.Write(req); err != nil {
		return 0, err
	}
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, err
	}
	data, err := echonetPropertyData(buf[:n], prop.EPC)
	if err != nil {
		return 0, err
	}
	if signed {
		return float64(signedBE(data)) * prop.Scale, nil
	}
	return float64(unsignedBE(data)) * prop.Scale, nil
}

func echonetPropertyData(packet []byte, epc byte) ([]byte, error) {
	if len(packet) < 14 || packet[0] != 0x10 || packet[1] != 0x81 {
		return nil, errors.New("invalid ECHONET Lite packet")
	}
	if packet[10] != 0x72 {
		return nil, fmt.Errorf("unexpected ESV 0x%02x", packet[10])
	}
	opc := int(packet[11])
	pos := 12
	for i := 0; i < opc; i++ {
		if pos+2 > len(packet) {
			return nil, errors.New("truncated property header")
		}
		gotEPC := packet[pos]
		pdc := int(packet[pos+1])
		pos += 2
		if pos+pdc > len(packet) {
			return nil, errors.New("truncated property data")
		}
		if gotEPC == epc {
			return packet[pos : pos+pdc], nil
		}
		pos += pdc
	}
	return nil, fmt.Errorf("EPC 0x%02x not found", epc)
}

func unsignedBE(data []byte) uint64 {
	var value uint64
	for _, b := range data {
		value = value<<8 | uint64(b)
	}
	return value
}

func signedBE(data []byte) int64 {
	if len(data) == 0 {
		return 0
	}
	value := int64(unsignedBE(data))
	bits := uint(len(data) * 8)
	sign := int64(1) << (bits - 1)
	if value&sign != 0 {
		value -= int64(1) << bits
	}
	return value
}

func parseECHONETFields(raw string) (map[string]ECHONETProperty, error) {
	fields := map[string]ECHONETProperty{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, rest, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("field %q must be name=EOJ:EPC[:scale]", item)
		}
		parts := strings.Split(rest, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("field %q must be name=EOJ:EPC[:scale]", item)
		}
		eoj, err := parseHexBytes(parts[0], 3)
		if err != nil {
			return nil, fmt.Errorf("%s EOJ: %w", name, err)
		}
		epc, err := parseHexBytes(parts[1], 1)
		if err != nil {
			return nil, fmt.Errorf("%s EPC: %w", name, err)
		}
		scale := 1.0
		if len(parts) == 3 {
			scale, err = strconv.ParseFloat(parts[2], 64)
			if err != nil {
				return nil, fmt.Errorf("%s scale: %w", name, err)
			}
		}
		fields[name] = ECHONETProperty{
			EOJ:   [3]byte{eoj[0], eoj[1], eoj[2]},
			EPC:   epc[0],
			Scale: scale,
		}
	}
	return fields, nil
}

func parseHexBytes(raw string, want int) ([]byte, error) {
	raw = strings.TrimPrefix(strings.ToLower(strings.ReplaceAll(raw, "0x", "")), "#")
	raw = strings.ReplaceAll(raw, "-", "")
	raw = strings.ReplaceAll(raw, " ", "")
	if len(raw) != want*2 {
		return nil, fmt.Errorf("want %d hex bytes", want)
	}
	out := make([]byte, want)
	for i := 0; i < want; i++ {
		n, err := strconv.ParseUint(raw[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		out[i] = byte(n)
	}
	return out, nil
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		w := csv.NewWriter(f)
		_ = w.Write([]string{"time", "pv_watts", "load_watts", "grid_watts", "today_kwh", "source"})
		w.Flush()
		if closeErr := f.Close(); closeErr != nil {
			return nil, closeErr
		}
		if err := w.Error(); err != nil {
			return nil, err
		}
	}
	return &Store{path: path}, nil
}

func (s *Store) Append(sample Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	err = w.Write([]string{
		sample.Time.Format(time.RFC3339),
		formatFloat(sample.PVWatts),
		formatFloat(sample.LoadWatts),
		formatFloat(sample.GridWatts),
		formatFloat(sample.TodayKWh),
		sample.Source,
	})
	w.Flush()
	if err != nil {
		return err
	}
	return w.Error()
}

func (s *Store) Latest(limit int) ([]Sample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, nil
	}
	data := rows[1:]
	if limit > 0 && len(data) > limit {
		data = data[len(data)-limit:]
	}
	out := make([]Sample, 0, len(data))
	for _, row := range data {
		sample, err := parseSample(row)
		if err != nil {
			continue
		}
		out = append(out, sample)
	}
	return out, nil
}

func parseSample(row []string) (Sample, error) {
	if len(row) < 6 {
		return Sample{}, errors.New("short row")
	}
	t, err := time.Parse(time.RFC3339, row[0])
	if err != nil {
		return Sample{}, err
	}
	return Sample{
		Time:      t,
		PVWatts:   mustFloat(row[1]),
		LoadWatts: mustFloat(row[2]),
		GridWatts: mustFloat(row[3]),
		TodayKWh:  mustFloat(row[4]),
		Source:    row[5],
	}, nil
}

func mustFloat(v string) float64 {
	f, _ := strconv.ParseFloat(v, 64)
	return f
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

type App struct {
	store  *Store
	source Source
	latest Sample
	mu     sync.RWMutex
}

func (a *App) runCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	a.collectOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.collectOnce(ctx)
		}
	}
}

func (a *App) collectOnce(ctx context.Context) {
	sample, err := a.source.Read(ctx)
	if err != nil {
		log.Printf("collect failed: %v", err)
		return
	}
	if err := a.store.Append(sample); err != nil {
		log.Printf("store failed: %v", err)
		return
	}
	a.mu.Lock()
	a.latest = sample
	a.mu.Unlock()
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/api/latest", a.handleLatest)
	mux.HandleFunc("/api/samples", a.handleSamples)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

func (a *App) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) handleLatest(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	sample := a.latest
	a.mu.RUnlock()
	writeJSON(w, sample)
}

func (a *App) handleSamples(w http.ResponseWriter, r *http.Request) {
	limit := 288
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 5000 {
			limit = n
		}
	}
	samples, err := a.store.Latest(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, samples)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	data := flag.String("data", "data/solar.csv", "CSV data file")
	interval := flag.Duration("interval", 60*time.Second, "collection interval")
	sourceKind := flag.String("source", "mock", "source type: mock, http-json, or echonet")
	sourceURL := flag.String("source-url", "", "HTTP JSON source URL")
	echonetAddr := flag.String("echonet-addr", "", "ECHONET Lite target IPv4 address")
	echonetFields := flag.String("echonet-fields", "pv=027901:e0,load=028701:e7,grid=028801:e7,today=027901:e1:0.001", "ECHONET fields: name=EOJ:EPC[:scale], comma-separated")
	flag.Parse()

	store, err := NewStore(*data)
	if err != nil {
		log.Fatal(err)
	}

	var source Source
	switch *sourceKind {
	case "mock":
		source = NewMockSource()
	case "http-json":
		if *sourceURL == "" {
			log.Fatal("-source-url is required for http-json")
		}
		source = NewHTTPJSONSource(*sourceURL)
	case "echonet":
		if *echonetAddr == "" {
			log.Fatal("-echonet-addr is required for echonet")
		}
		fields, err := parseECHONETFields(*echonetFields)
		if err != nil {
			log.Fatal(err)
		}
		source = NewECHONETSource(*echonetAddr, fields)
	default:
		log.Fatalf("unknown source %q", *sourceKind)
	}

	app := &App{store: store, source: source}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.runCollector(ctx, *interval)

	log.Printf("listening on http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, app.routes()))
}

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
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
const fmtKW = (w) => (w / 1000).toFixed(2);
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
  const max = Math.max(1000, ...samples.flatMap(s => [s.pv_watts, s.load_watts, Math.abs(s.grid_watts)]));
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
    const val = abs ? Math.abs(s[key]) : s[key];
    const y = rect.height - pad - (rect.height - pad * 2) * val / max;
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  });
  ctx.stroke();
}
refresh();
setInterval(refresh, 30000);
</script>
</body>
</html>`))
