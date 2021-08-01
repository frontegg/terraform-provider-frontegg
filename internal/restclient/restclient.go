package restclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Client struct {
	token               string
	client              http.Client
	conflictRetryMethod string
}

func New(baseURL string) Client {
	return Client{
		client: http.Client{},
	}
}

func (c *Client) Authenticate(token string) {
	c.token = token
}

func (c *Client) ConflictRetryMethod(method string) {
	c.conflictRetryMethod = method
}

func (c *Client) Delete(ctx context.Context, url string, out interface{}) error {
	return c.Request(ctx, "DELETE", url, nil, out)
}

func (c *Client) Get(ctx context.Context, url string, out interface{}) error {
	return c.Request(ctx, "GET", url, nil, out)
}

func (c *Client) Patch(ctx context.Context, url string, in interface{}, out interface{}) error {
	return c.Request(ctx, "PATCH", url, in, out)
}

func (c *Client) Post(ctx context.Context, url string, in interface{}, out interface{}) error {
	return c.Request(ctx, "POST", url, in, out)
}

func (c *Client) Put(ctx context.Context, url string, in interface{}, out interface{}) error {
	return c.Request(ctx, "PUT", url, in, out)
}

func (c *Client) Request(ctx context.Context, method string, url string, in interface{}, out interface{}) error {
	var reqBody io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("restclient: failed to serialize JSON request: %w", err)
		}
		reqBody = bytes.NewBuffer(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("restclient: failed to construct request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	log.Printf("[TRACE] Sending request %+v", req)
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("restclient: failed sending request: %w", err)
	}
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("restclient: failed to read response: %w", err)
	}
	if res.StatusCode == 409 && c.conflictRetryMethod != "" {
		method := c.conflictRetryMethod
		c.conflictRetryMethod = ""
		return c.Request(ctx, method, url, in, out)
	} else if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf(
			"restclient: request failed: %s %s: %s: %s",
			req.Method, req.URL, res.Status, resBody,
		)
	}
	log.Printf("[TRACE] Received response data %q", string(resBody))
	if out != nil {
		if err := json.Unmarshal(resBody, out); err != nil {
			return fmt.Errorf("restclient: failed to decode JSON response: %w", err)
		}
	}
	return nil
}
