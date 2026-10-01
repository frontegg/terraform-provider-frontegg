package restclient

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const tokenRefreshTimeout = 30 * time.Second

// TokenSource shares one management token across the API and portal clients.
// The refresh function returns the token and its lifetime; a zero lifetime
// means the server did not provide an expiry, so a 401 triggers the refresh.
type TokenSource struct {
	mu         sync.Mutex
	refresh    func(context.Context) (string, time.Duration, error)
	token      string
	refreshAt  time.Time
	expiresAt  time.Time
	refreshing chan struct{}
}

func NewTokenSource(refresh func(context.Context) (string, time.Duration, error)) *TokenSource {
	return &TokenSource{refresh: refresh}
}

func (s *TokenSource) Token(ctx context.Context) (string, error) {
	// Bound both authentication (including rate-limit waits) and the time a
	// caller spends waiting for another request to refresh the shared token.
	ctx, cancel := context.WithTimeout(ctx, tokenRefreshTimeout)
	defer cancel()

	for {
		s.mu.Lock()
		if err := ctx.Err(); err != nil {
			s.mu.Unlock()
			return "", err
		}
		if s.token != "" && (s.refreshAt.IsZero() || time.Now().Before(s.refreshAt)) {
			token := s.token
			s.mu.Unlock()
			return token, nil
		}
		if pending := s.refreshing; pending != nil {
			s.mu.Unlock()
			select {
			case <-pending:
				continue
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		s.refreshing = make(chan struct{})
		s.mu.Unlock()
		break
	}

	// Start the lifetime before the network call, so its duration cannot make
	// the client use a token beyond the server's expiry.
	startedAt := time.Now()
	token, lifetime, err := s.refresh(ctx)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && token == "" {
		err = fmt.Errorf("management token authentication returned an empty token")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		close(s.refreshing)
		s.refreshing = nil
	}()
	if err != nil {
		// An early refresh failure need not fail the resource while the old
		// token is still valid. Invalidate clears it after a 401, so a rejected
		// token can never be reused here. Unknown lifetimes also cannot fall back.
		if ctx.Err() == nil && s.token != "" && time.Now().Before(s.expiresAt) {
			log.Printf("[WARN] Management token refresh failed; using the cached token until its expiry")
			return s.token, nil
		}
		return "", err
	}

	s.token = token
	s.refreshAt = time.Time{}
	s.expiresAt = time.Time{}
	if lifetime > 0 {
		margin := min(30*time.Second, lifetime/10)
		s.expiresAt = startedAt.Add(lifetime)
		s.refreshAt = s.expiresAt.Add(-margin)
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
		s.expiresAt = time.Time{}
		s.token = ""
	}
}
