package provider

import (
	"sync"
	"testing"
)

func TestNormalizeAllowedOrigin(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"https://example.com", "https://example.com"},
		{"https://example.com/", "https://example.com"},
		{"https://example.com/oauth", "https://example.com/oauth"},
		{"https://example.com/oauth/", "https://example.com/oauth"},
		{"", ""},
	} {
		if got := normalizeAllowedOrigin(tc.in); got != tc.want {
			t.Errorf("normalizeAllowedOrigin(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The API stores origins with a trailing slash, so a lookup for the value the user
// configured has to match the value the API returns.
func TestContainsAllowedOriginIgnoresTrailingSlash(t *testing.T) {
	stored := &fronteggAllowedOrigins{
		AllowedOrigins: []string{"http://localhost:3000/", "https://app.example.com/"},
	}

	for _, origin := range []string{
		"https://app.example.com",
		"https://app.example.com/",
		"http://localhost:3000",
	} {
		if !containsAllowedOrigin(stored, origin) {
			t.Errorf("containsAllowedOrigin(%q) = false, want true", origin)
		}
	}

	for _, origin := range []string{
		"https://other.example.com",
		"https://app.example.com/extra",
	} {
		if containsAllowedOrigin(stored, origin) {
			t.Errorf("containsAllowedOrigin(%q) = true, want false", origin)
		}
	}
}

// Guards the read-modify-write serialization: without the mutex, concurrent
// create/delete pairs interleave their reads and lose each other's writes.
func TestAllowedOriginMutexSerializesReadModifyWrite(t *testing.T) {
	shared := []string{}
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			allowedOriginMu.Lock()
			defer allowedOriginMu.Unlock()
			current := append([]string{}, shared...)
			current = append(current, "origin")
			shared = current
		}(i)
	}

	wg.Wait()

	if len(shared) != 50 {
		t.Errorf("len(shared) = %d, want 50 - writes were lost", len(shared))
	}
}
