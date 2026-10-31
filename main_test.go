package main

import (
	"testing"
	"time"

	"github.com/epicshawty/morning/internal/weather"
)

func TestIsReleaseTag(t *testing.T) {
	cases := map[string]bool{
		"v1.0.0":     true,
		"v0.3.12":    true,
		"v12.0.7":    true,
		"dev":        false,
		"":           false,
		"(devel)":    false,
		"1.2.3":      false,
		"v1.2":       false,
		"v1.2.3.4":   false,
		"v1.2.x":     false,
		"v1.2.3-rc1": false,
		// pseudo-version stamped into a dirty local build
		"v1.0.1-0.20261031161145-c40108f2c048+dirty": false,
	}
	for in, want := range cases {
		if got := isReleaseTag(in); got != want {
			t.Errorf("isReleaseTag(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLdflagsVersionWins(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "v1.2.3"
	if got := buildVersion(); got != "v1.2.3" {
		t.Errorf("buildVersion with ldflags = %q, want v1.2.3", got)
	}
}

func TestAutoUnits(t *testing.T) {
	cases := []struct {
		country string
		want    string
	}{
		{"US", "f"},
		{"us", "f"},
		{"BS", "f"},
		{"LR", "f"},
		{"JP", "c"},
		{"GB", "c"},
		{"CA", "c"},
		{"", "c"},
	}
	for _, c := range cases {
		wx := &weather.Report{Place: weather.Place{Country: c.country}}
		if got := autoUnits(wx); got != c.want {
			t.Errorf("autoUnits(%q) = %q, want %q", c.country, got, c.want)
		}
	}
	if got := autoUnits(nil); got != "c" {
		t.Errorf("autoUnits(nil) = %q, want c", got)
	}
}

func TestLocationOf(t *testing.T) {
	loc := locationOf(weather.Place{TZ: "Asia/Tokyo"})
	if loc == nil {
		t.Fatal("expected a location for Asia/Tokyo")
	}
	// 2026-10-31T15:00Z is midnight in Tokyo, still 11:00 in New York.
	utc := time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC)
	if h := utc.In(loc).Hour(); h != 0 {
		t.Errorf("Tokyo hour = %d, want 0", h)
	}
	if day := utc.In(loc).Day(); day != 1 {
		t.Errorf("Tokyo day = %d, want 1 (already 1 November)", day)
	}

	if locationOf(weather.Place{}) != nil {
		t.Error("empty timezone should fall back to nil")
	}
	if locationOf(weather.Place{TZ: "Not/AZone"}) != nil {
		t.Error("unknown timezone should fall back to nil")
	}
}
