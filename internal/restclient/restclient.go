package restclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	token               string
	client              http.Client
	baseURL             string
	conflictRetryMethod string
	ignore404           bool
	environmentId       string
	applicationId       string
	rl                  *rateLimiter
}

func MakeRestClient(baseURL string, environmentId string, applicationId string) Client {
	return Client{
		client:        http.Client{},
		baseURL:       baseURL,
		environmentId: environmentId,
		applicationId: applicationId,
		rl:            newRateLimiter(),
	}
}

func (c *Client) Authenticate(token string) {
	c.token = token
}

func (c *Client) ConflictRetryMethod(method string) {
	c.conflictRetryMethod = method
}

func (c *Client) Ignore404() {
	c.ignore404 = true
}

func (c *Client) DeleteWithHeaders(ctx context.Context, url string, headers http.Header, out interface{}) error {
	return c.RequestWithHeaders(ctx, "DELETE", url, headers, nil, out)
}

func (c *Client) GetWithHeaders(ctx context.Context, url string, headers http.Header, out interface{}) error {
	return c.RequestWithHeaders(ctx, "GET", url, headers, nil, out)
}

func (c *Client) PatchWithHeaders(ctx context.Context, url string, headers http.Header, in interface{}, out interface{}) error {
	return c.RequestWithHeaders(ctx, "PATCH", url, headers, in, out)
}

func (c *Client) PostWithHeaders(ctx context.Context, url string, headers http.Header, in interface{}, out interface{}) error {
	return c.RequestWithHeaders(ctx, "POST", url, headers, in, out)
}

func (c *Client) PutWithHeaders(ctx context.Context, url string, headers http.Header, in interface{}, out interface{}) error {
	return c.RequestWithHeaders(ctx, "PUT", url, headers, in, out)
}

func (c *Client) Delete(ctx context.Context, url string, out interface{}) error {
	return c.RequestWithHeaders(ctx, "DELETE", url, nil, nil, out)
}

func (c *Client) Get(ctx context.Context, url string, out interface{}) error {
	return c.RequestWithHeaders(ctx, "GET", url, nil, nil, out)
}

func (c *Client) Patch(ctx context.Context, url string, in interface{}, out interface{}) error {
	return c.RequestWithHeaders(ctx, "PATCH", url, nil, in, out)
}

func (c *Client) Post(ctx context.Context, url string, in interface{}, out interface{}) error {
	return c.RequestWithHeaders(ctx, "POST", url, nil, in, out)
}

func (c *Client) Put(ctx context.Context, url string, in interface{}, out interface{}) error {
	return c.RequestWithHeaders(ctx, "PUT", url, nil, in, out)
}

// buildRequest constructs a fresh *http.Request for a single attempt. It is
// called once per attempt because http.Request.Body is consumed by client.Do
// and cannot be replayed.
func (c *Client) buildRequest(ctx context.Context, method string, url string, headers http.Header, body []byte) (*http.Request, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("restclient: failed to construct request: %w", err)
	}
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	if c.environmentId != "" {
		req.Header.Set("frontegg-environment-id", c.environmentId)
	}
	if c.applicationId != "" {
		req.Header.Set("frontegg-application-id", c.applicationId)
	}
	return req, nil
}

func (c *Client) RequestWithHeaders(ctx context.Context, method string, url string, headers http.Header, in interface{}, out interface{}) error {
	// Capture the single-use flags once. They must be reused on every retry
	// iteration below: the 429 retry is a loop (not recursion), so re-reading
	// the struct fields would see them already zeroed and drop 409/404 behavior.
	conflictRetryMethod := c.conflictRetryMethod
	c.conflictRetryMethod = ""
	ignore404 := c.ignore404
	c.ignore404 = false

	var body []byte
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("restclient: failed to serialize JSON request: %w", err)
		}
		body = b
	}

	routeKey := c.rl.routeKey(method, url)

	var (
		attempts  int
		totalWait time.Duration
	)
	for {
		// Pre-send wait: if this route is known to be rate-limited, wait until
		// its reset before sending. Re-check after each wait (TOCTOU) since
		// another goroutine may push the reset further out. These waits count
		// toward the same safety ceiling as retries, so a route whose reset is
		// continually pushed out by concurrent 429s cannot make a deadline-less
		// request sleep forever.
		for {
			wait := c.rl.waitBeforeSend(routeKey, time.Now())
			if wait <= 0 {
				break
			}
			totalWait += wait
			if c.rl.exceeded(attempts, totalWait) {
				return fmt.Errorf(
					"restclient: rate limited and gave up after %d attempts (%s total) waiting to send: %s %s%s",
					attempts, totalWait, method, c.baseURL, url,
				)
			}
			if err := waitContext(ctx, wait); err != nil {
				return err
			}
		}

		req, err := c.buildRequest(ctx, method, url, headers, body)
		if err != nil {
			return err
		}

		log.Printf("[TRACE] Sending request %+v", req)
		res, err := c.client.Do(req)
		if err != nil {
			return fmt.Errorf("restclient: failed sending request: %w", err)
		}
		resBody, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return fmt.Errorf("restclient: failed to read response: %w", err)
		}

		switch {
		case res.StatusCode == 404 && ignore404:
			return nil
		case res.StatusCode == 409 && conflictRetryMethod != "":
			// Preserve existing behavior exactly: recurse with the swapped method.
			return c.RequestWithHeaders(ctx, conflictRetryMethod, url, headers, in, out)
		case res.StatusCode == http.StatusTooManyRequests:
			wait, source := c.rl.onTooManyRequests(routeKey, res.Header, time.Now())

			attempts++
			totalWait += wait
			if c.rl.exceeded(attempts, totalWait) {
				return fmt.Errorf(
					"restclient: rate limited and gave up after %d attempts (%s total): %s %s: %s: %v: %s",
					attempts, totalWait, req.Method, req.URL, res.Status, res.Header, resBody,
				)
			}
			log.Printf(
				"[WARN] rate limited on %s; waiting %s (source: %s) before retry %d",
				routeKey, wait, source, attempts,
			)
			if err := waitContext(ctx, wait); err != nil {
				return err
			}
			continue
		case res.StatusCode < 200 || res.StatusCode >= 300:
			return fmt.Errorf(
				"restclient: request failed: %s %s: %s: %v: %s",
				req.Method, req.URL, res.Status, res.Header, resBody,
			)
		}

		log.Printf("[TRACE] Received response data %q", string(resBody))
		if out != nil {
			if err := json.Unmarshal(resBody, out); err != nil {
				return fmt.Errorf("restclient: failed to decode JSON response %#v: %w", string(resBody), err)
			}
		}
		return nil
	}
}
