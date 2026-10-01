package main

import "testing"

func TestParseECHONETPropertyMapList(t *testing.T) {
	props, err := parseECHONETPropertyMap([]byte{0x03, 0x80, 0x81, 0xe0})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x80, 0x81, 0xe0}
	if !sameBytes(props, want) {
		t.Fatalf("got %x, want %x", props, want)
	}
}

func TestParseECHONETPropertyMapBitmap(t *testing.T) {
	data := []byte{0x10, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x82}
	props, err := parseECHONETPropertyMap(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x80, 0x81, 0x9f, 0xff}
	if !sameBytes(props, want) {
		t.Fatalf("got %x, want %x", props, want)
	}
}

func TestFormatEOJAndClassName(t *testing.T) {
	eoj := [3]byte{0x02, 0x7d, 0x01}
	if got := formatEOJ(eoj); got != "027d01" {
		t.Fatalf("got %q", got)
	}
	if got := echonetClassName(eoj); got != "蓄電池" {
		t.Fatalf("got %q", got)
	}
}

func sameBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
