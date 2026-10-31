package weather

import "testing"

func TestTemp(t *testing.T) {
	cases := []struct {
		c     float64
		units string
		want  string
	}{
		{0, "c", "0°"},
		{0, "f", "32°"},
		{10.4, "c", "10°"},
		{10.4, "f", "51°"},
		{-5.5, "c", "-6°"},
		{-5.5, "f", "22°"},
		{100, "f", "212°"},
	}
	for _, c := range cases {
		if got := Temp(c.c, c.units); got != c.want {
			t.Errorf("Temp(%v, %q) = %q, want %q", c.c, c.units, got, c.want)
		}
	}
}

func TestValue(t *testing.T) {
	if got := Value(13.4, "c"); got != "13°C" {
		t.Errorf("Value celsius = %q", got)
	}
	if got := Value(13.4, "f"); got != "56°F" {
		t.Errorf("Value fahrenheit = %q", got)
	}
}

func TestDescribe(t *testing.T) {
	cases := map[int]string{
		0:  "Clear",
		3:  "Overcast",
		45: "Fog",
		61: "Rain",
		71: "Snow",
		95: "Thunderstorm",
	}
	for code, want := range cases {
		got, glyph := Describe(code)
		if got != want {
			t.Errorf("Describe(%d) = %q, want %q", code, got, want)
		}
		if glyph == "" {
			t.Errorf("Describe(%d) returned no glyph", code)
		}
	}
}

func TestClock(t *testing.T) {
	if got := clock("2026-10-31T07:16"); got != "07:16" {
		t.Errorf("clock = %q", got)
	}
	if got := clock("weird"); got != "weird" {
		t.Errorf("clock should pass through unparseable input, got %q", got)
	}
}

func TestPlaceLabel(t *testing.T) {
	if got := (Place{Name: "Boston", Country: "US"}).Label(); got != "Boston, US" {
		t.Errorf("Label = %q", got)
	}
	if got := (Place{Name: "Boston"}).Label(); got != "Boston" {
		t.Errorf("Label without country = %q", got)
	}
	if got := (Place{}).Label(); got != "" {
		t.Errorf("empty Label = %q", got)
	}
}
