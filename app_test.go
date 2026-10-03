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

func TestAppPropertiesAPIFiltersKeys(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "solar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	app := NewApp(store, staticSource{sample: Sample{
		Time:   time.Date(2026, 10, 3, 10, 55, 0, 0, time.UTC),
		Source: "echonet",
		Properties: []SampleProperty{
			{EOJ: "027901", EPC: "e0", Name: "発電", Raw: "000013e2", Float: floatPtr(5090)},
			{EOJ: "027d01", EPC: "eb", Name: "充電電力", Raw: "00000ca8", Float: floatPtr(3240)},
		},
	}})
	app.CollectOnce(context.Background())

	sampleReq := httptest.NewRequest(http.MethodGet, "/api/samples?limit=1", nil)
	sampleRes := httptest.NewRecorder()
	app.Routes().ServeHTTP(sampleRes, sampleReq)
	if sampleRes.Code != http.StatusOK {
		t.Fatalf("samples status %d", sampleRes.Code)
	}
	var samples []Sample
	if err := json.NewDecoder(sampleRes.Body).Decode(&samples); err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
	if len(samples[0].Properties) != 0 {
		t.Fatalf("samples API returned properties: %#v", samples[0].Properties)
	}

	propReq := httptest.NewRequest(http.MethodGet, "/api/properties?limit=1&keys=027d01:eb", nil)
	propRes := httptest.NewRecorder()
	app.Routes().ServeHTTP(propRes, propReq)
	if propRes.Code != http.StatusOK {
		t.Fatalf("properties status %d", propRes.Code)
	}
	var props []SampleProperty
	if err := json.NewDecoder(propRes.Body).Decode(&props); err != nil {
		t.Fatal(err)
	}
	if len(props) != 1 || props[0].EOJ != "027d01" || props[0].EPC != "eb" {
		t.Fatalf("unexpected properties: %#v", props)
	}
}
