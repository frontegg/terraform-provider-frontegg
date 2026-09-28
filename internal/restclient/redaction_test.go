package restclient

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthenticationResponseRedaction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"success", 200, `{"token":"sensitive-response"}`},
		{"malformed", 200, `{"token":"sensitive-response",`},
		{"invalid number", 200, `{"lifetime":1e1000,"token":"sensitive-response"}`},
		{"unauthorized", 401, `{"error":"sensitive-response"}`},
		{"server error", 500, `sensitive-response`},
		{"rate limited", 429, `sensitive-response`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logs)
			defer log.SetOutput(previous)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Set-Cookie", "sensitive-cookie")
				w.Header().Set("Frontegg-Trace-Id", "trace-id")
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			client := MakeRestClient(srv.URL, "", "")
			client.RedactResponses()
			client.rl.maxAttempts = 1
			client.rl.defaultWait = time.Millisecond
			var out struct {
				Token    string
				Lifetime float64
			}
			err := client.Post(context.Background(), "/auth/vendor", nil, &out)
			if tc.name == "success" {
				if err != nil || out.Token != "sensitive-response" {
					t.Fatalf("decode: %q, %v", out.Token, err)
				}
			} else if err == nil {
				t.Fatal("expected an error")
			}
			combined := logs.String() + fmt.Sprint(err)
			for _, secret := range []string{"sensitive-response", "sensitive-cookie", "1e1000"} {
				if strings.Contains(combined, secret) {
					t.Fatalf("secret %q leaked: %s", secret, combined)
				}
			}
			if tc.status >= 400 && !strings.Contains(fmt.Sprint(err), "trace-id") {
				t.Fatalf("error lost diagnostic trace ID: %v", err)
			}
		})
	}
}

func TestRequestLoggingPreservesScopeAndRedactsCredentials(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sensitive-bearer" || r.Header.Get("Cookie") != "sensitive-cookie" {
			t.Error("logging changed the request's credentials")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	client := MakeRestClient(srv.URL, "environment-id", "application-id")
	client.Authenticate("sensitive-bearer")
	headers := http.Header{}
	headers.Set("Cookie", "sensitive-cookie")
	headers.Set("Proxy-Authorization", "sensitive-proxy")
	headers.Set("frontegg-tenant-id", "tenant-id")
	if err := client.GetWithHeaders(context.Background(), "/resource", headers, nil); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"sensitive-bearer", "sensitive-cookie", "sensitive-proxy"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("credential leaked in request log: %s", logs.String())
		}
	}
	for _, scope := range []string{"environment-id", "application-id", "tenant-id"} {
		if !strings.Contains(logs.String(), scope) {
			t.Errorf("scope %q missing from request log: %s", scope, logs.String())
		}
	}
}
