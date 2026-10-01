package main

import "testing"

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
