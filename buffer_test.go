package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func bufferedTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "solar.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.EnableBuffering()
	return s
}
func diskCount(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM samples").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestBufferedReadFlushAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "solar.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	sample := Sample{Time: at, Source: "test", PVWatts: floatPtr(0), GridWatts: floatPtr(-100), Properties: []SampleProperty{{EOJ: "027d01", EPC: "e4", Float: floatPtr(73), Raw: "49"}}, BatteryPercent: floatPtr(73)}
	if err := s.Append(sample); err != nil {
		t.Fatal(err)
	}
	s.EnableBuffering()
	sample.Time = at.Add(time.Minute)
	sample.PVWatts = nil
	if err := s.Append(sample); err != nil {
		t.Fatal(err)
	}
	*sample.Properties[0].Float = 99 // queued data must be owned by Store
	if diskCount(t, s) != 1 {
		t.Fatal("wrote before flush")
	}
	rows, err := s.SummariesBetween(at, at.Add(time.Hour))
	if err != nil || len(rows) != 2 {
		t.Fatalf("range: %v %v", rows, err)
	}
	if rows[0].PVWatts == nil || *rows[0].PVWatts != 0 || rows[1].PVWatts != nil || *rows[1].BatteryPercent != 73 {
		t.Fatal("missing/zero/battery changed")
	}
	props, err := s.LatestProperties(1, map[string]struct{}{"027d01:e4": {}})
	if err != nil || len(props) != 1 || *props[0].Float != 73 || !props[0].Time.Equal(sample.Time) {
		t.Fatalf("props: %v %v", props, err)
	}
	props, err = s.LatestProperties(2, nil)
	if err != nil || len(props) != 2 {
		t.Fatalf("history props: %v %v", props, err)
	}
	stats, err := s.PropertyStats(2)
	if err != nil || len(stats) != 1 || stats[0].Count != 2 {
		t.Fatalf("stats: %v %v", stats, err)
	}
	latest, err := s.LatestSummaries(1)
	if err != nil || len(latest) != 1 || !latest[0].Time.Equal(sample.Time) {
		t.Fatal("limit not applied to merged data")
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if diskCount(t, s) != 2 {
		t.Fatal("missing or duplicated writes")
	}
	reopened, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err := reopened.LatestSummaries(2)
	if err != nil || len(saved) != 2 || *saved[1].BatteryPercent != 73 || saved[1].PVWatts != nil {
		t.Fatalf("reopen: %v %v", saved, err)
	}
}
func TestFailedFlushRollsBackAndRetainsBatch(t *testing.T) {
	s := bufferedTestStore(t)
	for i := 0; i < 3; i++ {
		if err := s.Append(Sample{Time: time.Now().Add(time.Duration(i) * time.Minute), PVWatts: floatPtr(float64(i))}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := s.db.Exec(`CREATE TRIGGER fail_batch BEFORE INSERT ON samples WHEN NEW.pv_watts=1 BEGIN SELECT RAISE(ABORT,'simulated failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if s.Flush() == nil {
		t.Fatal("expected write failure")
	}
	if diskCount(t, s) != 0 || len(s.pending) != 3 {
		t.Fatal("partial commit or lost batch")
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_batch"); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if diskCount(t, s) != 3 || len(s.pending) != 0 {
		t.Fatal("retry duplicated/lost data")
	}
}
func TestBufferCapacityFlushAndFailure(t *testing.T) {
	s := bufferedTestStore(t)
	sample := Sample{Time: time.Now()}
	raw, _ := json.Marshal(sample)
	s.maxPendingBytes = len(raw) * 2
	if err := s.Append(sample); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(sample); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(sample); err != nil {
		t.Fatal(err)
	}
	if diskCount(t, s) != 2 || len(s.pending) != 1 {
		t.Fatal("capacity did not flush early")
	}
	if err := s.Append(sample); err != nil {
		t.Fatal(err)
	}
	s.db.Exec(`CREATE TRIGGER fail_all BEFORE INSERT ON samples BEGIN SELECT RAISE(ABORT,'failure'); END`)
	if s.Append(sample) == nil {
		t.Fatal("expected capacity error")
	}
	if len(s.pending) != 2 || s.pendingBytes > s.maxPendingBytes {
		t.Fatal("buffer exceeded bound or lost data")
	}
}
func TestBufferedMetadataAndUnchangedRejection(t *testing.T) {
	s := bufferedTestStore(t)
	at := time.Now()
	props := []SampleProperty{{EOJ: "027d01", EPC: "d0", Raw: "01"}}
	if err := s.Append(Sample{Time: at, Properties: props}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordFixedReads("device", props); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRejectedFixed("device", map[string]bool{"027d01:d1": true}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.FixedReads("device")
	if len(before) != 0 {
		t.Fatal("metadata written early")
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	after, _ := s.FixedReads("device")
	rejected, _ := s.RejectedFixed("device")
	if !after["027d01:d0"] || !rejected["027d01:d1"] {
		t.Fatal("metadata not persisted")
	}
	var changesBefore, changesAfter int
	s.db.QueryRow("SELECT total_changes()").Scan(&changesBefore)
	s.SaveRejectedFixed("device", rejected)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	s.db.QueryRow("SELECT total_changes()").Scan(&changesAfter)
	if changesBefore != changesAfter {
		t.Fatal("unchanged rejection was rewritten")
	}
	s.SaveRejectedFixed("device", map[string]bool{})
	s.Flush()
	rejected, _ = s.RejectedFixed("device")
	if len(rejected) != 0 {
		t.Fatal("cleared rejection retained")
	}
}
func TestBufferedAPIsAndDayAvailability(t *testing.T) {
	s := bufferedTestStore(t)
	at := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	a := NewApp(s, staticSource{Sample{Time: at, PVWatts: floatPtr(10), Properties: []SampleProperty{{EOJ: "027901", EPC: "e0", Raw: "0a", Float: floatPtr(10)}}}})
	a.CollectOnce(context.Background())
	for _, test := range []struct{ url, want string }{
		{"/api/samples?from=2026-10-28T00:00:00Z&to=2026-10-29T00:00:00Z&exists=1", "{\"exists\":true}\n"},
		{"/api/samples?from=2026-10-27T00:00:00Z&to=2026-10-28T00:00:00Z&exists=1", "{\"exists\":false}\n"},
	} {
		r := httptest.NewRecorder()
		a.Routes().ServeHTTP(r, httptest.NewRequest("GET", test.url, nil))
		if r.Code != 200 || r.Body.String() != test.want {
			t.Fatalf("%s: %s", test.url, r.Body.String())
		}
	}
	for _, url := range []string{"/api/samples?limit=1", "/api/properties?limit=1", "/api/property-stats?limit=1"} {
		r := httptest.NewRecorder()
		a.Routes().ServeHTTP(r, httptest.NewRequest("GET", url, nil))
		if r.Code != 200 || r.Body.String() == "[]\n" {
			t.Fatalf("%s: %s", url, r.Body.String())
		}
	}
	if diskCount(t, s) != 0 {
		t.Fatal("API access wrote data")
	}
}
func TestConcurrentBufferReadsAndFlush(t *testing.T) {
	s := bufferedTestStore(t)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			if err := s.Append(Sample{Time: time.Now()}); err != nil {
				t.Error(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			if _, err := s.LatestSummaries(10); err != nil {
				t.Error(err)
			}
			s.LatestProperties(1, nil)
			s.PropertyStats(10)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			if err := s.Flush(); err != nil {
				t.Error(err)
			}
		}
	}()
	wg.Wait()
	s.Flush()
	if diskCount(t, s) != 30 {
		t.Fatal("lost or duplicated concurrent data")
	}
}
