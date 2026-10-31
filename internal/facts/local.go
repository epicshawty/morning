package facts

import (
	"context"
	"strings"
	"time"
)

// LocalRadiusKM is how far from the city an event may sit and still count as
// local. Wide enough for an airport or a neighbouring borough, tight enough to
// exclude "the same country".
const LocalRadiusKM = 50

// Local returns today's events that happened near a place.
//
// Wikipedia's on-this-day corpus is global, so most cities have nothing on any
// given day; callers should fall back to OnThisDay when this returns nothing.
// Matching needs both proximity (Wikidata coordinates within LocalRadiusKM) and
// the city's name in the article title or the event text, because a coordinate
// alone also matches broad entities that happen to be geolocated there.
func Local(ctx context.Context, day time.Time, lat, lon float64, place string, limit int) ([]Fact, error) {
	place = strings.TrimSpace(place)
	if place == "" || (lat == 0 && lon == 0) {
		return nil, nil
	}

	entries, err := fetchFeed(ctx, "events", day)
	if err != nil {
		return nil, err
	}

	// One coordinate lookup for the primary article of every event.
	var qids []string
	seenQID := map[string]bool{}
	for _, e := range entries {
		if len(e.Pages) == 0 {
			continue
		}
		q := e.Pages[0].QID
		if q != "" && !seenQID[q] {
			seenQID[q] = true
			qids = append(qids, q)
		}
	}
	coords, err := coordinates(ctx, qids)
	if err != nil {
		return nil, err
	}
	return filterLocal(entries, coords, coord{Lat: lat, Lon: lon}, place, limit), nil
}

// filterLocal keeps the events that happened near here and names the city.
// Split out from Local so the matching rules are testable without a network.
func filterLocal(entries []entry, coords map[string]coord, here coord, place string, limit int) []Fact {
	name := normalise(place)
	seenEvent := map[string]bool{}
	var out []Fact

	for _, e := range entries {
		if len(e.Pages) == 0 {
			continue
		}
		primary := e.Pages[0]
		if primary.QID == "" || seenEvent[primary.QID] {
			continue
		}
		c, ok := coords[primary.QID]
		if !ok {
			continue
		}
		km := haversine(here, c)
		if km > LocalRadiusKM {
			continue
		}
		haystack := normalise(primary.Title) + " " + normalise(e.Text)
		if len(name) >= 3 && !strings.Contains(haystack, name) {
			continue
		}
		text := strings.TrimSpace(e.Text)
		if text == "" {
			continue
		}
		seenEvent[primary.QID] = true

		f := Fact{Year: e.Year, Text: text, KM: int(km + 0.5)}
		title := primary.Norm
		if title == "" {
			title = primary.Title
		}
		if title != "" {
			f.Link = "https://en.wikipedia.org/wiki/" + title
		}
		out = append(out, f)
	}

	sortByDistance(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func sortByDistance(f []Fact) {
	for i := 1; i < len(f); i++ {
		for j := i; j > 0 && f[j].KM < f[j-1].KM; j-- {
			f[j], f[j-1] = f[j-1], f[j]
		}
	}
}

// normalise folds a title or phrase for loose comparison.
func normalise(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "_", " "))
}
