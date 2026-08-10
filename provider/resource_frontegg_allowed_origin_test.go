package provider

import "testing"

// The API stores every origin with a trailing slash, so a configuration that
// omits it must still match what comes back.
func TestContainsAllowedOriginIgnoresTrailingSlash(t *testing.T) {
	stored := &fronteggAllowedOrigins{AllowedOrigins: []string{
		"http://localhost:3000/",
		"https://app.example.com/",
	}}

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
		"https://app.example.com/path",
		"",
	} {
		if containsAllowedOrigin(stored, origin) {
			t.Errorf("containsAllowedOrigin(%q) = true, want false", origin)
		}
	}
}
