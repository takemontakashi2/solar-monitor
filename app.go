package main

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
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

func (a *App) handleProperties(w http.ResponseWriter, r *http.Request) {
	limit := 1
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	samples, err := a.store.Latest(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := []SampleProperty{}
	for _, sample := range samples {
		props = append(props, sample.Properties...)
	}
	writeJSON(w, props)
}

func (a *App) handlePropertyStats(w http.ResponseWriter, r *http.Request) {
	limit := 1440
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 10000 {
			limit = n
		}
	}
	stats, err := a.store.PropertyStats(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var indexTemplate = template.Must(template.New("index").Parse(indexHTML))
