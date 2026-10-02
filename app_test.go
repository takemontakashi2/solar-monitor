package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

type staticSource struct {
	sample Sample
}

func (s staticSource) Read(context.Context) (Sample, error) {
	return s.sample, nil
}

func TestAppSamplesAPI(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "solar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	app := NewApp(store, staticSource{sample: Sample{
		Time:      time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		PVWatts:   floatPtr(3200),
		LoadWatts: floatPtr(900),
		GridWatts: floatPtr(-2300),
		TodayKWh:  floatPtr(18.4),
		Source:    "test",
	}})
	app.CollectOnce(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/api/samples?limit=1", nil)
	res := httptest.NewRecorder()
	app.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d", res.Code)
	}
	var samples []Sample
	if err := json.NewDecoder(res.Body).Decode(&samples); err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].PVWatts == nil || *samples[0].PVWatts != 3200 {
		t.Fatalf("unexpected samples: %#v", samples)
	}
}
