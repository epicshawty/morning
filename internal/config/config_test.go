package config

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	d := Default()
	if cfg.Units != d.Units || cfg.Limit != d.Limit || len(cfg.Sections) != len(d.Sections) {
		t.Errorf("got %+v, want defaults %+v", cfg, d)
	}
}

func TestNormalizeRejectsJunk(t *testing.T) {
	cfg := Config{Units: "kelvin", Limit: -3}
	cfg.normalize()
	if cfg.Units != "" {
		t.Errorf("units = %q, want empty (auto)", cfg.Units)
	}
	if cfg.Limit != 5 {
		t.Errorf("limit = %d, want 5", cfg.Limit)
	}
	if len(cfg.Sections) == 0 {
		t.Error("sections should fall back to defaults")
	}
}

func TestLimitIsCapped(t *testing.T) {
	cfg := Config{Limit: 500}
	cfg.normalize()
	if cfg.Limit != 20 {
		t.Errorf("limit = %d, want 20", cfg.Limit)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	want := Config{City: "Tokyo", Units: "f", Limit: 3, Sections: []string{"weather", "tech"}}
	if err := want.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.City != "Tokyo" || got.Units != "f" || got.Limit != 3 || len(got.Sections) != 2 {
		t.Errorf("round trip mismatch: %+v", got)
	}
}
