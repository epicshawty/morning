package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type memCache struct {
	store map[string][]byte
	hits  int
}

func (m *memCache) Get(k string) ([]byte, bool) {
	b, ok := m.store[k]
	if ok {
		m.hits++
	}
	return b, ok
}

func (m *memCache) Set(k string, b []byte) { m.store[k] = b }

func TestGetOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Error("no User-Agent sent")
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	b, err := Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(b) != `{"ok":true}` {
		t.Errorf("body = %q", b)
	}
}

func TestGetNon200IsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	if _, err := Get(context.Background(), srv.URL); err == nil {
		t.Fatal("expected an error for a 429")
	}
}

func TestGetJSONDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"Boston","lat":42.36}`))
	}))
	defer srv.Close()

	var v struct {
		Name string  `json:"name"`
		Lat  float64 `json:"lat"`
	}
	if err := GetJSON(context.Background(), srv.URL, &v); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if v.Name != "Boston" || v.Lat != 42.36 {
		t.Errorf("decoded %+v", v)
	}
}

func TestGetJSONRejectsBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))
	defer srv.Close()

	var v map[string]any
	if err := GetJSON(context.Background(), srv.URL, &v); err == nil {
		t.Fatal("expected an error for a non-JSON body")
	}
}

// TestCacheShortCircuits proves a cached body is returned without a second hit.
func TestCacheShortCircuits(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	UseCache(&memCache{store: map[string][]byte{}})
	t.Cleanup(func() { Cache = nil })

	for i := 0; i < 3; i++ {
		b, err := Get(context.Background(), srv.URL)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if string(b) != "payload" {
			t.Fatalf("body = %q", b)
		}
	}
	if calls != 1 {
		t.Errorf("upstream called %d times, want 1 (cache should absorb the rest)", calls)
	}
}

func TestUseCacheIgnoresNil(t *testing.T) {
	UseCache(nil)
	if Cache != nil {
		t.Error("nil cache should be ignored")
	}
}
