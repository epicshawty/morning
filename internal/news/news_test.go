package news

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestClean(t *testing.T) {
	in := "Tom &amp; Jerry&#39;s   \"great\"\n\tadventure &nbsp;here"
	want := `Tom & Jerry's "great" adventure here`
	if got := clean(in); got != want {
		t.Errorf("clean = %q, want %q", got, want)
	}
}

func TestInterleave(t *testing.T) {
	a := []Item{{Title: "a1"}, {Title: "a2"}, {Title: "a3"}}
	b := []Item{{Title: "b1"}, {Title: "b2"}}

	got := interleave([][]Item{a, b}, 4)
	want := []string{"a1", "b1", "a2", "b2"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Title != want[i] {
			t.Errorf("item %d = %q, want %q", i, got[i].Title, want[i])
		}
	}

	// A nil list must not stop the other one.
	got = interleave([][]Item{a, nil}, 2)
	if len(got) != 2 || got[0].Title != "a1" || got[1].Title != "a2" {
		t.Errorf("nil list broke interleave: %+v", got)
	}
}

const rssFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <title>Example</title>
  <item><title>First &amp; foremost</title><link>https://example.com/1</link></item>
  <item><title>Second</title><link>https://example.com/2</link></item>
</channel></rss>`

const atomFixture = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example</title>
  <entry>
    <title>Atom one</title>
    <link rel="alternate" href="https://example.org/one"/>
  </entry>
</feed>`

func serve(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestParseRSS(t *testing.T) {
	url := serve(t, rssFixture)
	items, err := parseRSS(context.Background(), url, "Test")
	if err != nil {
		t.Fatalf("parseRSS: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Title != "First & foremost" {
		t.Errorf("title = %q, entity not decoded", items[0].Title)
	}
	if items[0].Source != "Test" || items[0].URL != "https://example.com/1" {
		t.Errorf("unexpected item: %+v", items[0])
	}
}

func TestParseAtom(t *testing.T) {
	url := serve(t, atomFixture)
	items, err := parseRSS(context.Background(), url, "Test")
	if err != nil {
		t.Fatalf("parseRSS atom: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Atom one" {
		t.Fatalf("unexpected items: %+v", items)
	}
	if items[0].URL != "https://example.org/one" {
		t.Errorf("atom link = %q", items[0].URL)
	}
}

func TestParseRSSRejectsGarbage(t *testing.T) {
	url := serve(t, "not xml at all")
	if _, err := parseRSS(context.Background(), url, "Test"); err == nil {
		t.Fatal("expected an error for a non-feed body")
	}
}

// TestFeedFixtures guards the real-world feed shapes we depend on.
func TestFeedFixtures(t *testing.T) {
	for _, name := range []string{"bbc.xml", "hnrss.xml"} {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Skipf("fixture missing: %v", err)
			}
			url := serve(t, string(b))
			items, err := parseRSS(context.Background(), url, "Fixture")
			if err != nil {
				t.Fatalf("parseRSS(%s): %v", name, err)
			}
			if len(items) == 0 {
				t.Fatalf("%s yielded no items", name)
			}
			for _, it := range items {
				if it.Title == "" {
					t.Fatalf("%s produced an empty title", name)
				}
			}
		})
	}
}
