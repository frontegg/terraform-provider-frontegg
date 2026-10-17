package restclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRefreshFailureFallsBackOnlyToValidToken(t *testing.T) {
	for _, state := range []string{"valid", "expired", "invalidated", "unknown lifetime"} {
		t.Run(state, func(t *testing.T) {
			refreshErr := errors.New("authentication temporarily unavailable")
			calls := 0
			source := NewTokenSource(func(context.Context) (string, time.Duration, error) {
				calls++
				if calls == 1 {
					return "cached", time.Minute, nil
				}
				return "", 0, refreshErr
			})
			if _, err := source.Token(context.Background()); err != nil {
				t.Fatal(err)
			}
			source.refreshAt = time.Now().Add(-time.Second)
			switch state {
			case "expired":
				source.expiresAt = time.Now().Add(-time.Second)
			case "invalidated":
				source.Invalidate("cached")
			case "unknown lifetime":
				source.expiresAt = time.Time{}
			}
			token, err := source.Token(context.Background())
			if state == "valid" {
				if token != "cached" || err != nil {
					t.Fatalf("Token() = %q, %v; want cached token", token, err)
				}
			} else if token != "" || !errors.Is(err, refreshErr) {
				t.Fatalf("Token() = %q, %v; want refresh error", token, err)
			}
		})
	}
}

func TestRefreshHasDeadline(t *testing.T) {
	started := time.Now()
	source := NewTokenSource(func(ctx context.Context) (string, time.Duration, error) {
		deadline, ok := ctx.Deadline()
		if !ok || deadline.After(started.Add(31*time.Second)) {
			t.Errorf("refresh deadline = %v, present = %v; want at most 30s", deadline, ok)
		}
		return "token", time.Minute, nil
	})
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRateLimitedAuthenticationHonorsDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	authClient := MakeRestClient(srv.URL, "", "")
	source := NewTokenSource(func(ctx context.Context) (string, time.Duration, error) {
		return "", 0, authClient.Post(ctx, "/auth/vendor", nil, nil)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := source.Token(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Token() error = %v, want deadline exceeded", err)
	}
}

func TestWaitingForRefreshCanBeCanceled(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	source := NewTokenSource(func(ctx context.Context) (string, time.Duration, error) {
		close(started)
		select {
		case <-release:
			return "token", time.Minute, nil
		case <-ctx.Done():
			return "", 0, ctx.Err()
		}
	})
	done := make(chan error, 1)
	go func() {
		_, err := source.Token(context.Background())
		done <- err
	}()
	defer func() {
		close(release)
		if err := <-done; err != nil {
			t.Errorf("refresh: %v", err)
		}
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := source.Token(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting caller error = %v, want deadline exceeded", err)
	}
}
