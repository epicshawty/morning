// Package cache is a tiny file cache so repeat runs are instant and upstream
// services are not hammered. Failures are always soft: a cache miss just refetches.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

// Store is a TTL-bounded blob cache on disk.
type Store struct {
	dir string
	ttl time.Duration
}

// Open returns a Store under the user cache dir. A zero ttl disables caching.
func Open(ttl time.Duration) *Store {
	if ttl <= 0 {
		return &Store{}
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return &Store{}
	}
	dir := filepath.Join(base, "morning", "http")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &Store{}
	}
	return &Store{dir: dir, ttl: ttl}
}

func (s *Store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:12])+".cache")
}

// Get returns a cached body when it is younger than the TTL.
func (s *Store) Get(key string) ([]byte, bool) {
	if s == nil || s.dir == "" {
		return nil, false
	}
	p := s.path(key)
	fi, err := os.Stat(p)
	if err != nil || time.Since(fi.ModTime()) > s.ttl {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return b, true
}

// Set stores body. Errors are intentionally swallowed: caching is best-effort.
func (s *Store) Set(key string, body []byte) {
	if s == nil || s.dir == "" {
		return
	}
	_ = os.WriteFile(s.path(key), body, 0o644)
}
