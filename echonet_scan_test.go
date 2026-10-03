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

func TestECHONETPropertyNamesFromDeviceDescriptions(t *testing.T) {
	tests := []struct {
		eoj  [3]byte
		epc  byte
		want string
	}{
		{[3]byte{0x02, 0x79, 0x01}, 0xe3, "積算売電電力量計測値"},
		{[3]byte{0x02, 0x7d, 0x01}, 0xd3, "瞬時充放電電力計測値"},
		{[3]byte{0x02, 0x88, 0x01}, 0xe7, "瞬時電力計測値"},
		{[3]byte{0x05, 0xff, 0x01}, 0xc2, "機器情報リスト"},
	}
	for _, tt := range tests {
		if got := echonetPropertyName(tt.eoj, tt.epc); got != tt.want {
			t.Fatalf("echonetPropertyName(%x, %02x) = %q, want %q", tt.eoj, tt.epc, got, tt.want)
		}
	}
}

func TestECHONETPowerDescriptions(t *testing.T) {
	if got := describeECHONETValue([3]byte{0x02, 0x7d, 0x01}, 0xd3, []byte{0xff, 0xff, 0xf3, 0x58}); got != "-3240 W" {
		t.Fatalf("got %q", got)
	}
	if got := describeECHONETValue([3]byte{0x02, 0x88, 0x01}, 0xe7, []byte{0x00, 0x00, 0x00, 0x8c}); got != "140 W" {
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
