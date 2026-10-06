package main

import (
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

func TestDeriveECHONETSummaryUsesControllerExportCandidate(t *testing.T) {
	sample := Sample{
		Source: "echonet",
		Properties: []SampleProperty{
			{EOJ: "027901", EPC: "e0", Float: floatPtr(2040)},
			{EOJ: "05ff01", EPC: "f2", Float: floatPtr(1200)},
		},
	}
	deriveECHONETSummary(&sample)
	if sample.GridWatts == nil || *sample.GridWatts != -1200 {
		t.Fatalf("GridWatts = %v, want -1200", sample.GridWatts)
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
