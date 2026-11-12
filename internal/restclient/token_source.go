package restclient

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const tokenRefreshTimeout = 30 * time.Second

// TokenSource shares one management token across clients; a zero lifetime means refresh only on a 401.
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
	// Bounds both the refresh, including rate-limit waits, and waiting on another caller's refresh.
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

	// Measured before the call so its duration can't push use past the server's expiry.
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
		// A rejected (invalidated) or unknown-lifetime token never reaches this fallback.
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

// Invalidate forces a refresh after a 401 unless another request already replaced the token.
func (s *TokenSource) Invalidate(rejectedToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == rejectedToken {
		s.refreshAt = time.Time{}
		s.expiresAt = time.Time{}
		s.token = ""
	}
}
