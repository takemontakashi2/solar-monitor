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

func TestStoreLatestSummariesKeepsECHONETZeroPV(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "solar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	err = store.Append(Sample{
		Time:     time.Date(2026, 10, 3, 20, 11, 0, 0, time.UTC),
		PVWatts:  floatPtr(0),
		TodayKWh: floatPtr(22.1),
		Source:   "echonet",
	})
	if err != nil {
		t.Fatal(err)
	}

	samples, err := store.LatestSummaries(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
	if samples[0].PVWatts == nil || *samples[0].PVWatts != 0 {
		t.Fatalf("PVWatts = %v, want 0", samples[0].PVWatts)
	}
	if samples[0].LoadWatts != nil {
		t.Fatalf("LoadWatts = %v, want nil", samples[0].LoadWatts)
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

func TestStoreLatestPropertiesFiltersKeys(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "solar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	err = store.Append(Sample{
		Time:   time.Date(2026, 10, 3, 20, 11, 0, 0, time.UTC),
		Source: "echonet",
		Properties: []SampleProperty{
			{EOJ: "027901", EPC: "e0", Name: "発電", Raw: "0000", Float: floatPtr(0)},
			{EOJ: "027d01", EPC: "80", Name: "動作状態", Raw: "30", Float: floatPtr(48), Description: "ON"},
			{EOJ: "05ff01", EPC: "88", Name: "異常発生状態", Raw: "42", Float: floatPtr(66), Description: "異常なし"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	props, err := store.LatestProperties(1, map[string]struct{}{
		"027d01:80": {},
		"05ff01:88": {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 2 {
		t.Fatalf("got %d properties, want 2: %#v", len(props), props)
	}
	if props[0].EOJ != "027d01" || props[0].EPC != "80" || props[1].EOJ != "05ff01" || props[1].EPC != "88" {
		t.Fatalf("unexpected properties: %#v", props)
	}
}
