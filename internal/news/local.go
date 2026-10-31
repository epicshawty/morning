package news

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
)

// localFeeds build a search URL for a city, tried in order.
var localFeeds = []struct {
	name string
	url  func(q string) string
}{
	{"Google News", func(q string) string {
		return "https://news.google.com/rss/search?hl=en-US&gl=US&ceid=US:en&q=" + url.QueryEscape(q)
	}},
	{"Bing", func(q string) string {
		return "https://www.bing.com/news/search?format=rss&q=" + url.QueryEscape(q)
	}},
}

// Local returns recent headlines mentioning a place.
//
// Google News RSS is the keyless standby: a plain search query, one item per
// article, with the outlet in a <source> element. Bing's RSS is the fallback
// when Google rate-limits or the feed shape changes.
func Local(ctx context.Context, place string, limit int) ([]Item, error) {
	place = strings.TrimSpace(place)
	if place == "" {
		return nil, errors.New("no place to search for")
	}

	type result struct {
		index int
		items []Item
		err   error
	}
	ch := make(chan result, len(localFeeds))
	var wg sync.WaitGroup
	for i, f := range localFeeds {
		wg.Add(1)
		go func(i int, build func(string) string, name string) {
			defer wg.Done()
			items, err := parseRSS(ctx, build(place), name)
			ch <- result{index: i, items: items, err: err}
		}(i, f.url, f.name)
	}
	wg.Wait()
	close(ch)

	// Both feeds land together but are chosen in declared order, so a fast Bing
	// response cannot outrank Google's real outlet names.
	byFeed := make([]result, len(localFeeds))
	var lastErr error
	for r := range ch {
		byFeed[r.index] = r
		if r.err != nil {
			lastErr = r.err
		}
	}
	for _, r := range byFeed {
		if len(r.items) == 0 {
			continue
		}
		if limit > 0 && len(r.items) > limit {
			r.items = r.items[:limit]
		}
		return r.items, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no local headlines found")
	}
	return nil, lastErr
}
