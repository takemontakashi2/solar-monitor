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

func TestSummaryDistinguishesMissingAndZero(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "solar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	base := time.Now()
	for i, v := range []*float64{nil, floatPtr(0), floatPtr(123)} {
		if err := store.Append(Sample{Time: base.Add(time.Duration(i) * time.Minute), Source: "echonet", PVWatts: v, LoadWatts: v, GridWatts: v, TodayKWh: v}); err != nil {
			t.Fatal(err)
		}
	}
	samples, err := store.LatestSummaries(3)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range samples {
		for _, v := range []*float64{s.PVWatts, s.LoadWatts, s.GridWatts, s.TodayKWh} {
			if i == 0 {
				if v != nil {
					t.Fatal("missing value became zero")
				}
			} else if v == nil || *v != []float64{0, 0, 123}[i] {
				t.Fatalf("wrong value at %d: %v", i, v)
			}
		}
	}
}

func TestLatestPropertySurvivesMissingCollectionAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "solar.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	for i := 0; i < 3; i++ {
		s := Sample{Time: base.Add(time.Duration(i) * time.Minute), Source: "echonet"}
		if i < 2 {
			s.Properties = []SampleProperty{{EOJ: "027d01", EPC: "e4", Float: floatPtr(float64(50 + i))}}
		}
		if err := store.Append(s); err != nil {
			t.Fatal(err)
		}
	}
	store.Close()
	store, err = NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	props, err := store.LatestProperties(1, map[string]struct{}{"027d01:e4": {}})
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 1 || props[0].Float == nil || *props[0].Float != 51 || !props[0].Time.Equal(base.Add(time.Minute)) {
		t.Fatalf("lost last successful reading: %+v", props)
	}
}

func TestLegacyDatabaseMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "solar.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(Sample{Time: time.Now(), Source: "echonet", Properties: []SampleProperty{{EOJ: "027901", EPC: "e0", Float: floatPtr(0)}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`ALTER TABLE samples DROP COLUMN valid_fields`); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	samples, err := store.LatestSummaries(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].PVWatts == nil || *samples[0].PVWatts != 0 || samples[0].TodayKWh != nil {
		t.Fatalf("legacy missing/zero reconstruction failed: %+v", samples)
	}
}

func TestFixedReadsPersistPerTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixed.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	props := []SampleProperty{{EOJ: "027901", EPC: "8a", Raw: "000001"}, {EOJ: "027d01", EPC: "e4", Raw: "32"}, {EOJ: "027901", EPC: "83", Raw: ""}}
	if err := s.RecordFixedReads("192.168.100.202:3610", props); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cached, err := s.FixedReads("192.168.100.202:3610")
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) != 1 || !cached["027901:8a"] {
		t.Fatalf("wrong cache: %v", cached)
	}
	src := NewECHONETSource("192.168.100.202", nil, false)
	src.fixedRead = cached
	got := src.pendingProperties([3]byte{2, 0x79, 1}, []byte{0x8a, 0x83, 0xe0, 0x80})
	if containsByte(got, 0x8a) || !containsByte(got, 0x83) || !containsByte(got, 0xe0) || !containsByte(got, 0x80) {
		t.Fatalf("wrong requests: %x", got)
	}
	src.fullScan = true
	if !containsByte(src.pendingProperties([3]byte{2, 0x79, 1}, []byte{0x8a}), 0x8a) {
		t.Fatal("explicit refresh must include fixed values")
	}
	other, err := s.FixedReads("192.168.100.126:3610")
	if err != nil || len(other) != 0 {
		t.Fatalf("target cache leaked: %v %v", other, err)
	}
	for _, epc := range []string{"80", "81", "86", "88", "89", "97", "98", "cf", "da", "e4", "d3"} {
		if isFixedProperty("027d01", epc) {
			t.Fatalf("dynamic property classified fixed: %s", epc)
		}
	}
}

func TestRejectedFixedPersistenceAndReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rejected.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRejectedFixed("A", map[string]bool{"027d01:c7": true}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.RejectedFixed("A")
	if err != nil || !a["027d01:c7"] {
		t.Fatalf("lost rejection: %v %v", a, err)
	}
	b, err := s.RejectedFixed("B")
	if err != nil || len(b) != 0 {
		t.Fatalf("target cache leaked: %v %v", b, err)
	}
	if err := s.SaveRejectedFixed("A", nil); err != nil {
		t.Fatal(err)
	}
	a, err = s.RejectedFixed("A")
	if err != nil || len(a) != 0 {
		t.Fatalf("reset failed: %v %v", a, err)
	}
}

func TestBatteryHistoryKeepsMissingAndRange(t *testing.T) {
	s, err := NewStore(filepath.Join(t.TempDir(), "battery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	values := []*float64{floatPtr(0), floatPtr(73), nil, floatPtr(100), floatPtr(255)}
	for i, v := range values {
		sample := Sample{Time: time.Date(2026, 10, 21, 0, i, 0, 0, time.UTC), Source: "echonet"}
		if v != nil {
			sample.Properties = []SampleProperty{{EOJ: "027d01", EPC: "e4", Float: v}}
		}
		if err := s.Append(sample); err != nil {
			t.Fatal(err)
		}
	}
	for _, read := range []func(int) ([]Sample, error){s.LatestSummaries, s.Latest} {
		got, err := read(5)
		if err != nil {
			t.Fatal(err)
		}
		for i, p := range got {
			if i == 2 || i == 4 {
				if p.BatteryPercent != nil {
					t.Fatalf("missing/invalid converted to value at %d", i)
				}
			} else if p.BatteryPercent == nil || *p.BatteryPercent != *values[i] {
				t.Fatalf("bad battery at %d: %v", i, p.BatteryPercent)
			}
		}
	}
}
