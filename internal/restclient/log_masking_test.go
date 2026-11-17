package restclient

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
)

func TestFailedRequestLogsMaskBearerTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Echoed-Authorization", "Bearer leaked-response-token")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	var logs bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &logs)
	client := MakeRestClient(srv.URL, "", "")
	client.Authenticate("request-token")

	if err := client.Get(ctx, "/resource", nil); err == nil {
		t.Fatal("expected the 400 response to fail the request")
	}

	output := logs.String()
	if !strings.Contains(output, "response headers for failed request") {
		t.Fatalf("expected the failed response headers to be logged, got:\n%s", output)
	}
	for _, secret := range []string{"leaked-response-token", "request-token"} {
		if strings.Contains(output, secret) {
			t.Errorf("log output contains %q:\n%s", secret, output)
		}
	}
}

func TestSensitiveLogFieldsAreMasked(t *testing.T) {
	var logs bytes.Buffer
	ctx := withSecretMasking(tflogtest.RootLogger(context.Background(), &logs))

	tflog.Trace(ctx, "credentials in fields", map[string]interface{}{
		"client_secret": "field-client-secret",
		"token":         "field-token",
		"http_url":      "https://api.frontegg.com/resource",
	})

	output := logs.String()
	for _, secret := range []string{"field-client-secret", "field-token"} {
		if strings.Contains(output, secret) {
			t.Errorf("log output contains %q:\n%s", secret, output)
		}
	}
	if !strings.Contains(output, "https://api.frontegg.com/resource") {
		t.Errorf("expected non-sensitive fields to stay readable, got:\n%s", output)
	}
}
