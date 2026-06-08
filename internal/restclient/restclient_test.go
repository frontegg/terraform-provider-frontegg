package restclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient points a Client at a test server's URL and gives it an
// authenticated token so the auth header path is exercised.
func newTestClient(baseURL string) Client {
	c := MakeRestClient(baseURL, "", "")
	c.Authenticate("test-token")
	return c
}

func TestParseRateLimitReset(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		value   string
		wantOK  bool
		wantDur time.Duration
	}{
		{"rfc3339 future", now.Add(30 * time.Second).Format(time.RFC3339), true, 30 * time.Second},
		{"rfc3339 future with millis", now.Add(5 * time.Second).Format(time.RFC3339Nano), true, 5 * time.Second},
		{"rfc3339 past", now.Add(-10 * time.Second).Format(time.RFC3339), false, 0},
		{"empty", "", false, 0},
		{"malformed", "not-a-date", false, 0},
		{"numeric is rejected", "1750000000", false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.value != "" {
				h.Set(rateLimitResetHeader, tc.value)
			}
			d, ok := parseRateLimitReset(h, now)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && d != tc.wantDur {
				t.Fatalf("dur = %v, want %v", d, tc.wantDur)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		value   string
		wantOK  bool
		wantDur time.Duration
	}{
		{"integer seconds", "5", true, 5 * time.Second},
		{"zero seconds", "0", false, 0},
		{"negative seconds", "-3", false, 0},
		{"http-date future", now.Add(20 * time.Second).UTC().Format(http.TimeFormat), true, 20 * time.Second},
		{"http-date past", now.Add(-20 * time.Second).UTC().Format(http.TimeFormat), false, 0},
		{"empty", "", false, 0},
		{"garbage", "soon", false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.value != "" {
				h.Set(retryAfterHeader, tc.value)
			}
			d, ok := parseRetryAfter(h, now)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && d != tc.wantDur {
				t.Fatalf("dur = %v, want %v", d, tc.wantDur)
			}
		})
	}
}

func TestRateLimitWaitPriority(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	// Reset header wins over Retry-After.
	h := http.Header{}
	h.Set(rateLimitResetHeader, now.Add(30*time.Second).Format(time.RFC3339))
	h.Set(retryAfterHeader, "5")
	d, src := rateLimitWaitFromHeaders(h, now, defaultRateLimitWait)
	if src != "x-rate-limit-reset" || d != 30*time.Second {
		t.Fatalf("got (%v, %q), want (30s, x-rate-limit-reset)", d, src)
	}

	// No reset -> Retry-After.
	h = http.Header{}
	h.Set(retryAfterHeader, "5")
	d, src = rateLimitWaitFromHeaders(h, now, defaultRateLimitWait)
	if src != "retry-after" || d != 5*time.Second {
		t.Fatalf("got (%v, %q), want (5s, retry-after)", d, src)
	}

	// No usable header -> default.
	d, src = rateLimitWaitFromHeaders(http.Header{}, now, defaultRateLimitWait)
	if src != "default" || d != defaultRateLimitWait {
		t.Fatalf("got (%v, %q), want (%v, default)", d, src, defaultRateLimitWait)
	}
}

// TestRetryAfter429ResetHeader verifies a 429 with X-Rate-Limit-Reset is waited
// out and the request then succeeds.
func TestRetryAfter429ResetHeader(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			// 2s in the future; RFC3339 truncates sub-second, so the parsed
			// wait is ~1–2s — safely above the measurable floor below.
			w.Header().Set(rateLimitResetHeader, time.Now().Add(2*time.Second).UTC().Format(time.RFC3339))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	var out struct {
		OK bool `json:"ok"`
	}
	start := time.Now()
	if err := c.Get(context.Background(), "/thing", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Fatalf("response not decoded: %+v", out)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	// Must have actually waited for the reset before retrying (not retried
	// instantly). Lower bound is generous to tolerate RFC3339 truncation.
	if time.Since(start) < 500*time.Millisecond {
		t.Fatalf("expected to wait for reset, waited only %v", time.Since(start))
	}
}

// TestRetryAfter429RetryAfterHeader verifies the defensive Retry-After path.
func TestRetryAfter429RetryAfterHeader(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set(retryAfterHeader, "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.Get(context.Background(), "/thing", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

// TestRetry429PastResetUsesDefault verifies a 429 with no usable header falls
// back to the default wait. We override nothing; instead the server sends a 429
// with a past reset, then succeeds — the client must still recover. To keep the
// test fast we use a context deadline shorter than the default and assert the
// recovery happens only because the second attempt is allowed once the (past)
// reset yields a zero pre-wait. So here we send NO header and expect a default
// wait; we cap the test with a short-lived server response.
func TestRetry429StrippedHeadersUsesDefault(t *testing.T) {
	// Unit check: a 429 with no usable header must select the default source.
	d, src := rateLimitWaitFromHeaders(http.Header{}, time.Now(), defaultRateLimitWait)
	if src != "default" || d != defaultRateLimitWait {
		t.Fatalf("stripped-header 429 should use default, got (%v, %q)", d, src)
	}
}

// TestRetry429StrippedHeadersEndToEnd exercises the full retry path when the
// gateway strips the X-Rate-Limit-* headers (upstream-proxied 429). The default
// wait is shortened on the limiter so the test stays fast (TEST-002).
func TestRetry429StrippedHeadersEndToEnd(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			// No rate-limit headers at all.
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.rl.defaultWait = 100 * time.Millisecond
	c.rl.jitter = 0
	if err := c.Get(context.Background(), "/thing", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls (429 then 200), got %d", calls)
	}
}

// TestPostBodyReplayedOnRetry ensures the request body is present on attempt 2.
func TestPostBodyReplayedOnRetry(t *testing.T) {
	var calls int32
	var bodies []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set(rateLimitResetHeader, time.Now().Add(500*time.Millisecond).UTC().Format(time.RFC3339))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	in := map[string]string{"name": "widget"}
	if err := c.Post(context.Background(), "/things", in, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("expected 2 bodies, got %d: %v", len(bodies), bodies)
	}
	for i, b := range bodies {
		if !strings.Contains(b, `"name":"widget"`) {
			t.Fatalf("attempt %d body missing payload: %q", i+1, b)
		}
	}
}

// TestContextCancelDuringWait verifies a cancelled context aborts the wait
// promptly rather than blocking for the full reset duration.
func TestContextCancelDuringWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always 429 with a long reset so the client would otherwise wait.
		w.Header().Set(rateLimitResetHeader, time.Now().Add(1*time.Hour).UTC().Format(time.RFC3339))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := c.Get(ctx, "/thing", nil)
	if err == nil {
		t.Fatalf("expected context error, got nil")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("wait did not abort on context cancel; took %v", time.Since(start))
	}
}

// TestSafetyCeilingAttempts verifies the attempts ceiling fires and returns the
// "gave up after N attempts" error — using a background context (no deadline)
// so the ceiling, not the context, is what stops the loop (TEST-001).
func TestSafetyCeilingAttempts(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	// Tiny waits + a low attempts ceiling so the branch is reached quickly.
	c.rl.defaultWait = time.Millisecond
	c.rl.jitter = 0
	c.rl.maxAttempts = 3
	c.rl.maxTotalWait = time.Hour // ensure the attempts ceiling wins

	err := c.Get(context.Background(), "/thing", nil)
	if err == nil || !strings.Contains(err.Error(), "gave up after") {
		t.Fatalf("expected 'gave up after' ceiling error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected exactly maxAttempts (3) calls, got %d", got)
	}
}

// TestSafetyCeilingTotalWait verifies the total-wait ceiling fires independently
// of the attempts ceiling.
func TestSafetyCeilingTotalWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.rl.defaultWait = 20 * time.Millisecond
	c.rl.jitter = 0
	c.rl.maxAttempts = 1000                   // do not let attempts win
	c.rl.maxTotalWait = 60 * time.Millisecond // ~3 attempts worth

	err := c.Get(context.Background(), "/thing", nil)
	if err == nil || !strings.Contains(err.Error(), "gave up after") {
		t.Fatalf("expected 'gave up after' ceiling error, got %v", err)
	}
}

// TestPreSendWaitBoundedByCeiling verifies that the pre-send wait loop (entered
// when a route is already known to be rate-limited) is bounded by the safety
// ceiling, even with a deadline-less context and a reset that keeps being pushed
// into the future. Addresses Cursor Bugbot "Pre-send wait bypasses ceilings".
func TestPreSendWaitBoundedByCeiling(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.rl.defaultWait = 10 * time.Millisecond
	c.rl.jitter = 0
	c.rl.maxAttempts = 1000                   // attempts never increments here
	c.rl.maxTotalWait = 50 * time.Millisecond // pre-send waits must hit this

	// Seed the route as rate-limited far into the future so waitBeforeSend
	// always returns a positive wait and the pre-send loop never naturally exits.
	routeKey := c.rl.routeKey("GET", "/thing")
	c.rl.resetAt[routeKey] = time.Now().Add(time.Hour)

	// Background context (no deadline) — only the ceiling can stop this.
	err := c.Get(context.Background(), "/thing", nil)
	if err == nil || !strings.Contains(err.Error(), "gave up after") {
		t.Fatalf("expected pre-send ceiling error, got %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("request should never have been sent (stuck in pre-send wait), but server saw %d hits", got)
	}
}

// TestContextStopsPersistent429 verifies that, with no ceiling reached, a
// persistent 429 is still bounded by the request context.
func TestContextStopsPersistent429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(rateLimitResetHeader, time.Now().Add(50*time.Millisecond).UTC().Format(time.RFC3339))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := c.Get(ctx, "/thing", nil)
	if err == nil {
		t.Fatalf("expected an error from persistent 429, got nil")
	}
}

// Test409ConflictRetryPreserved verifies the existing 409 conflict-retry still
// re-issues with the swapped method.
func Test409ConflictRetryPreserved(t *testing.T) {
	var methods []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method)
		n := len(methods)
		mu.Unlock()
		if n == 1 {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.ConflictRetryMethod("PUT")
	if err := c.Post(context.Background(), "/things", map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(methods) != 2 || methods[0] != "POST" || methods[1] != "PUT" {
		t.Fatalf("expected [POST PUT], got %v", methods)
	}
}

// Test404IgnorePreserved verifies the existing 404-ignore still returns nil.
func Test404IgnorePreserved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.Ignore404()
	if err := c.Get(context.Background(), "/missing", nil); err != nil {
		t.Fatalf("expected nil for ignored 404, got %v", err)
	}
}

// Test429Then409 verifies the captured conflict-retry flag still fires when a
// 429 precedes a 409 (the flag must survive the 429 loop iterations).
func Test429Then409(t *testing.T) {
	var step int32
	var mu sync.Mutex
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method)
		mu.Unlock()
		switch atomic.AddInt32(&step, 1) {
		case 1:
			w.Header().Set(rateLimitResetHeader, time.Now().Add(300*time.Millisecond).UTC().Format(time.RFC3339))
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusConflict)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.ConflictRetryMethod("PUT")
	if err := c.Post(context.Background(), "/things", map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	// POST (429) -> POST (409) -> PUT (200)
	if len(methods) != 3 || methods[0] != "POST" || methods[1] != "POST" || methods[2] != "PUT" {
		t.Fatalf("expected [POST POST PUT], got %v", methods)
	}
}

// TestConcurrentSameRouteWaits exercises the per-route rate-limit map (the new
// shared state added by this story) under -race. Each goroutine uses its own
// Client value that shares one *rateLimitState, mirroring concurrent traffic
// hitting the rate-limit memory. (Per-Client flag fields like
// conflictRetryMethod are intentionally NOT shared here: that pre-existing
// unguarded race is documented as out-of-scope, and callers serialize it.)
func TestConcurrentSameRouteWaits(t *testing.T) {
	var limitedHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/limited" {
			// First hit 429s with a short reset; subsequent hits succeed.
			if atomic.AddInt32(&limitedHits, 1) == 1 {
				w.Header().Set(rateLimitResetHeader, time.Now().Add(300*time.Millisecond).UTC().Format(time.RFC3339))
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	shared := newRateLimiter()
	newClient := func() Client {
		c := newTestClient(srv.URL)
		c.rl = shared
		return c
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := newClient()
			_ = c.Get(context.Background(), "/limited", nil)
		}()
	}
	// Unrelated route must not be blocked by the limited route's reset.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := newClient()
			if err := c.Get(context.Background(), "/other", nil); err != nil {
				t.Errorf("unrelated route errored: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestRouteKeyStripsQuery(t *testing.T) {
	rl := newRateLimiter()
	got := rl.routeKey("GET", "/users?offset=10&limit=50")
	want := "GET /users"
	if got != want {
		t.Fatalf("route key = %q, want %q", got, want)
	}
}

// TestJitterAppliedToWait verifies the wait returned to the in-flight caller
// includes jitter (PERF-001), so concurrent callers do not all wake at the same
// instant. Consecutive onTooManyRequests calls with the same base must return
// different waits.
func TestJitterAppliedToWait(t *testing.T) {
	rl := newRateLimiter()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	h := http.Header{} // no headers -> base = defaultWait

	w1, _ := rl.onTooManyRequests("GET /a", h, now)
	w2, _ := rl.onTooManyRequests("GET /b", h, now)

	if w1 < rl.defaultWait || w2 < rl.defaultWait {
		t.Fatalf("waits should be >= base default; got %v, %v", w1, w2)
	}
	if w1 >= rl.defaultWait+rl.jitter || w2 >= rl.defaultWait+rl.jitter {
		t.Fatalf("waits should be < base+jitter; got %v, %v", w1, w2)
	}
	if w1 == w2 {
		t.Fatalf("consecutive waits should differ due to jitter; both %v", w1)
	}
}

// TestNoJitterWhenDisabled verifies jitter can be turned off (used by other
// tests to get deterministic waits).
func TestNoJitterWhenDisabled(t *testing.T) {
	rl := newRateLimiter()
	rl.jitter = 0
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	w, _ := rl.onTooManyRequests("GET /a", http.Header{}, now)
	if w != rl.defaultWait {
		t.Fatalf("with jitter disabled, wait should equal default %v, got %v", rl.defaultWait, w)
	}
}

// TestNon429ErrorUnchanged verifies a generic non-2xx still returns the existing
// formatted error and does not get swallowed by the 429 path.
func TestNon429ErrorUnchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.Get(context.Background(), "/thing", nil)
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("expected formatted request-failed error, got %v", err)
	}
}

// sanity: ensure the example reset value format in the story parses.
func TestStoryExampleResetParses(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	h := http.Header{}
	h.Set(rateLimitResetHeader, "2026-06-07T12:34:56.789Z")
	d, ok := parseRateLimitReset(h, now)
	if !ok {
		t.Fatalf("story example reset should parse")
	}
	if d <= 0 {
		t.Fatalf("expected positive duration, got %v", d)
	}
	_ = fmt.Sprintf("%v", d)
}
