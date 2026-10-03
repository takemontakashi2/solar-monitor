package main

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type App struct {
	store  *Store
	source Source
	latest Sample
	mu     sync.RWMutex
}

func NewApp(store *Store, source Source) *App {
	return &App{store: store, source: source}
}

func (a *App) RunCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	a.CollectOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.CollectOnce(ctx)
		}
	}
}

func (a *App) CollectOnce(ctx context.Context) {
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

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/api/latest", a.handleLatest)
	mux.HandleFunc("/api/samples", a.handleSamples)
	mux.HandleFunc("/api/properties", a.handleProperties)
	mux.HandleFunc("/api/property-stats", a.handlePropertyStats)
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
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 288, 1, 5000)
	samples, err := a.store.LatestSummaries(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, samples)
}

func (a *App) handleProperties(w http.ResponseWriter, r *http.Request) {
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 1, 1, 100)
	keys := parsePropertyKeys(r.URL.Query().Get("keys"))
	props, err := a.store.LatestProperties(limit, keys)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, props)
}

func parsePropertyKeys(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, item := range strings.Split(raw, ",") {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		out[key] = struct{}{}
	}
	return out
}

func (a *App) handlePropertyStats(w http.ResponseWriter, r *http.Request) {
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 1440, 1, 10000)
	stats, err := a.store.PropertyStats(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func parseBoundedInt(raw string, fallback, min, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return fallback
	}
	return n
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var indexTemplate = template.Must(template.New("index").Parse(indexHTML))
