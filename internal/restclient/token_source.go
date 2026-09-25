package restclient

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TokenSource shares one vendor token across the API and portal clients. The
// refresh function returns the token and its lifetime; a zero lifetime means
// the server did not provide an expiry, so a 401 will trigger the refresh.
type TokenSource struct {
	mu        sync.Mutex
	refresh   func(context.Context) (string, time.Duration, error)
	token     string
	refreshAt time.Time
}

func NewTokenSource(refresh func(context.Context) (string, time.Duration, error)) *TokenSource {
	return &TokenSource{refresh: refresh}
}

func (s *TokenSource) Token(ctx context.Context) (string, error) {
	// The lock is held across the refresh on purpose: concurrent callers wait
	// for one vendor authentication instead of each starting their own. A
	// rate-limited refresh therefore blocks every request until it completes.
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && (s.refreshAt.IsZero() || time.Now().Before(s.refreshAt)) {
		return s.token, nil
	}

	// Start the lifetime before the network call, so its duration cannot make
	// the client use a token beyond the server's expiry.
	startedAt := time.Now()
	token, lifetime, err := s.refresh(ctx)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("vendor authentication returned an empty token")
	}

	s.token = token
	s.refreshAt = time.Time{}
	if lifetime > 0 {
		margin := min(30*time.Second, lifetime/10)
		s.refreshAt = startedAt.Add(lifetime - margin)
	}
	return token, nil
}

// Invalidate forces a refresh after a 401, unless another request has already
// replaced the rejected token.
func (s *TokenSource) Invalidate(rejectedToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == rejectedToken {
		s.refreshAt = time.Time{}
		s.token = ""
	}
}
