package restclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	// rateLimitResetHeader is set by the Frontegg api-gateway as an RFC 3339
	// date-time string indicating when the rate-limit window resets.
	rateLimitResetHeader = "X-Rate-Limit-Reset"
	// retryAfterHeader is parsed defensively. Frontegg does not send it today,
	// but it is the HTTP-standard signal and may appear in the future.
	retryAfterHeader = "Retry-After"

	// defaultRateLimitWait is used when a 429 carries no usable wait header.
	// This happens for upstream-proxied 429s, where the gateway strips the
	// X-Rate-Limit-* headers.
	defaultRateLimitWait = time.Minute

	// rateLimitMaxAttempts and rateLimitMaxTotalWait are safety ceilings that
	// only protect against a hang if the request context lacks a deadline.
	// They are not reached under normal operation.
	rateLimitMaxAttempts  = 100
	rateLimitMaxTotalWait = time.Hour

	// rateLimitJitter bounds the random-ish spread added on top of a computed
	// wait so that many concurrent waiters on the same route do not all wake at
	// the same instant and immediately re-trigger a 429. This is not backoff;
	// the base wait is still the server-specified reset time.
	rateLimitJitter = 2 * time.Second
)

// rateLimiter encapsulates all 429 / rate-limit policy: per-route memory of
// when a route may next be retried, parsing of the rate-limit headers, the
// wait computation (including jitter), and the safety ceilings. The REST client
// owns HTTP mechanics and delegates rate-limit decisions here.
//
// It is held behind a pointer on Client so its mutex is never copied when a
// Client value is copied (e.g. into ClientHolder).
type rateLimiter struct {
	mu       sync.Mutex
	resetAt  map[string]time.Time
	jitterIx int // advances per record to vary jitter without a clock/RNG

	// Policy knobs. Defaults come from the package constants; tests override
	// them to keep runs fast and to exercise the ceiling branch directly.
	defaultWait  time.Duration
	maxAttempts  int
	maxTotalWait time.Duration
	jitter       time.Duration
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		resetAt:      make(map[string]time.Time),
		defaultWait:  defaultRateLimitWait,
		maxAttempts:  rateLimitMaxAttempts,
		maxTotalWait: rateLimitMaxTotalWait,
		jitter:       rateLimitJitter,
	}
}

// routeKey builds the per-route map key. The query string is stripped so
// paginated reads do not fragment into many unrelated keys. Path params / IDs
// are kept as-is (see story Known Limitations).
func (rl *rateLimiter) routeKey(method, rawURL string) string {
	path := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		path = u.Path
	}
	return method + " " + path
}

// waitBeforeSend returns how long the caller must wait before sending to
// routeKey (0 if the route is not currently rate-limited).
func (rl *rateLimiter) waitBeforeSend(routeKey string, now time.Time) time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	t, ok := rl.resetAt[routeKey]
	if !ok {
		return 0
	}
	if d := t.Sub(now); d > 0 {
		return d
	}
	return 0
}

// onTooManyRequests is called after a 429. It computes how long this caller
// should wait (header-derived or default, plus jitter), records the route's
// reset time for other concurrent callers, and reports which header drove the
// decision (for logging). The returned wait already includes the jitter so the
// in-flight caller that sleeps it is de-synced from its peers (PERF-001).
func (rl *rateLimiter) onTooManyRequests(routeKey string, h http.Header, now time.Time) (wait time.Duration, source string) {
	base, source := rateLimitWaitFromHeaders(h, now, rl.defaultWait)

	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.jitterIx++
	wait = base + rl.jitterFor(rl.jitterIx)

	resetAt := now.Add(wait)
	// Keep the latest of any concurrently recorded value so two simultaneous
	// 429s never shorten an already-recorded window.
	if existing, ok := rl.resetAt[routeKey]; !ok || resetAt.After(existing) {
		rl.resetAt[routeKey] = resetAt
	}
	return wait, source
}

// jitterFor returns a deterministic offset in [0, rl.jitter) derived from a
// counter. No clock or RNG is used (both are unavailable here), yet consecutive
// callers get different offsets, which is what spreads a concurrent herd.
// Caller must hold rl.mu.
func (rl *rateLimiter) jitterFor(ix int) time.Duration {
	if rl.jitter <= 0 {
		return 0
	}
	const jitterStep = 250 * time.Millisecond
	return time.Duration(ix) * jitterStep % rl.jitter
}

// exceeded reports whether the safety ceiling has been reached. It is the only
// loop exit other than a non-429 response and context cancellation.
func (rl *rateLimiter) exceeded(attempts int, totalWait time.Duration) bool {
	return attempts >= rl.maxAttempts || totalWait >= rl.maxTotalWait
}

// rateLimitWaitFromHeaders computes the base wait (no jitter) from the 429
// response headers and reports the source. Priority: X-Rate-Limit-Reset
// (RFC 3339) -> Retry-After (defensive) -> the provided default.
func rateLimitWaitFromHeaders(h http.Header, now time.Time, def time.Duration) (time.Duration, string) {
	if d, ok := parseRateLimitReset(h, now); ok {
		return d, "x-rate-limit-reset"
	}
	if d, ok := parseRetryAfter(h, now); ok {
		return d, "retry-after"
	}
	return def, "default"
}

// parseRateLimitReset parses the X-Rate-Limit-Reset header. The Frontegg
// api-gateway sets it as an RFC 3339 date-time string (verified against
// rate-limit.service.ts). It is never a number, so no numeric parsing.
func parseRateLimitReset(h http.Header, now time.Time) (time.Duration, bool) {
	v := h.Get(rateLimitResetHeader)
	if v == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0, false
	}
	d := t.Sub(now)
	if d <= 0 {
		return 0, false
	}
	return d, true
}

// parseRetryAfter parses the standard Retry-After header (integer seconds or an
// HTTP-date). Defensive only: Frontegg does not send it today.
func parseRetryAfter(h http.Header, now time.Time) (time.Duration, bool) {
	v := h.Get(retryAfterHeader)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := t.Sub(now)
		if d <= 0 {
			return 0, false
		}
		return d, true
	}
	return 0, false
}

// waitContext waits for d, returning early with the context error if ctx is
// cancelled or its deadline passes first.
func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
