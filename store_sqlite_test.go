package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAppendAndLatest(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "solar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		err := store.Append(Sample{
			Time:      base.Add(time.Duration(i) * time.Minute),
			PVWatts:   floatPtr(float64(1000 + i)),
			LoadWatts: floatPtr(float64(500 + i)),
			GridWatts: floatPtr(float64(-100 - i)),
			TodayKWh:  floatPtr(float64(10 + i)),
			Source:    "test",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	samples, err := store.Latest(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 {
		t.Fatalf("got %d samples, want 2", len(samples))
	}
	if samples[0].PVWatts == nil || samples[1].PVWatts == nil || *samples[0].PVWatts != 1001 || *samples[1].PVWatts != 1002 {
		t.Fatalf("samples not returned oldest-to-newest within limit: %#v", samples)
	}
}

func TestStorePropertyStats(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "solar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	base := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	values := []float64{100, 250, 150}
	for i, value := range values {
		err := store.Append(Sample{
			Time:   base.Add(time.Duration(i) * time.Minute),
			Source: "echonet",
			Properties: []SampleProperty{
				{
					Time:  base.Add(time.Duration(i) * time.Minute),
					EOJ:   "05ff01",
					EPC:   "e7",
					Name:  "候補",
					Raw:   "00000000",
					Float: floatPtr(value),
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	stats, err := store.PropertyStats(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("got %d stats, want 1", len(stats))
	}
	stat := stats[0]
	if stat.Count != 3 {
		t.Fatalf("got count %d, want 3", stat.Count)
	}
	if stat.Latest == nil || *stat.Latest != 150 {
		t.Fatalf("got latest %v, want 150", stat.Latest)
	}
	if stat.Min == nil || *stat.Min != 100 {
		t.Fatalf("got min %v, want 100", stat.Min)
	}
	if stat.Max == nil || *stat.Max != 250 {
		t.Fatalf("got max %v, want 250", stat.Max)
	}
	if stat.Delta == nil || *stat.Delta != 50 {
		t.Fatalf("got delta %v, want 50", stat.Delta)
	}
	if stat.Range == nil || *stat.Range != 150 {
		t.Fatalf("got range %v, want 150", stat.Range)
	}
}
