// Package fetch is the single HTTP entry point for morning.
// One client, one User-Agent, bounded timeouts, no surprises.
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Version is stamped by main at start-up so every request identifies itself.
var Version = "dev"

var client = &http.Client{Timeout: 12 * time.Second}

// UserAgent is the polite identity sent to every upstream host.
func UserAgent() string {
	return "morning/" + Version + " (+https://github.com/epicshawty/morning)"
}

// Store is the subset of cache.Store that fetch needs. Declaring it here keeps
// the HTTP layer free of a hard dependency on the cache package.
type Store interface {
	Get(string) ([]byte, bool)
	Set(string, []byte)
}

// Cache memoises every GET for the store's TTL when non-nil.
var Cache Store

// UseCache attaches a response cache. A nil store is ignored.
func UseCache(s Store) {
	if s != nil {
		Cache = s
	}
}

// Get fetches url and returns the body, capped at 4 MiB.
func Get(ctx context.Context, url string) ([]byte, error) {
	if Cache != nil {
		if b, ok := Cache.Get(url); ok {
			return b, nil
		}
	}
	b, err := get(ctx, url)
	if err != nil {
		return nil, err
	}
	if Cache != nil {
		Cache.Set(url, b)
	}
	return b, nil
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent())
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: http %d", hostOf(url), resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", hostOf(url), err)
	}
	return b, nil
}

// GetJSON fetches url and decodes it into v.
func GetJSON(ctx context.Context, url string, v any) error {
	b, err := Get(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: bad json: %w", hostOf(url), err)
	}
	return nil
}

func hostOf(url string) string {
	u, err := parseHost(url)
	if err != nil {
		return "upstream"
	}
	return u
}
