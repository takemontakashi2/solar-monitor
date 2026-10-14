package main

import (
	"path/filepath"
	"testing"
)

func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ECHONETAddr != "" {
		t.Fatalf("ECHONETAddr = %q, want empty", cfg.ECHONETAddr)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "config.json")
	if err := SaveConfig(path, Config{ECHONETAddr: "192.168.100.202"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ECHONETAddr != "192.168.100.202" {
		t.Fatalf("ECHONETAddr = %q, want 192.168.100.202", cfg.ECHONETAddr)
	}
}

func TestBuildSourceUsesSavedECHONETAddr(t *testing.T) {
	source, err := buildSource("echonet", "", "", "pv=027901:e0", false, Config{ECHONETAddr: "192.168.100.202"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	echonet, ok := source.(*ECHONETSource)
	if !ok {
		t.Fatalf("source type = %T, want *ECHONETSource", source)
	}
	if echonet.addr != "192.168.100.202:3610" {
		t.Fatalf("addr = %q, want 192.168.100.202:3610", echonet.addr)
	}
}

func TestBuildSourceSavesExplicitECHONETAddr(t *testing.T) {
	var saved Config
	_, err := buildSource("echonet", "", "192.168.100.202", "pv=027901:e0", false, Config{}, func(cfg Config) error {
		saved = cfg
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ECHONETAddr != "192.168.100.202" {
		t.Fatalf("saved ECHONETAddr = %q, want 192.168.100.202", saved.ECHONETAddr)
	}
}

func TestHasMonitorObject(t *testing.T) {
	if hasMonitorObject([]ECHONETObject{{EOJ: [3]byte{0x05, 0xff, 0x01}}}) {
		t.Fatal("controller-only node should not be selected as monitor")
	}
	if !hasMonitorObject([]ECHONETObject{
		{EOJ: [3]byte{0x05, 0xff, 0x01}},
		{EOJ: [3]byte{0x02, 0x79, 0x01}},
	}) {
		t.Fatal("node with PV object should be selected as monitor")
	}
	if !hasMonitorObject([]ECHONETObject{{EOJ: [3]byte{0x02, 0x7d, 0x01}}}) {
		t.Fatal("node with battery object should be selected as monitor")
	}
}
