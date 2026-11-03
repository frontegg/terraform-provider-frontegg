package provider

import (
	"net/http"
	"testing"
)

func TestPrehookWriteOnlySecretLifecycle(t *testing.T) {
	api := newFakeAPI(t, fakeCollection(fronteggPrehookPath, fronteggPrehookPath, "id")...)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address: "frontegg_prehook.test",
		secrets: map[string][]string{"secret": {"secret"}},
		config: func(arguments string) string {
			return `
resource "frontegg_prehook" "test" {
  enabled     = true
  name        = "lifecycle"
  description = "write-only lifecycle"
  type        = "API"
  url         = "https://example.com/prehook"
  events      = ["USER_INVITE"]
  fail_method = "OPEN"
` + arguments + `}
`
		},
	})
}

func TestEmailProviderWriteOnlySecretLifecycle(t *testing.T) {
	api := newFakeAPI(t,
		fakeAPIRoute{http.MethodPost, fronteggEmailPorivderPath, func(a *fakeAPI, _ string, body fakeDoc) (int, interface{}) {
			payload, _ := body["payload"].(map[string]interface{})
			a.put("mail", fakeDoc{"secret": payload["secret"], "createdAt": "2026-01-01T00:00:00Z"})
			return http.StatusOK, nil
		}},
		fakeAPIRoute{http.MethodGet, fronteggEmailPorivderPathV1, func(a *fakeAPI, _ string, _ fakeDoc) (int, interface{}) {
			return fakeGet(a, "mail", nil)
		}},
		fakeAPIRoute{http.MethodDelete, fronteggEmailPorivderPathV1, func(a *fakeAPI, _ string, _ fakeDoc) (int, interface{}) {
			return fakeDelete(a, "mail", nil)
		}},
	)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address: "frontegg_email_provider.test",
		secrets: map[string][]string{"secret": {"payload", "secret"}},
		config: func(arguments string) string {
			return `
resource "frontegg_email_provider" "test" {
  provider_name = "sendgrid"
` + arguments + `}
`
		},
	})
}
