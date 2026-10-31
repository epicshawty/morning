// Package news aggregates keyless RSS feeds and the Hacker News API.
package news

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/epicshawty/morning/internal/fetch"
)

// feedResult is one upstream feed's outcome.
type feedResult struct {
	name  string
	items []Item
}

// Item is one headline.
type Item struct {
	Title  string `json:"title"`
	Source string `json:"source"`
	URL    string `json:"url,omitempty"`
	Score  int    `json:"score,omitempty"`
}

type rssFeed struct {
	Channel struct {
		Items []struct {
			Title   string `xml:"title"`
			Link    string `xml:"link"`
			PubDate string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

type atomFeed struct {
	Entries []struct {
		Title string `xml:"title"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

// worldFeeds are tried in order until one yields headlines.
var worldFeeds = []struct{ name, url string }{
	{"BBC", "https://feeds.bbci.co.uk/news/world/rss.xml"},
	{"NPR", "https://feeds.npr.org/1001/rss.xml"},
	{"Guardian", "https://www.theguardian.com/world/rss"},
	{"Al Jazeera", "https://www.aljazeera.com/xml/rss/all.xml"},
}

// World returns general headlines, merging the first two reachable feeds.
func World(ctx context.Context, limit int) ([]Item, error) {
	ch := make(chan feedResult, len(worldFeeds))
	var wg sync.WaitGroup
	for _, f := range worldFeeds {
		wg.Add(1)
		go func(name, url string) {
			defer wg.Done()
			items, err := parseRSS(ctx, url, name)
			if err != nil {
				ch <- feedResult{name: name}
				return
			}
			ch <- feedResult{name: name, items: items}
		}(f.name, f.url)
	}
	wg.Wait()
	close(ch)

	var ok []feedResult
	for r := range ch {
		if len(r.items) > 0 {
			ok = append(ok, r)
		}
	}
	if len(ok) == 0 {
		return nil, errors.New("no news feed reachable")
	}
	sort.Slice(ok, func(i, j int) bool { return len(ok[i].items) > len(ok[j].items) })

	// Interleave the two strongest feeds so one outlet cannot dominate.
	second := []Item(nil)
	if len(ok) > 1 {
		second = ok[1].items
	}
	return interleave([][]Item{ok[0].items, second}, limit), nil
}

// Tech returns developer headlines from Hacker News plus Lobsters.
func Tech(ctx context.Context, limit int) ([]Item, error) {
	var (
		wg      sync.WaitGroup
		hn, lob []Item
		hnErr   error
	)
	wg.Add(2)
	go func() { defer wg.Done(); hn, hnErr = HackerNews(ctx, limit) }()
	go func() { defer wg.Done(); lob, _ = Lobsters(ctx, limit) }()
	wg.Wait()

	out := interleave([][]Item{hn, lob}, limit)
	if len(out) == 0 && hnErr != nil {
		return nil, hnErr
	}
	return out, nil
}

// HackerNews returns the top stories with their scores.
func HackerNews(ctx context.Context, limit int) ([]Item, error) {
	var ids []int64
	if err := fetch.GetJSON(ctx, "https://hacker-news.firebaseio.com/v0/topstories.json", &ids); err != nil {
		return nil, err
	}
	if len(ids) > limit*2 {
		ids = ids[:limit*2]
	}

	type slot struct {
		i   int
		it  Item
		err error
	}
	ch := make(chan slot, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id int64) {
			defer wg.Done()
			var s struct {
				Title string `json:"title"`
				URL   string `json:"url"`
				Score int    `json:"score"`
				Dead  bool   `json:"dead"`
				Type  string `json:"type"`
			}
			u := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id)
			if err := fetch.GetJSON(ctx, u, &s); err != nil {
				ch <- slot{i: i, err: err}
				return
			}
			if s.Dead || s.Title == "" || s.Type != "story" {
				ch <- slot{i: i, err: errors.New("skip")}
				return
			}
			link := s.URL
			if link == "" {
				link = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", id)
			}
			ch <- slot{i: i, it: Item{Title: s.Title, Source: "HN", URL: link, Score: s.Score}}
		}(i, id)
	}
	wg.Wait()
	close(ch)

	slots := make([]slot, 0, len(ids))
	for s := range ch {
		slots = append(slots, s)
	}
	sort.Slice(slots, func(a, b int) bool { return slots[a].i < slots[b].i })

	out := make([]Item, 0, limit)
	for _, s := range slots {
		if s.err != nil {
			continue
		}
		out = append(out, s.it)
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("hacker news unavailable")
	}
	return out, nil
}

// Lobsters returns the hottest links.
func Lobsters(ctx context.Context, limit int) ([]Item, error) {
	var raw []struct {
		Title      string `json:"title"`
		URL        string `json:"url"`
		ShortID    string `json:"short_id"`
		Score      int    `json:"score"`
		CommentCnt int    `json:"comment_count"`
	}
	if err := fetch.GetJSON(ctx, "https://lobste.rs/hottest.json", &raw); err != nil {
		return nil, err
	}
	out := make([]Item, 0, limit)
	for _, r := range raw {
		if r.Title == "" {
			continue
		}
		link := r.URL
		if link == "" {
			link = "https://lobste.rs/s/" + r.ShortID
		}
		out = append(out, Item{Title: r.Title, Source: "lobsters", URL: link, Score: r.Score})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func parseRSS(ctx context.Context, url, source string) ([]Item, error) {
	b, err := fetch.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var f rssFeed
	if err := xml.Unmarshal(b, &f); err == nil && len(f.Channel.Items) > 0 {
		out := make([]Item, 0, len(f.Channel.Items))
		for _, it := range f.Channel.Items {
			t := strings.TrimSpace(it.Title)
			if t == "" {
				continue
			}
			out = append(out, Item{Title: clean(t), Source: source, URL: strings.TrimSpace(it.Link)})
		}
		return out, nil
	}
	var a atomFeed
	if err := xml.Unmarshal(b, &a); err == nil && len(a.Entries) > 0 {
		out := make([]Item, 0, len(a.Entries))
		for _, e := range a.Entries {
			var link string
			for _, l := range e.Links {
				if l.Rel == "alternate" || l.Rel == "" {
					link = l.Href
					break
				}
			}
			t := strings.TrimSpace(e.Title)
			if t == "" {
				continue
			}
			out = append(out, Item{Title: clean(t), Source: source, URL: link})
		}
		return out, nil
	}
	return nil, errors.New("unrecognized feed format")
}

// clean collapses whitespace and strips HTML entities that leak into titles.
func clean(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#39;", "'", "&apos;", "'",
		"&lt;", "<", "&gt;", ">", "&nbsp;", " ", "\n", " ", "\t", " ")
	return strings.Join(strings.Fields(r.Replace(s)), " ")
}

// interleave round-robins several lists, then truncates to limit.
func interleave(lists [][]Item, limit int) []Item {
	var out []Item
	for n := 0; ; n++ {
		added := false
		for _, l := range lists {
			if n < len(l) {
				out = append(out, l[n])
				added = true
				if len(out) >= limit {
					return out
				}
			}
		}
		if !added {
			return out
		}
	}
}
