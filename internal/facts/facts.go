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
}

type feed struct {
	Selected []struct {
		Text  string `json:"text"`
		Year  int    `json:"year"`
		Pages []struct {
			Title string `json:"title"`
			Norm  string `json:"normalizedtitle"`
		} `json:"pages"`
	} `json:"selected"`
}

// OnThisDay returns curated events for the given date.
func OnThisDay(ctx context.Context, day time.Time) ([]Fact, error) {
	u := fmt.Sprintf("https://api.wikimedia.org/feed/v1/wikipedia/en/onthisday/selected/%02d/%02d",
		int(day.Month()), day.Day())

	var f feed
	if err := fetch.GetJSON(ctx, u, &f); err != nil {
		return nil, err
	}
	if len(f.Selected) == 0 {
		return nil, errors.New("no events for today")
	}

	out := make([]Fact, 0, len(f.Selected))
	for _, s := range f.Selected {
		t := strings.TrimSpace(s.Text)
		if t == "" {
			continue
		}
		fa := Fact{Year: s.Year, Text: t}
		if len(s.Pages) > 0 {
			title := s.Pages[0].Norm
			if title == "" {
				title = s.Pages[0].Title
			}
			if title != "" {
				fa.Link = "https://en.wikipedia.org/wiki/" + title
			}
		}
		out = append(out, fa)
	}
	if len(out) == 0 {
		return nil, errors.New("no events for today")
	}
	return out, nil
}
