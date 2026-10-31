package facts

import (
	"math"
	"testing"
)

func TestParsePoint(t *testing.T) {
	c, ok := parsePoint("Point(-74.0131 40.7173)")
	if !ok {
		t.Fatal("parsePoint failed")
	}
	if math.Abs(c.Lat-40.7173) > 1e-9 || math.Abs(c.Lon-(-74.0131)) > 1e-9 {
		t.Errorf("got %+v, want lat 40.7173 lon -74.0131", c)
	}

	for _, bad := range []string{"", "Point()", "Point(1)", "Point(a b)", "garbage"} {
		if _, ok := parsePoint(bad); ok {
			t.Errorf("parsePoint(%q) should fail", bad)
		}
	}
}

func TestHaversine(t *testing.T) {
	ny := coord{Lat: 40.7128, Lon: -74.0060}
	london := coord{Lat: 51.5074, Lon: -0.1278}
	if d := haversine(ny, london); math.Abs(d-5570) > 30 {
		t.Errorf("New York to London = %.0f km, want ~5570", d)
	}
	if d := haversine(ny, ny); d > 0.001 {
		t.Errorf("distance to self = %v, want 0", d)
	}
}

func TestFilterLocal(t *testing.T) {
	here := coord{Lat: 40.7128, Lon: -74.0060} // New York

	entries := []entry{
		{Year: 2017, Text: "A truck drives into a crowd in Lower Manhattan, New York City.", Pages: []page{
			{Title: "2017_New_York_City_truck_attack", QID: "Q42440670"},
		}},
		{Year: 2020, Text: "Berlin Brandenburg Airport opens its doors.", Pages: []page{
			{Title: "Berlin_Brandenburg_Airport", QID: "Q160556"},
		}},
		{Year: 1989, Text: "The Berlin Wall falls.", Pages: []page{
			{Title: "Berlin_Wall", QID: "Q4004"},
		}},
		{Year: 1900, Text: "No article for this one.", Pages: []page{
			{Title: "Untitled", QID: "Q999"},
		}},
		// Duplicate of the first event, as the feed carries it twice.
		{Year: 2017, Text: "A truck drives into a crowd in Lower Manhattan, New York City.", Pages: []page{
			{Title: "2017_New_York_City_truck_attack", QID: "Q42440670"},
		}},
	}
	coords := map[string]coord{
		"Q42440670": {Lat: 40.7173, Lon: -74.0131}, // Lower Manhattan
		"Q160556":   {Lat: 52.3667, Lon: 13.5033},  // Berlin
		"Q4004":     {Lat: 52.5163, Lon: 13.3777},  // Berlin
	}

	got := filterLocal(entries, coords, here, "New York", 5)
	if len(got) != 1 {
		t.Fatalf("got %d local facts, want 1: %+v", len(got), got)
	}
	if got[0].Year != 2017 || got[0].KM > 5 {
		t.Errorf("unexpected fact: %+v", got[0])
	}
	if got[0].Link == "" {
		t.Error("expected a Wikipedia link")
	}

	// A city with nothing nearby yields nothing rather than the global list.
	if got := filterLocal(entries, coords, here, "Tokyo", 5); len(got) != 0 {
		t.Errorf("Tokyo should have no local facts, got %+v", got)
	}

	// Without coordinates the event cannot be placed, so it is skipped.
	if got := filterLocal(entries, map[string]coord{}, here, "New York", 5); len(got) != 0 {
		t.Errorf("no coords should yield nothing, got %+v", got)
	}
}

func TestFilterLocalNeedsTheName(t *testing.T) {
	here := coord{Lat: 51.5074, Lon: -0.1278} // London
	// Coordinates say London, but nothing on the page mentions London — the
	// classic false positive from a broadly-geolocated article.
	entries := []entry{{Year: 1863, Text: "British forces in New Zealand resume the New Zealand Wars.", Pages: []page{
		{Title: "British_Empire", QID: "Q8680"},
	}}}
	coords := map[string]coord{"Q8680": {Lat: 51.5074, Lon: -0.1278}}

	if got := filterLocal(entries, coords, here, "London", 5); len(got) != 0 {
		t.Errorf("name guard should drop this, got %+v", got)
	}
}

func TestFilterLocalSortsByDistance(t *testing.T) {
	here := coord{Lat: 52.52, Lon: 13.405} // Berlin
	entries := []entry{
		{Year: 2000, Text: "Far away in Berlin.", Pages: []page{{Title: "Far_Berlin", QID: "QF"}}},
		{Year: 2001, Text: "Close by in Berlin.", Pages: []page{{Title: "Near_Berlin", QID: "QN"}}},
	}
	coords := map[string]coord{
		"QF": {Lat: 52.78, Lon: 13.78}, // ~40 km out, still inside the radius
		"QN": {Lat: 52.521, Lon: 13.406},
	}
	got := filterLocal(entries, coords, here, "Berlin", 5)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	if got[0].Year != 2001 {
		t.Errorf("closest should come first, got %+v", got)
	}
	if got[0].KM > got[1].KM {
		t.Errorf("not sorted by distance: %+v", got)
	}
}

func TestLocalRadiusCoversTheSuburbs(t *testing.T) {
	if LocalRadiusKM < 20 || LocalRadiusKM > 100 {
		t.Errorf("LocalRadiusKM = %d, want a suburban radius", LocalRadiusKM)
	}
}
