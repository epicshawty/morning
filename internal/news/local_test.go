package news

import (
	"context"
	"strings"
	"testing"
)

func TestStripSourceSuffix(t *testing.T) {
	cases := []struct{ title, source, want string }{
		{"Osaka bakery wins award - Japan Times", "Japan Times", "Osaka bakery wins award"},
		{"Osaka bakery wins award | NHK", "NHK", "Osaka bakery wins award"},
		{"Osaka bakery wins award — NHK", "NHK", "Osaka bakery wins award"},
		{"Headline that mentions NHK in the middle", "NHK", "Headline that mentions NHK in the middle"},
		{"No suffix here", "", "No suffix here"},
	}
	for _, c := range cases {
		if got := stripSourceSuffix(c.title, c.source); got != c.want {
			t.Errorf("stripSourceSuffix(%q, %q) = %q, want %q", c.title, c.source, got, c.want)
		}
	}
}

const googleNewsFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <title>"Tokyo" - Google News</title>
  <item>
    <title>Shinkansen line resumes service - Japan Today</title>
    <link>https://news.google.com/rss/articles/abc</link>
    <source url="https://japantoday.com">Japan Today</source>
  </item>
  <item>
    <title>Quake drill held across the city - NHK</title>
    <link>https://news.google.com/rss/articles/def</link>
    <source url="https://www3.nhk.or.jp">NHK</source>
  </item>
</channel></rss>`

// TestLocalFeedShape proves the <source> outlet survives parsing and that the
// duplicated " - Outlet" suffix is removed.
func TestLocalFeedShape(t *testing.T) {
	url := serve(t, googleNewsFixture)
	items, err := parseRSS(context.Background(), url, "Google News")
	if err != nil {
		t.Fatalf("parseRSS: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Source != "Japan Today" {
		t.Errorf("source = %q, want Japan Today", items[0].Source)
	}
	if strings.HasSuffix(items[0].Title, "Japan Today") {
		t.Errorf("title still carries the outlet: %q", items[0].Title)
	}
	if items[0].Title != "Shinkansen line resumes service" {
		t.Errorf("title = %q", items[0].Title)
	}
	if items[1].Source != "NHK" {
		t.Errorf("second source = %q", items[1].Source)
	}
}

func TestLocalWithoutAPlaceIsAnError(t *testing.T) {
	if _, err := Local(context.Background(), "   ", 3); err == nil {
		t.Fatal("expected an error for an empty place")
	}
}
