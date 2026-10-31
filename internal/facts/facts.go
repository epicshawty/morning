// Package facts provides the "on this day" section from Wikimedia.
package facts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/epicshawty/morning/internal/fetch"
)

// Fact is one historical event.
type Fact struct {
	Year int    `json:"year"`
	Text string `json:"text"`
	Link string `json:"link,omitempty"`
	// KM is the distance from the requested place, set only for local facts.
	KM int `json:"km,omitempty"`
}

type page struct {
	Title string `json:"title"`
	Norm  string `json:"normalizedtitle"`
	QID   string `json:"wikibase_item"`
}

type entry struct {
	Text  string `json:"text"`
	Year  int    `json:"year"`
	Pages []page `json:"pages"`
}

type feed struct {
	Selected []entry `json:"selected"`
	Events   []entry `json:"events"`
}

func feedURL(kind string, day time.Time) string {
	return fmt.Sprintf("https://api.wikimedia.org/feed/v1/wikipedia/en/onthisday/%s/%02d/%02d",
		kind, int(day.Month()), day.Day())
}

// fetchFeed pulls one on-this-day feed: "selected" is the curated set,
// "events" the wider pool the local search needs.
func fetchFeed(ctx context.Context, kind string, day time.Time) ([]entry, error) {
	var f feed
	if err := fetch.GetJSON(ctx, feedURL(kind, day), &f); err != nil {
		return nil, err
	}
	switch kind {
	case "selected":
		return f.Selected, nil
	case "events":
		return f.Events, nil
	}
	return nil, errors.New("unknown feed: " + kind)
}

// OnThisDay returns the curated global events for the given date.
func OnThisDay(ctx context.Context, day time.Time) ([]Fact, error) {
	entries, err := fetchFeed(ctx, "selected", day)
	if err != nil {
		return nil, err
	}
	out := toFacts(entries)
	if len(out) == 0 {
		return nil, errors.New("no events for today")
	}
	return out, nil
}

func toFacts(entries []entry) []Fact {
	out := make([]Fact, 0, len(entries))
	for _, e := range entries {
		t := strings.TrimSpace(e.Text)
		if t == "" {
			continue
		}
		f := Fact{Year: e.Year, Text: t}
		if len(e.Pages) > 0 {
			title := e.Pages[0].Norm
			if title == "" {
				title = e.Pages[0].Title
			}
			if title != "" {
				f.Link = "https://en.wikipedia.org/wiki/" + title
			}
		}
		out = append(out, f)
	}
	return out
}
