package main

import "testing"

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
