package restclient

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const vendorLoginPath = "/auth/vendor"

const tokenRefreshTimeout = 30 * time.Second

const tokenRefreshRetryDelay = 5 * time.Second

// TokenSource shares one management token across clients; a zero lifetime means refresh only on a 401.
type TokenSource struct {
	mu         sync.Mutex
	refresh    func(context.Context) (string, time.Duration, error)
	token      string
	refreshAt  time.Time
	expiresAt  time.Time
	refreshing chan struct{}
}

func NewVendorTokenSource(baseURL, environmentId, applicationId, clientID, secretKey string) *TokenSource {
	login := MakeRestClient(baseURL, environmentId, applicationId)
	login.redactResponses = true
	credentials := vendorCredentials{ClientID: clientID, SecretKey: secretKey}
	return newTokenSource(func(ctx context.Context) (string, time.Duration, error) {
		return vendorLogin(ctx, &login, credentials)
	})
}

func newTokenSource(refresh func(context.Context) (string, time.Duration, error)) *TokenSource {
	return &TokenSource{refresh: refresh}
}

type vendorCredentials struct {
	ClientID  string `json:"clientId"`
	SecretKey string `json:"secret"`
}

func vendorLogin(ctx context.Context, login *Client, credentials vendorCredentials) (string, time.Duration, error) {
	var out struct {
		AccessToken string   `json:"token"`
		ExpiresIn   *float64 `json:"expiresIn"`
	}
	if err := login.Post(ctx, vendorLoginPath, credentials, &out); err != nil {
		return "", 0, err
	}
	if out.ExpiresIn == nil || *out.ExpiresIn == 0 {
		log.Printf("[WARN] Frontegg authentication response has no positive expiresIn; management tokens will refresh only after a 401")
		return out.AccessToken, 0, nil
	}
	if *out.ExpiresIn < 0 {
		return "", 0, fmt.Errorf("management token authentication returned a negative token lifetime")
	}
	return out.AccessToken, time.Duration(*out.ExpiresIn * float64(time.Second)), nil
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
			retryAt := time.Now().Add(tokenRefreshRetryDelay)
			if retryAt.After(s.expiresAt) {
				retryAt = s.expiresAt
			}
			s.refreshAt = retryAt
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
