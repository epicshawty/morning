package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/epicshawty/morning/internal/facts"
	"github.com/epicshawty/morning/internal/news"
	"github.com/epicshawty/morning/internal/weather"
)

func fixture() Data {
	return Data{
		Now:      time.Date(2026, 10, 31, 7, 42, 0, 0, time.UTC),
		Greeting: "Good morning",
		Weather: &weather.Report{
			Place:   weather.Place{Name: "Boston", Country: "US"},
			TempC:   10.4,
			FeelsC:  8.0,
			HighC:   14.0,
			LowC:    6.0,
			Rain:    20,
			WindKPH: 8,
			Desc:    "Partly cloudy",
			Emoji:   "⛅",
			Sunrise: "07:16",
			Sunset:  "17:38",
		},
		World: []news.Item{
			{Title: "A headline about the world", Source: "BBC"},
		},
		Tech: []news.Item{
			{Title: "A tool for developers", Source: "HN", Score: 412},
		},
		Facts: []facts.Fact{
			{Year: 1517, Text: "Martin Luther posted his Ninety-five Theses."},
		},
		Opts: Options{Color: false, Width: 76, Units: "c", Limit: 5},
	}
}

// TestPlainIsPureASCII is the contract for pipes: no escape codes, ever.
func TestPlainIsPureASCII(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, fixture()); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := b.String()
	if strings.ContainsRune(out, 0x1b) {
		t.Error("plain output contains an escape sequence")
	}
	for _, want := range []string{
		"Good morning",
		"Boston, US",
		"WEATHER",
		"PARTLY CLOUDY",
		"High 14°C, low 6°C, rain 20%, wind 8 km/h",
		"Sunrise 07:16, sunset 17:38",
		"WORLD",
		"- A headline about the world (BBC)",
		"TECH",
		"- A tool for developers (HN, 412 points)",
		"ON THIS DAY",
		"- 1517: Martin Luther",
	} {
		if !strings.Contains(strings.ToUpper(out), strings.ToUpper(want)) {
			t.Errorf("plain output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestStyledHasNoBrokenWrapping(t *testing.T) {
	d := fixture()
	d.Opts.Color = true
	d.Opts.Width = 76
	var b bytes.Buffer
	if err := Render(&b, d); err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Strip ANSI and check that no rendered line exceeds the requested width.
	plain := stripANSI(b.String())
	for _, ln := range strings.Split(plain, "\n") {
		if n := len([]rune(ln)); n > d.Opts.Width {
			t.Errorf("line is %d columns, over the %d limit: %q", n, d.Opts.Width, ln)
		}
	}
}

func TestMissingSectionIsReportedNotFatal(t *testing.T) {
	d := fixture()
	d.World = nil
	d.WorldErr = errFake("feed down")
	var b bytes.Buffer
	if err := Render(&b, d); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(b.String(), "unavailable: feed down") {
		t.Errorf("expected an unavailable note, got:\n%s", b.String())
	}
}

// TestUnitsSwitch confirms -units f changes every temperature, not just one.
func TestUnitsSwitch(t *testing.T) {
	d := fixture()
	d.Opts.Units = "f"
	var b bytes.Buffer
	if err := Render(&b, d); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, "51°F") {
		t.Errorf("temperature not converted to Fahrenheit:\n%s", out)
	}
	if !strings.Contains(out, "High 57°F, low 43°F") {
		t.Errorf("high/low not converted:\n%s", out)
	}
}

// TestLocalSections checks the city-specific sections and their labels.
func TestLocalSections(t *testing.T) {
	d := fixture()
	d.LocalName = "Boston"
	d.Local = []news.Item{{Title: "A local headline", Source: "The Boston Globe"}}
	d.FactCity = "Boston"
	d.Facts = []facts.Fact{{Year: 1919, Text: "The Great Molasses Flood.", KM: 2}}

	var b bytes.Buffer
	if err := Render(&b, d); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := b.String()
	for _, want := range []string{
		"LOCAL · BOSTON",
		"- A local headline (The Boston Globe)",
		"ON THIS DAY IN BOSTON",
		"- 1919: The Great Molasses Flood.",
	} {
		if !strings.Contains(strings.ToUpper(out), strings.ToUpper(want)) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// TestGlobalFactsStayUnlabelled guards the fallback: no city means no claim.
func TestGlobalFactsStayUnlabelled(t *testing.T) {
	d := fixture()
	var b bytes.Buffer
	if err := Render(&b, d); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(b.String(), "ON THIS DAY IN") {
		t.Errorf("global facts should not claim a city:\n%s", b.String())
	}
}

func TestWrap(t *testing.T) {
	got := wrap("one two three four", 7)
	want := []string{"one two", "three", "four"}
	if len(got) != len(want) {
		t.Fatalf("wrap = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func stripANSI(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			out.WriteRune(r)
		}
	}
	return out.String()
}

type errFake string

func (e errFake) Error() string { return string(e) }
