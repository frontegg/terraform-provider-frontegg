package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
)

type capturedMFARequest struct {
	method string
	path   string
	header http.Header
}

func exerciseMFARequests(t *testing.T, applicationID string) []capturedMFARequest {
	t.Helper()

	var (
		mu       sync.Mutex
		captured []capturedMFARequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		captured = append(captured, capturedMFARequest{
			method: r.Method,
			path:   r.URL.Path,
			header: r.Header.Clone(),
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{}`)); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer srv.Close()

	client := restclient.MakeRestClient(srv.URL, "", applicationID)
	client.Authenticate("test-token")

	ctx := context.Background()
	tenantHeaders := http.Header{}
	tenantHeaders.Add("frontegg-tenant-id", "tenant-1")

	if _, err := getMFAPolicy(ctx, &client, nil); err != nil {
		t.Fatalf("getMFAPolicy (workspace): %v", err)
	}
	if _, err := getMFAPolicy(ctx, &client, tenantHeaders); err != nil {
		t.Fatalf("getMFAPolicy (tenant): %v", err)
	}
	if err := writeMFAPolicy(ctx, &client, nil, fronteggMFAPolicy{}); err != nil {
		t.Fatalf("writeMFAPolicy (workspace): %v", err)
	}
	if err := writeMFAPolicy(ctx, &client, tenantHeaders, fronteggMFAPolicy{}); err != nil {
		t.Fatalf("writeMFAPolicy (tenant): %v", err)
	}

	var out fronteggMFA
	if err := client.Get(ctx, fronteggMFAURL, &out); err != nil {
		t.Fatalf("get authentication app config: %v", err)
	}
	if err := client.Post(ctx, fronteggMFAURL, fronteggMFA{}, nil); err != nil {
		t.Fatalf("post authentication app config: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	return append([]capturedMFARequest(nil), captured...)
}

func TestMFARequestsSendApplicationIDHeader(t *testing.T) {
	captured := exerciseMFARequests(t, "app-123")

	if len(captured) != 6 {
		t.Fatalf("captured %d requests, want 6", len(captured))
	}
	for _, req := range captured {
		if req.path != fronteggMFAPolicyURL && req.path != fronteggMFAURL {
			t.Errorf("unexpected path %q", req.path)
		}
		if got := req.header.Get("frontegg-application-id"); got != "app-123" {
			t.Errorf("%s %s: frontegg-application-id = %q, want %q", req.method, req.path, got, "app-123")
		}
	}
}

func TestMFATenantScopedRequestsKeepBothHeaders(t *testing.T) {
	captured := exerciseMFARequests(t, "app-123")

	tenantScoped := 0
	for _, req := range captured {
		if req.header.Get("frontegg-tenant-id") == "" {
			continue
		}
		tenantScoped++
		if got := req.header.Get("frontegg-application-id"); got != "app-123" {
			t.Errorf("%s %s: frontegg-application-id = %q, want %q", req.method, req.path, got, "app-123")
		}
	}
	if tenantScoped != 2 {
		t.Fatalf("%d tenant-scoped requests, want 2", tenantScoped)
	}
}

func TestMFARequestsOmitApplicationIDHeaderWhenUnset(t *testing.T) {
	captured := exerciseMFARequests(t, "")

	for _, req := range captured {
		if got := req.header.Get("frontegg-application-id"); got != "" {
			t.Errorf("%s %s: frontegg-application-id = %q, want it unset", req.method, req.path, got)
		}
	}
}
