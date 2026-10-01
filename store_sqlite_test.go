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
			PVWatts:   float64(1000 + i),
			LoadWatts: float64(500 + i),
			GridWatts: float64(-100 - i),
			TodayKWh:  float64(10 + i),
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
	if samples[0].PVWatts != 1001 || samples[1].PVWatts != 1002 {
		t.Fatalf("samples not returned oldest-to-newest within limit: %#v", samples)
	}
}
