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
	baseURL             string
	conflictRetryMethod string
	ignore404           bool
	vendorId            string
}

func MakeRestClient(baseURL string) Client {
	return Client{
		client:  http.Client{},
		baseURL: baseURL,
	}
}

func (c *Client) SpecifyVendor(vendorId string) {
	c.vendorId = vendorId
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
	return c.RequestWithHeaders(ctx, "DELETE", url, nil, headers, out)
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

func (c *Client) RequestWithHeaders(ctx context.Context, method string, url string, headers http.Header, in interface{}, out interface{}) error {
	conflictRetryMethod := c.conflictRetryMethod
	c.conflictRetryMethod = ""
	ignore404 := c.ignore404
	c.ignore404 = false
	var reqBody io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("restclient: failed to serialize JSON request: %w", err)
		}
		reqBody = bytes.NewBuffer(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+url, reqBody)
	if err != nil {
		return fmt.Errorf("restclient: failed to construct request: %w", err)
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
	log.Printf("[TRACE] Sending request %+v", req)
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("restclient: failed sending request: %w", err)
	}
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("restclient: failed to read response: %w", err)
	}
	if res.StatusCode == 404 && ignore404 {
		return nil
	} else if res.StatusCode == 409 && conflictRetryMethod != "" {
		return c.RequestWithHeaders(ctx, conflictRetryMethod, url, headers, in, out)
	} else if res.StatusCode < 200 || res.StatusCode >= 300 {
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
