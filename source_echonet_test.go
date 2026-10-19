package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseECHONETFields(t *testing.T) {
	fields, err := parseECHONETFields("pv=027901:e0,grid=028801:e7:-1,today=027901:e1:0.001")
	if err != nil {
		t.Fatal(err)
	}
	if fields["pv"].EOJ != [3]byte{0x02, 0x79, 0x01} || fields["pv"].EPC != 0xe0 {
		t.Fatalf("unexpected pv field: %#v", fields["pv"])
	}
	if fields["today"].Scale != 0.001 {
		t.Fatalf("unexpected scale: %v", fields["today"].Scale)
	}
}

func TestECHONETPropertyDataAndSignedBE(t *testing.T) {
	packet := []byte{0x10, 0x81, 0x00, 0x01, 0x02, 0x88, 0x01, 0x05, 0xff, 0x01, 0x72, 0x01, 0xe7, 0x02, 0xff, 0x38}
	data, err := echonetPropertyData(packet, 0xe7)
	if err != nil {
		t.Fatal(err)
	}
	if got := signedBE(data); got != -200 {
		t.Fatalf("got %d, want -200", got)
	}
}

func TestECHONETReadCandidatesIncludeSharpBatteryMeasurements(t *testing.T) {
	got := echonetReadCandidates([3]byte{0x02, 0x7d, 0x01}, false)
	for _, want := range []byte{0xd3, 0xd6, 0xd8, 0xe3, 0xe4, 0xcf} {
		if !containsByte(got, want) {
			t.Fatalf("battery candidates missing 0x%02x: %x", want, got)
		}
	}
}

func TestMakeSamplePropertyUsesSignedBatteryPower(t *testing.T) {
	prop := makeSampleProperty(time.Time{}, [3]byte{0x02, 0x7d, 0x01}, 0xd3, []byte{0xff, 0xff, 0xfc, 0x90})
	if prop.Float == nil || *prop.Float != -880 {
		t.Fatalf("Float = %v, want -880", prop.Float)
	}
	if prop.Description != "-880 W" {
		t.Fatalf("Description = %q, want -880 W", prop.Description)
	}
}

func TestMakeSamplePropertyScalesSharpControllerValues(t *testing.T) {
	sell := makeSampleProperty(time.Time{}, [3]byte{0x05, 0xff, 0x01}, 0xf2, []byte{0x00, 0x00, 0x04, 0xb0})
	if sell.Float == nil || *sell.Float != 120 {
		t.Fatalf("sell Float = %v, want 120", sell.Float)
	}
	if sell.Description != "120.0 W" {
		t.Fatalf("sell Description = %q, want 120.0 W", sell.Description)
	}
	buyTotal := makeSampleProperty(time.Time{}, [3]byte{0x05, 0xff, 0x01}, 0xf5, []byte{0x00, 0x01, 0xe2, 0x08})
	if buyTotal.Float == nil || *buyTotal.Float != 123.4 {
		t.Fatalf("buyTotal Float = %v, want 123.4", buyTotal.Float)
	}
	if buyTotal.Description != "123.400 kWh" {
		t.Fatalf("buyTotal Description = %q, want 123.400 kWh", buyTotal.Description)
	}
}

func TestDeriveECHONETSummaryUsesControllerGridAndLoad(t *testing.T) {
	sample := Sample{
		Source: "echonet",
		Properties: []SampleProperty{
			{EOJ: "027901", EPC: "e0", Float: floatPtr(2040)},
			{EOJ: "027d01", EPC: "d3", Float: floatPtr(500)},
			{EOJ: "05ff01", EPC: "f2", Float: floatPtr(120)},
			{EOJ: "05ff01", EPC: "f3", Float: floatPtr(0)},
		},
	}
	deriveECHONETSummary(&sample)
	if sample.GridWatts == nil || *sample.GridWatts != -120 {
		t.Fatalf("GridWatts = %v, want -120", sample.GridWatts)
	}
	if sample.LoadWatts == nil || *sample.LoadWatts != 1420 {
		t.Fatalf("LoadWatts = %v, want 1420", sample.LoadWatts)
	}
}

func containsByte(values []byte, want byte) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestCollectionDiagnostics(t *testing.T) {
	d := collectionDiagnostics{}
	eoj := [3]byte{2, 0x79, 1}
	d.record(eoj, []byte{0xe0}, map[byte][]byte{0xe0: {0, 0}}, nil)
	d.record(eoj, []byte{0xe0, 0xe1}, map[byte][]byte{0xe0: {1}}, nil)
	d.record(eoj, []byte{0xe0}, nil, fmt.Errorf("read: %w", os.ErrDeadlineExceeded))
	if d.Requests != 3 || d.OK != 1 || d.Failed != 2 || d.Timeouts != 1 {
		t.Fatalf("unexpected counters: %+v", d)
	}
	if strings.Join(d.Missing, ",") != "027901:e1(missing),027901:e0(timeout)" {
		t.Fatalf("wrong failures: %v", d.Missing)
	}
}

func TestRequestMatchingRejectsDelayedAndOtherResponses(t *testing.T) {
	eoj := [3]byte{2, 0x79, 1}
	packet := []byte{0x10, 0x81, 0x12, 0x34, 2, 0x79, 1, 5, 0xff, 1, 0x72, 0}
	if !matchesECHONETRequest(packet, 0x1234, eoj) {
		t.Fatal("valid response rejected")
	}
	for _, index := range []int{0, 2, 3, 4, 7, 10} {
		bad := append([]byte{}, packet...)
		bad[index] ^= 1
		if matchesECHONETRequest(bad, 0x1234, eoj) {
			t.Fatalf("accepted mismatch at %d", index)
		}
	}
	packet[10] = 0x52
	if !matchesECHONETRequest(packet, 0x1234, eoj) {
		t.Fatal("GET rejection must match request")
	}
}

func TestNormalRequestsExcludeFailingControllerAttributes(t *testing.T) {
	eoj := [3]byte{5, 0xff, 1}
	normal := echonetReadCandidates(eoj, false)
	for _, epc := range []byte{0x97, 0x98, 0x8c, 0xf1} {
		if containsByte(normal, epc) {
			t.Fatalf("unnecessary normal request: %x", epc)
		}
	}
	full := echonetReadCandidates(eoj, true)
	for _, epc := range []byte{0x97, 0x98, 0x8c, 0xf1} {
		if !containsByte(full, epc) {
			t.Fatalf("diagnostic missing: %x", epc)
		}
	}
	prioritized := prioritizeMeasurements(eoj, normal)
	for i, epc := range []byte{0xf2, 0xf3, 0xf4, 0xf5} {
		if prioritized[i] != epc {
			t.Fatalf("power not prioritized: %x", prioritized)
		}
	}
}

func TestRejectedFixedFiltering(t *testing.T) {
	s := NewECHONETSource("192.168.100.202", nil, false)
	eoj := [3]byte{2, 0x7d, 1}
	s.rememberFixedOutcome(eoj, 0xc7, fmt.Errorf("device: %w", errGETRejected))
	s.rememberFixedOutcome(eoj, 0xd0, os.ErrDeadlineExceeded)
	s.rememberFixedOutcome(eoj, 0xe4, errGETRejected)
	got := s.pendingProperties(eoj, []byte{0xc7, 0xd0, 0xe4})
	if containsByte(got, 0xc7) || !containsByte(got, 0xd0) || !containsByte(got, 0xe4) {
		t.Fatalf("wrong filter: %x", got)
	}
	s.fullScan = true
	if !containsByte(s.pendingProperties(eoj, []byte{0xc7}), 0xc7) {
		t.Fatal("full scan must retry")
	}
	s.rememberFixedOutcome(eoj, 0xc7, nil)
	if s.rejectedFixed["027d01:c7"] {
		t.Fatal("success did not clear rejection")
	}
	d := collectionDiagnostics{}
	d.record(eoj, []byte{0xc7}, nil, errGETRejected)
	if d.Timeouts != 0 || len(d.Missing) != 1 || d.Missing[0] != "027d01:c7(rejected)" {
		t.Fatalf("wrong diagnostic: %+v", d)
	}
}
