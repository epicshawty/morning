package cache

import (
	"testing"
	"time"
)

func TestGetSet(t *testing.T) {
	s := Open(time.Hour)
	s.dir = t.TempDir()

	s.Set("https://example.com/a", []byte("hello"))
	b, ok := s.Get("https://example.com/a")
	if !ok || string(b) != "hello" {
		t.Fatalf("Get = %q, %v", b, ok)
	}

	if _, ok := s.Get("https://example.com/other"); ok {
		t.Error("unexpected hit for a key that was never set")
	}
}

func TestExpiredEntryIsAMiss(t *testing.T) {
	s := Open(time.Nanosecond)
	s.dir = t.TempDir()
	s.Set("k", []byte("v"))
	time.Sleep(2 * time.Millisecond)
	if _, ok := s.Get("k"); ok {
		t.Error("expired entry should be a miss")
	}
}

func TestZeroTTLDisablesCache(t *testing.T) {
	s := Open(0)
	s.Set("k", []byte("v"))
	if _, ok := s.Get("k"); ok {
		t.Error("a zero TTL should never cache")
	}
}
